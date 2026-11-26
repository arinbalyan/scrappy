//! Shared emulation profile headers and feature-gated HTTP fetcher.
//!
//! See documentation/006-architecture.md.

// Rust guideline compliant 2026-10-09

use std::collections::BTreeMap;

#[cfg(feature = "emulation")]
use super::{Fetcher, Request, Response};

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
            .build()
            .map_err(|error| format!("cannot build emulation client: {error}"))?;
        let mut builder = client.get(&request.url);
        for (name, value) in &request.headers {
            builder = builder.header(name, value);
        }
        let response = builder
            .send()
            .await
            .map_err(|error| format!("emulated request failed: {error}"))?;
        let status = response.status().as_u16();
        let url = response.uri().to_string();
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
            .map_err(|error| format!("cannot read emulated body: {error}"))?
            .to_vec();
        Response::new(url, status, headers, body)
    }
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
