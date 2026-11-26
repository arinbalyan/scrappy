//! Shared emulation profile headers for fetchers.
//!
//! See documentation/006-architecture.md.

// Rust guideline compliant 2026-10-09

use std::collections::BTreeMap;

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
}
