//! Thread-safe proxy selection with health feedback.
//!
//! See documentation/010-proxy.md.

// Rust guideline compliant 2026-10-09

use std::collections::BTreeSet;
use std::sync::Mutex;

/// A configured proxy and its observed health.
#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord)]
pub struct Proxy {
    /// Proxy URL with secret query values redacted by the caller.
    pub url: String,
}

impl Proxy {
    /// Create a proxy entry from a URL.
    ///
    /// # Errors
    /// Returns a diagnostic for a non-http scheme or an empty host.
    pub fn new(url: impl Into<String>) -> Result<Self, String> {
        let url = url.into();
        let (scheme, host) = url
            .split_once("://")
            .ok_or("proxy URL must include a scheme")?;
        if scheme != "http" && scheme != "https" {
            return Err(format!("unsupported proxy scheme: {scheme}"));
        }
        if host.is_empty() {
            return Err("proxy URL has no host".to_owned());
        }
        Ok(Self { url })
    }
}

/// Selects healthy proxies in stable order and records failures.
#[derive(Debug, Default)]
pub struct ProxySelector {
    proxies: Vec<Proxy>,
    rejected: Mutex<BTreeSet<String>>,
}

impl ProxySelector {
    /// Build a selector from configured proxies.
    ///
    /// # Errors
    /// Returns the first invalid proxy URL.
    pub fn new(proxies: Vec<String>) -> Result<Self, String> {
        let proxies = proxies
            .into_iter()
            .map(Proxy::new)
            .collect::<Result<Vec<_>, _>>()?;
        Ok(Self {
            proxies,
            rejected: Mutex::new(BTreeSet::new()),
        })
    }

    /// Return the next healthy proxy, or none when no proxy is configured.
    #[must_use]
    pub fn next(&self) -> Option<Proxy> {
        let rejected = self.rejected.lock().ok()?;
        self.proxies
            .iter()
            .find(|proxy| !rejected.contains(&proxy.url))
            .cloned()
    }

    /// Mark a proxy unhealthy after a failed request.
    pub fn reject(&self, url: &str) {
        if let Ok(mut rejected) = self.rejected.lock() {
            rejected.insert(url.to_owned());
        }
    }

    /// Return the number of rejected proxies.
    #[must_use]
    pub fn rejected_count(&self) -> usize {
        self.rejected.lock().map_or(0, |rejected| rejected.len())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn rejected_proxies_are_not_reused() {
        let selector = ProxySelector::new(vec![
            "http://proxy-a.test:8080".to_owned(),
            "http://proxy-b.test:8080".to_owned(),
        ])
        .expect("valid proxies");
        let first = selector.next().expect("first proxy");
        selector.reject(&first.url);
        let second = selector.next().expect("second proxy");
        assert_ne!(first.url, second.url);
        assert_eq!(selector.rejected_count(), 1);
    }

    #[test]
    fn rejects_invalid_proxy() {
        assert!(
            ProxySelector::new(vec!["ftp://proxy.test".to_owned()])
                .unwrap_err()
                .contains("scheme")
        );
    }
}
