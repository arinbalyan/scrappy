//! Shared emulation profile headers and feature-gated HTTP fetcher.
//!
//! See documentation/006-architecture.md.

// Rust guideline compliant 2026-10-09

use std::collections::BTreeMap;

#[cfg(feature = "emulation")]
use super::{Fetcher, MAX_BODY_BYTES, Request, Response};
#[cfg(feature = "emulation")]
use crate::security::{resolve_public_host, validate_target};

/// One pinned browser-like request profile.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct EmulationProfile {
    /// Stable profile name declared in a source manifest.
    pub name: String,
    /// User agent sent with every request in the profile.
    pub user_agent: String,
}

/// Look up a pinned profile by name.
#[must_use]
pub fn profile(name: &str) -> Option<EmulationProfile> {
    match name {
        "chrome-131" => Some(EmulationProfile {
            name: name.to_owned(),
            user_agent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36".to_owned(),
        }),
        _ => None,
    }
}

/// Return headers that agree with the named profile.
#[must_use]
pub fn headers(name: &str) -> BTreeMap<String, String> {
    let Some(profile) = profile(name) else {
        return BTreeMap::new();
    };
    let mut headers = BTreeMap::new();
    headers.insert("user-agent".to_owned(), profile.user_agent);
    headers.insert(
        "sec-ch-ua".to_owned(),
        "\"Chromium\";v=\"131\", \"Google Chrome\";v=\"131\"".to_owned(),
    );
    headers.insert("sec-ch-ua-platform".to_owned(), "\"Windows\"".to_owned());
    headers
}

/// Fetcher using a pinned wreq emulation profile.
#[cfg(feature = "emulation")]
#[derive(Debug, Clone, Default)]
pub struct EmulationFetcher {
    profile: Option<String>,
}

#[cfg(feature = "emulation")]
impl EmulationFetcher {
    /// Build a fetcher for one pinned profile.
    #[must_use]
    pub fn new(profile: impl Into<String>) -> Self {
        Self {
            profile: Some(profile.into()),
        }
    }
}

#[cfg(feature = "emulation")]
impl Fetcher for EmulationFetcher {
    async fn fetch(&self, request: &Request) -> Result<Response, String> {
        let profile = self
            .profile
            .as_deref()
            .ok_or("no emulation profile selected")?;
        let client = wreq::Client::builder()
            .emulation(profile_name(profile)?)
            .dns_resolver(SafeResolver)
            .redirect(wreq::redirect::Policy::none())
            .build()
            .map_err(|error| format!("cannot build emulation client: {error}"))?;
        let method = wreq::Method::from_bytes(request.method.as_bytes())
            .map_err(|error| format!("invalid HTTP method: {error}"))?;
        let mut current = request.url.clone();
        let mut response = None;
        for _ in 0..=MAX_REDIRECTS {
            validate_target(&current)?;
            let mut builder = client.request(method.clone(), &current);
            for (name, value) in &request.headers {
                builder = builder.header(name, value);
            }
            if !request.body.is_empty() {
                builder = builder.body(request.body.clone());
            }
            let sent = builder
                .send()
                .await
                .map_err(|error| format!("emulated request failed: {error}"))?;
            if !sent.status().is_redirection() {
                response = Some(sent);
                break;
            }
            let location = sent
                .headers()
                .get(wreq::header::LOCATION)
                .and_then(|value| value.to_str().ok())
                .ok_or("redirect has no usable location")?;
            current = resolve_location(&current, location)?;
        }
        let response = response.ok_or("too many redirects")?;
        let status = response.status().as_u16();
        let url = current;
        let headers = response
            .headers()
            .iter()
            .filter_map(|(name, value)| {
                value
                    .to_str()
                    .ok()
                    .map(|value| (name.as_str().to_owned(), value.to_owned()))
            })
            .collect();
        let body = bounded_body(response).await?;
        Response::new(url, status, headers, body)
    }
}

#[cfg(feature = "emulation")]
const MAX_REDIRECTS: usize = 10;

/// Read a bounded body so oversized responses are rejected before they are kept.
///
/// # Errors
/// Returns a diagnostic when the body cannot be read.
#[cfg(feature = "emulation")]
async fn bounded_body(response: wreq::Response) -> Result<Vec<u8>, String> {
    use futures::StreamExt;

    let mut stream = response.bytes_stream();
    let mut body = Vec::new();
    while let Some(item) = stream.next().await {
        let chunk = item.map_err(|error| format!("cannot read emulated body: {error}"))?;
        if body.len() + chunk.len() > MAX_BODY_BYTES {
            return Err(format!("response body exceeds {MAX_BODY_BYTES} bytes"));
        }
        body.extend_from_slice(&chunk);
    }
    Ok(body)
}

#[cfg(feature = "emulation")]
#[derive(Debug)]
struct SafeResolver;

#[cfg(feature = "emulation")]
impl wreq::dns::Resolve for SafeResolver {
    fn resolve(&self, name: wreq::dns::Name) -> wreq::dns::Resolving {
        let host = name.as_str().to_owned();
        Box::pin(async move {
            let addresses = resolve_public_host(&host)?;
            let addrs: Vec<_> = addresses
                .into_iter()
                .map(|address| std::net::SocketAddr::new(address, 0))
                .collect();
            Ok(Box::new(addrs.into_iter()) as wreq::dns::Addrs)
        })
    }
}

#[cfg(feature = "emulation")]
fn join_relative(directory: &str, location: &str) -> String {
    if directory.ends_with('/') {
        format!("{directory}{location}")
    } else {
        format!("{directory}/{location}")
    }
}

#[cfg(feature = "emulation")]
fn resolve_location(current: &str, location: &str) -> Result<String, String> {
    if location.starts_with("http://") || location.starts_with("https://") {
        validate_target(location)?;
        return Ok(location.to_owned());
    }
    let (scheme, remainder) = current.split_once("://").ok_or("invalid current URL")?;
    let authority_end = remainder.find(['/', '?', '#']).unwrap_or(remainder.len());
    let authority = &remainder[..authority_end];
    let path =
        if location.starts_with('/') {
            location.to_owned()
        } else {
            let base = &remainder[authority_end..];
            let base_path = base.split('?').next().unwrap_or(base);
            let directory = base_path.rsplit_once('/').map_or("/", |(prefix, _)| {
                if prefix.is_empty() { "/" } else { prefix }
            });
            join_relative(directory, location)
        };
    let target = format!("{scheme}://{authority}{path}");
    validate_target(&target)?;
    Ok(target)
}

#[cfg(feature = "emulation")]
fn profile_name(name: &str) -> Result<wreq_util::Profile, String> {
    match name {
        "chrome-131" => Ok(wreq_util::Profile::Chrome131),
        other => Err(format!("unknown emulation profile: {other}")),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[cfg(feature = "emulation")]
    #[test]
    fn join_relative_avoids_double_slash() {
        assert_eq!(join_relative("/", "next"), "/next");
        assert_eq!(join_relative("/jobs", "next"), "/jobs/next");
        assert_eq!(join_relative("/jobs/", "next"), "/jobs/next");
    }

    #[test]
    fn profile_headers_match_user_agent() {
        let values = headers("chrome-131");
        assert!(values["user-agent"].contains("Chrome/131"));
        assert!(values["sec-ch-ua"].contains("131"));
    }

    #[test]
    fn unknown_profile_has_no_headers() {
        assert!(headers("unknown").is_empty());
    }

    #[cfg(feature = "emulation")]
    #[test]
    fn pinned_profile_maps_to_wreq() {
        profile_name("chrome-131").unwrap();
        assert!(profile_name("unknown").unwrap_err().contains("unknown"));
    }
}
