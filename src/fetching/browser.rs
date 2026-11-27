// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Arin Balyan
// Last modified: 2026-11-27

//! Optional browser rendering fetcher and its dependency resolver.
//!
//! See documentation/012-browser-automation.md.

// Rust guideline compliant 2026-10-09

use std::net::ToSocketAddrs;
use std::path::{Path, PathBuf};

#[cfg(feature = "browser")]
use std::collections::BTreeMap;

#[cfg(feature = "browser")]
use crate::fetching::proxy::ProxySelector;
#[cfg(feature = "browser")]
use crate::fetching::{Fetcher, Request, Response};

/// Pinned Chromium revision used by the optional browser build.
pub const PINNED_REVISION: u32 = 1_585_606;

/// Environment variable for an explicit browser executable path.
pub const EXECUTABLE_ENV: &str = "SCRAPPY_BROWSER_EXECUTABLE";

/// Highest accepted rendered body size.
pub const MAX_RENDERED_BYTES: usize = 8 * 1024 * 1024;

/// Highest accepted cookie size in bytes.
pub const MAX_COOKIE_BYTES: usize = 4096;

/// One reason a browser backend may be unavailable.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Unavailable {
    /// The binary was built without the browser feature.
    NotCompiled,
    /// No browser executable is configured or installed.
    NoExecutable,
    /// The configured executable path does not exist.
    MissingExecutable,
}

impl Unavailable {
    /// Return a short diagnostic without exposing environment values.
    #[must_use]
    pub const fn reason(&self) -> &'static str {
        match self {
            Self::NotCompiled => "browser feature is disabled in this build",
            Self::NoExecutable => "no browser executable is configured or installed",
            Self::MissingExecutable => "configured browser executable does not exist",
        }
    }
}

/// Resolve the browser executable used by doctor and the engine.
///
/// # Errors
/// Returns the reason no usable browser executable was found.
pub fn resolve_executable() -> Result<PathBuf, Unavailable> {
    if !cfg!(feature = "browser") {
        return Err(Unavailable::NotCompiled);
    }
    if let Some(value) = std::env::var_os(EXECUTABLE_ENV) {
        let path = PathBuf::from(value);
        if !path.is_file() {
            return Err(Unavailable::MissingExecutable);
        }
        return Ok(path);
    }
    let cache = browser_cache();
    if let Some(path) = cached_executable(&cache) {
        return Ok(path);
    }
    if let Some(path) = installed_executable() {
        return Ok(path);
    }
    Err(Unavailable::NoExecutable)
}

/// Report whether this build can use browser automation.
#[must_use]
pub fn available() -> bool {
    resolve_executable().is_ok()
}

/// Return the pinned browser cache directory.
#[must_use]
pub fn browser_cache() -> PathBuf {
    let base = std::env::var_os("XDG_CACHE_HOME").map_or_else(
        || PathBuf::from(std::env::var_os("HOME").unwrap_or_default()).join(".cache"),
        PathBuf::from,
    );
    base.join("scrappy").join("browser")
}

#[cfg(feature = "browser")]
fn installed_executable() -> Option<PathBuf> {
    use chromiumoxide::detection::DetectionOptions;
    chromiumoxide::detection::default_executable(DetectionOptions::default()).ok()
}

#[cfg(not(feature = "browser"))]
fn installed_executable() -> Option<PathBuf> {
    None
}

/// Find a previously fetched browser in the pinned cache directory.
#[must_use]
pub fn cached_executable(root: &Path) -> Option<PathBuf> {
    let mut candidates = Vec::new();
    collect_executables(root, &mut candidates, 0);
    candidates.sort();
    candidates.into_iter().next()
}

fn collect_executables(root: &Path, found: &mut Vec<PathBuf>, depth: u32) {
    if depth > 6 {
        return;
    }
    let Ok(entries) = std::fs::read_dir(root) else {
        return;
    };
    for entry in entries.flatten() {
        let path = entry.path();
        if path.is_dir() {
            collect_executables(&path, found, depth + 1);
        } else if is_browser_filename(&path) {
            found.push(path);
        }
    }
}

fn is_browser_filename(path: &Path) -> bool {
    path.file_name()
        .and_then(|name| name.to_str())
        .is_some_and(|name| matches!(name, "chrome" | "chromium" | "headless_shell"))
}

/// Reject a URL that resolves to a private or reserved address.
///
/// # Errors
/// Returns a diagnostic when the host is missing, cannot resolve, or resolves
/// to a disallowed address.
pub fn validate_public_target(url: &str) -> Result<(), String> {
    let rest = url.split_once("://").map_or(url, |(_, rest)| rest);
    let host = rest.split(['/', '?', '#', '@']).next().unwrap_or_default();
    let host = host.rsplit(':').next().unwrap_or_default();
    if host.is_empty() {
        return Err("browser target has no host".to_owned());
    }
    if is_disallowed_host(host) {
        return Err(format!("browser target host is not public: {host}"));
    }
    let mut attempts = (host, 0)
        .to_socket_addrs()
        .map_err(|error| format!("cannot resolve browser target host {host}: {error}"))?;
    let mut resolved = false;
    for address in attempts.by_ref() {
        resolved = true;
        if is_disallowed_address(&address.ip()) {
            return Err(format!("browser target IP is not public: {}", address.ip()));
        }
    }
    if !resolved {
        return Err(format!("browser target host has no address: {host}"));
    }
    Ok(())
}

fn is_disallowed_host(host: &str) -> bool {
    let host = host.trim_matches(['[', ']']).to_ascii_lowercase();
    host == "localhost" || host.ends_with(".localhost") || host.ends_with(".internal")
}

fn is_disallowed_address(address: &std::net::IpAddr) -> bool {
    use std::net::IpAddr::{V4, V6};
    match address {
        V4(ip) => {
            ip.is_private()
                || ip.is_loopback()
                || ip.is_link_local()
                || ip.is_unspecified()
                || ip.is_broadcast()
                || ip.is_documentation()
                || ip.is_multicast()
        }
        V6(ip) => {
            ip.is_loopback()
                || ip.is_unspecified()
                || ip.is_multicast()
                || ip.is_unique_local()
                || ip.is_unicast_link_local()
        }
    }
}

/// Detect a challenge without retaining cookies, tokens, or query strings.
#[must_use]
pub fn challenge_detected(text: &str) -> bool {
    crate::engine::challenge::detect(200, &std::collections::BTreeMap::new(), text).is_some()
}

/// Download the pinned Chromium build into the scrappy cache.
///
/// # Errors
/// Returns a diagnostic when the pinned build cannot be fetched or installed.
#[cfg(feature = "browser")]
pub async fn fetch_pinned_executable() -> Result<PathBuf, String> {
    use chromiumoxide::fetcher::{BrowserFetcher, BrowserFetcherOptions, BrowserKind, Revision};

    let options = BrowserFetcherOptions::builder()
        .with_path(browser_cache())
        .with_kind(BrowserKind::Chromium)
        .with_version(Revision::new(PINNED_REVISION))
        .build()
        .map_err(|error| format!("cannot configure browser download: {error}"))?;
    let installation = BrowserFetcher::new(options)
        .fetch()
        .await
        .map_err(|error| format!("cannot download pinned browser: {error}"))?;
    Ok(installation.executable_path)
}

/// Fetcher that renders a page with the pinned Chromium build over CDP.
#[cfg(feature = "browser")]
#[derive(Debug)]
pub struct BrowserFetcher {
    executable: Option<PathBuf>,
    proxies: Option<ProxySelector>,
}

#[cfg(feature = "browser")]
impl BrowserFetcher {
    /// Build a fetcher with an optional explicit browser executable.
    #[must_use]
    pub fn new(executable: Option<PathBuf>) -> Self {
        Self {
            executable,
            proxies: None,
        }
    }

    /// Attach a proxy selector used for every browser request.
    #[must_use]
    pub fn with_proxies(mut self, proxies: ProxySelector) -> Self {
        self.proxies = Some(proxies);
        self
    }
}

#[cfg(feature = "browser")]
impl Fetcher for BrowserFetcher {
    async fn fetch(&self, request: &Request) -> Result<Response, String> {
        validate_public_target(&request.url)?;
        let executable = self
            .executable
            .clone()
            .or_else(|| resolve_executable().ok())
            .ok_or(Unavailable::NoExecutable.reason())?;
        let proxy = self.proxies.as_ref().and_then(ProxySelector::next);
        let config = browser_config(&executable, proxy.as_ref().map(|item| item.url.as_str()))?;
        let (mut browser, mut handler) = chromiumoxide::Browser::launch(config)
            .await
            .map_err(|error| format!("cannot launch browser: {error}"))?;
        let driver =
            tokio::spawn(
                async move { while futures::StreamExt::next(&mut handler).await.is_some() {} },
            );
        let page_result = open_page(&browser, request).await;
        if let Ok(page) = &page_result
            && let Ok(Some(final_url)) = page.url().await
        {
            validate_public_target(&final_url)?;
        }
        let result = render(&page_result, request).await;
        let _ = browser.close().await;
        driver.abort();
        match result {
            Ok(response) => response,
            Err(error) => {
                if let (Some(selector), Some(proxy)) = (&self.proxies, proxy) {
                    selector.reject(&proxy.url);
                }
                Err(error)
            }
        }
    }
}

#[cfg(feature = "browser")]
fn browser_config(
    executable: &Path,
    proxy: Option<&str>,
) -> Result<chromiumoxide::BrowserConfig, String> {
    use chromiumoxide::browser::BrowserConfig;

    let mut builder = BrowserConfig::builder()
        .chrome_executable(executable)
        .incognito()
        .respect_https_errors()
        .disable_cache()
        .request_timeout(crate::fetching::DEFAULT_TIMEOUT);
    if let Some(proxy) = proxy {
        builder = builder.arg(format!("--proxy-server={proxy}"));
    }
    builder
        .build()
        .map_err(|error| format!("cannot configure browser: {error}"))
}

#[cfg(feature = "browser")]
async fn open_page(
    browser: &chromiumoxide::Browser,
    request: &Request,
) -> Result<chromiumoxide::Page, String> {
    use chromiumoxide::cdp::browser_protocol::network::{Headers, SetExtraHttpHeadersParams};
    use chromiumoxide::cdp::browser_protocol::page::NavigateParams;

    let page = browser
        .new_page("about:blank")
        .await
        .map_err(|error| format!("cannot open browser page: {error}"))?;
    let headers: serde_json::Map<String, serde_json::Value> = request
        .headers
        .iter()
        .map(|(name, value)| (name.clone(), serde_json::Value::String(value.clone())))
        .collect();
    if !headers.is_empty() {
        page.execute(SetExtraHttpHeadersParams::new(Headers::new(
            serde_json::Value::Object(headers),
        )))
        .await
        .map_err(|error| format!("cannot set browser headers: {error}"))?;
    }
    page.goto(NavigateParams::new(&request.url))
        .await
        .map_err(|error| format!("browser navigation failed: {error}"))?;
    page.wait_for_navigation()
        .await
        .map_err(|error| format!("browser navigation did not complete: {error}"))?;
    Ok(page)
}

#[cfg(feature = "browser")]
async fn render(
    page_result: &Result<chromiumoxide::Page, String>,
    request: &Request,
) -> Result<Result<Response, String>, String> {
    let page = match page_result {
        Ok(page) => page,
        Err(error) => return Ok(Err(error.clone())),
    };
    let html = page
        .content()
        .await
        .map_err(|error| format!("cannot read rendered content: {error}"))?;
    if html.len() > MAX_RENDERED_BYTES {
        return Ok(Err(format!(
            "rendered body exceeds {MAX_RENDERED_BYTES} bytes"
        )));
    }
    if challenge_detected(&html) {
        return Ok(Err("browser challenge detected".to_owned()));
    }
    let cookies = page
        .get_cookies()
        .await
        .map_err(|error| format!("cannot read browser cookies: {error}"))?;
    let cookie_size: usize = cookies
        .iter()
        .map(|cookie| cookie.name.len() + cookie.value.len())
        .sum();
    if cookie_size > MAX_COOKIE_BYTES {
        return Ok(Err(format!("cookies exceed {MAX_COOKIE_BYTES} bytes")));
    }
    let final_url = page
        .url()
        .await
        .ok()
        .flatten()
        .unwrap_or_else(|| request.url.clone());
    validate_public_target(&final_url)?;
    let body = html.into_bytes();
    if body.len() > MAX_RENDERED_BYTES {
        return Ok(Err(format!(
            "rendered body exceeds {MAX_RENDERED_BYTES} bytes"
        )));
    }
    Ok(Response::new(final_url, 200, BTreeMap::new(), body))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn reports_build_feature() {
        assert_eq!(available(), resolve_executable().is_ok());
    }

    #[test]
    fn rejects_localhost_target() {
        assert!(validate_public_target("http://localhost/jobs").is_err());
    }

    #[test]
    fn rejects_private_literal() {
        assert!(validate_public_target("http://127.0.0.1/jobs").is_err());
    }

    #[test]
    fn detects_challenge_marker() {
        assert!(challenge_detected("<title>Verify You Are Human</title>"));
        assert!(!challenge_detected("<main>Jobs</main>"));
    }

    #[test]
    fn finds_cached_executable() {
        let root =
            std::env::temp_dir().join(format!("scrappy-browser-test-{}", std::process::id()));
        let nested = root.join("chrome-linux64");
        std::fs::create_dir_all(&nested).expect("test directory");
        let executable = nested.join("chrome");
        std::fs::write(&executable, b"test").expect("test executable");
        assert_eq!(cached_executable(&root), Some(executable));
        std::fs::remove_dir_all(root).expect("test cleanup");
    }
}
