//! Plain HTTP fetcher using reqwest with rustls.
//!
//! See documentation/006-architecture.md.

// Rust guideline compliant 2026-10-09

use std::time::Duration;

use reqwest::header::{HeaderMap, HeaderName, HeaderValue, LOCATION};

use crate::security::validate_target;

use super::{Fetcher, MAX_BODY_BYTES, Request, Response};

/// Fetcher for public APIs and static feeds.
#[derive(Debug, Clone, Default)]
pub struct PlainFetcher {
    timeout: Option<Duration>,
}

impl PlainFetcher {
    /// Create a fetcher with the default timeout.
    #[must_use]
    pub fn new() -> Self {
        Self::default()
    }

    /// Override the request timeout.
    #[must_use]
    pub fn with_timeout(mut self, timeout: Duration) -> Self {
        self.timeout = Some(timeout);
        self
    }
}

impl Fetcher for PlainFetcher {
    async fn fetch(&self, request: &Request) -> Result<Response, String> {
        let client = reqwest::Client::builder()
            .default_headers(headers(request)?)
            .dns_resolver(SafeResolver)
            .redirect(reqwest::redirect::Policy::none())
            .timeout(self.timeout.unwrap_or(request.timeout))
            .build()
            .map_err(|error| format!("cannot build HTTP client: {error}"))?;
        let method = reqwest::Method::from_bytes(request.method.as_bytes())
            .map_err(|error| format!("invalid HTTP method: {error}"))?;
        let mut current = request.url.clone();
        let mut response = None;
        for _ in 0..=MAX_REDIRECTS {
            validate_target(&current)?;
            let sent = client
                .request(method.clone(), &current)
                .body(request.body.clone())
                .send()
                .await
                .map_err(|error| format!("request failed: {error}"))?;
            if !sent.status().is_redirection() {
                response = Some(sent);
                break;
            }
            let location = sent
                .headers()
                .get(LOCATION)
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

/// Maximum redirects followed in one fetch.
///
/// Ten hops is enough for ordinary redirect chains and bounds work done by an
/// untrusted response.
const MAX_REDIRECTS: usize = 10;

/// Read a bounded body so oversized responses are rejected before they are kept.
///
/// # Errors
/// Returns a diagnostic when the body cannot be read.
async fn bounded_body(response: reqwest::Response) -> Result<Vec<u8>, String> {
    let mut stream = response;
    let mut body = Vec::new();
    while let Some(chunk) = stream
        .chunk()
        .await
        .map_err(|error| format!("cannot read response body: {error}"))?
    {
        if body.len() + chunk.len() > MAX_BODY_BYTES {
            return Err(format!("response body exceeds {MAX_BODY_BYTES} bytes"));
        }
        body.extend_from_slice(&chunk);
    }
    Ok(body)
}

/// DNS resolver that permits only vetted addresses.
#[derive(Debug)]
struct SafeResolver;

impl reqwest::dns::Resolve for SafeResolver {
    fn resolve(&self, name: reqwest::dns::Name) -> reqwest::dns::Resolving {
        let host = name.as_str().to_owned();
        Box::pin(async move {
            let addresses = crate::security::resolve_public_host(&host)?;
            let addrs: Vec<_> = addresses
                .into_iter()
                .map(|address| std::net::SocketAddr::new(address, 0))
                .collect();
            Ok(Box::new(addrs.into_iter()) as reqwest::dns::Addrs)
        })
    }
}

/// Join a base directory and a relative redirect location.
fn join_relative(directory: &str, location: &str) -> String {
    if directory.ends_with('/') {
        format!("{directory}{location}")
    } else {
        format!("{directory}/{location}")
    }
}

/// Resolve a relative redirect location against the current URL.
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

fn headers(request: &Request) -> Result<HeaderMap, String> {
    let mut map = HeaderMap::new();
    for (name, value) in &request.headers {
        let name = HeaderName::from_bytes(name.as_bytes())
            .map_err(|error| format!("invalid header name: {error}"))?;
        let value = HeaderValue::from_str(value)
            .map_err(|error| format!("invalid header value: {error}"))?;
        map.insert(name, value);
    }
    Ok(map)
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::{Read, Write};
    use std::net::TcpListener;

    #[test]
    fn join_relative_avoids_double_slash() {
        assert_eq!(join_relative("/", "next"), "/next");
        assert_eq!(join_relative("/jobs", "next"), "/jobs/next");
        assert_eq!(join_relative("/jobs/", "next"), "/jobs/next");
    }

    #[test]
    fn resolves_relative_redirect_location() {
        let target = resolve_location("https://93.184.216.34/jobs", "next").unwrap();
        assert_eq!(target, "https://93.184.216.34/next");
        let target = resolve_location("https://93.184.216.34/jobs/page", "next").unwrap();
        assert_eq!(target, "https://93.184.216.34/jobs/next");
    }

    #[test]
    fn fetches_from_local_server() {
        let listener = TcpListener::bind("127.0.0.1:0").expect("listener bound");
        let address = listener.local_addr().expect("local address");
        let thread = std::thread::spawn(move || {
            let (mut stream, _) = listener.accept().expect("connection accepted");
            let mut buffer = [0u8; 1024];
            let _ = stream.read(&mut buffer).expect("request read");
            let body = b"[{\"title\":\"Engineer\",\"company\":\"Acme\",\"url\":\"https://example.test/1\"}]";
            let response = format!(
                "HTTP/1.1 200 OK\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
                body.len()
            );
            stream
                .write_all(response.as_bytes())
                .expect("headers written");
            stream.write_all(body).expect("body written");
        });
        let runtime = tokio::runtime::Builder::new_current_thread()
            .enable_all()
            .build()
            .expect("runtime built");
        let request = Request::get(format!("http://{address}/jobs")).expect("valid request");
        let response = runtime
            .block_on(PlainFetcher::new().fetch(&request))
            .expect("response fetched");
        assert_eq!(response.status, 200);
        assert!(response.text().expect("valid text").contains("Engineer"));
        thread.join().expect("server thread finished");
    }
}
