//! Plain HTTP fetcher using reqwest with rustls.
//!
//! See documentation/006-architecture.md.

// Rust guideline compliant 2026-10-09

use std::time::Duration;

use reqwest::header::{HeaderMap, HeaderName, HeaderValue};

use super::{Fetcher, Request, Response};

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
            .timeout(self.timeout.unwrap_or(request.timeout))
            .build()
            .map_err(|error| format!("cannot build HTTP client: {error}"))?;
        let response = client
            .get(&request.url)
            .send()
            .await
            .map_err(|error| format!("request failed: {error}"))?;
        let status = response.status().as_u16();
        let url = response.url().to_string();
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
        let body = response
            .bytes()
            .await
            .map_err(|error| format!("cannot read response body: {error}"))?
            .to_vec();
        Response::new(url, status, headers, body)
    }
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
