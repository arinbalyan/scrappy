//! Fetcher contracts and implementations.
//!
//! Parsers receive `Response` values and never call the network. See
//! documentation/006-architecture.md.

// Rust guideline compliant 2026-10-09

pub mod browser;
pub mod emulation;
pub mod plain;
pub mod proxy;

use std::collections::BTreeMap;
use std::time::Duration;

/// Maximum accepted response body size.
pub const MAX_BODY_BYTES: usize = 8 * 1024 * 1024;

/// Default request timeout.
pub const DEFAULT_TIMEOUT: Duration = Duration::from_secs(30);

/// Validated HTTP request used by every fetcher.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Request {
    /// Absolute HTTP or HTTPS URL.
    pub url: String,
    /// HTTP method name.
    pub method: String,
    /// Request headers, lower-case names only.
    pub headers: BTreeMap<String, String>,
    /// Request timeout.
    pub timeout: Duration,
}

impl Request {
    /// Build and validate a GET request.
    ///
    /// # Errors
    /// Returns a diagnostic for an empty method, an unsupported scheme, or an
    /// empty host.
    pub fn get(url: impl Into<String>) -> Result<Self, String> {
        let url = url.into();
        let (scheme, host) = split_url(&url)?;
        if scheme != "http" && scheme != "https" {
            return Err(format!("unsupported URL scheme: {scheme}"));
        }
        if host.is_empty() {
            return Err("request URL has no host".to_owned());
        }
        Ok(Self {
            url,
            method: "GET".to_owned(),
            headers: BTreeMap::new(),
            timeout: DEFAULT_TIMEOUT,
        })
    }

    /// Add a header after validating its name and value.
    ///
    /// # Errors
    /// Returns a diagnostic when a name or value contains invalid bytes.
    pub fn with_header(mut self, name: &str, value: &str) -> Result<Self, String> {
        if name.is_empty() || name != name.to_ascii_lowercase() {
            return Err("header name must be lower-case and non-empty".to_owned());
        }
        if name
            .bytes()
            .any(|byte| matches!(byte, b'\r' | b'\n' | b':'))
            || value.bytes().any(|byte| matches!(byte, b'\r' | b'\n'))
        {
            return Err("header contains invalid bytes".to_owned());
        }
        self.headers.insert(name.to_owned(), value.to_owned());
        Ok(self)
    }
}

fn split_url(url: &str) -> Result<(&str, &str), String> {
    let Some(index) = url.find("://") else {
        return Err("request URL must include a scheme".to_owned());
    };
    let scheme = &url[..index];
    let rest = &url[index + 3..];
    let host = rest.split(['/', '?', '#', '@']).next().unwrap_or_default();
    let host = host.rsplit(':').next().unwrap_or_default();
    if scheme.is_empty() {
        return Err("request URL has no scheme".to_owned());
    }
    Ok((scheme, host))
}

/// Bounded response returned by a fetcher.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Response {
    /// Final URL after redirects.
    pub url: String,
    /// HTTP status code.
    pub status: u16,
    /// Response headers, lower-case names only.
    pub headers: BTreeMap<String, String>,
    /// Response body bytes.
    pub body: Vec<u8>,
}

impl Response {
    /// Build a response after enforcing the body ceiling.
    ///
    /// # Errors
    /// Returns a diagnostic when the body exceeds `MAX_BODY_BYTES`.
    pub fn new(
        url: impl Into<String>,
        status: u16,
        headers: BTreeMap<String, String>,
        body: Vec<u8>,
    ) -> Result<Self, String> {
        if body.len() > MAX_BODY_BYTES {
            return Err(format!("response body exceeds {MAX_BODY_BYTES} bytes"));
        }
        Ok(Self {
            url: url.into(),
            status,
            headers,
            body,
        })
    }

    /// Return the body as UTF-8 text.
    ///
    /// # Errors
    /// Returns a diagnostic when the body is not valid UTF-8.
    pub fn text(&self) -> Result<&str, String> {
        std::str::from_utf8(&self.body).map_err(|error| format!("invalid UTF-8 body: {error}"))
    }
}

/// A fetcher that can be driven by a parser test.
pub trait Fetcher {
    /// Fetch one validated request.
    ///
    /// # Errors
    /// Returns a diagnostic for transport or policy failures.
    fn fetch(
        &self,
        request: &Request,
    ) -> impl std::future::Future<Output = Result<Response, String>> + Send;
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn validates_request_url() {
        assert!(
            Request::get("ftp://example.test/jobs")
                .unwrap_err()
                .contains("scheme")
        );
        Request::get("https://example.test/jobs").unwrap();
    }

    #[test]
    fn rejects_oversized_response() {
        let body = vec![b'a'; MAX_BODY_BYTES + 1];
        assert!(
            Response::new("https://example.test", 200, BTreeMap::new(), body)
                .unwrap_err()
                .contains("exceeds")
        );
    }

    #[test]
    fn rejects_header_injection() {
        let request = Request::get("https://example.test").expect("valid request");
        assert!(
            request
                .with_header("x-test", "a\r\nb: c")
                .unwrap_err()
                .contains("invalid bytes")
        );
    }
}
