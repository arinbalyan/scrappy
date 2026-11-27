// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Arin Balyan
// Last modified: 2026-11-27

//! Canonical URLs, content hashes, and deduplication.
//!
//! See documentation/011-dedup.md.

// Rust guideline compliant 2026-10-09

use std::collections::BTreeMap;
use std::collections::hash_map::DefaultHasher;
use std::hash::{Hash, Hasher};

/// Tracking parameters removed from every canonical URL.
pub const TRACKING_PARAMS: &[&str] = &[
    "utm_source",
    "utm_medium",
    "utm_campaign",
    "utm_term",
    "utm_content",
    "fbclid",
    "gclid",
    "ref",
];

fn decode(component: &str) -> String {
    let bytes = component.as_bytes();
    let mut out = Vec::with_capacity(bytes.len());
    let mut index = 0;
    while index < bytes.len() {
        if bytes[index] == b'%'
            && index + 2 < bytes.len()
            && let (Some(high), Some(low)) = (
                char::from(bytes[index + 1]).to_digit(16),
                char::from(bytes[index + 2]).to_digit(16),
            )
        {
            let byte = u8::try_from(high * 16 + low).unwrap_or(0);
            out.push(byte);
            index += 3;
            continue;
        }
        out.push(bytes[index]);
        index += 1;
    }
    String::from_utf8_lossy(&out).into_owned()
}

/// Normalize a URL for stable deduplication.
///
/// # Errors
/// Returns a diagnostic for an empty URL or a missing http scheme and host.
pub fn canonical_url(url: &str) -> Result<String, String> {
    let (scheme, rest) = url
        .split_once("://")
        .ok_or_else(|| "URL must include a scheme".to_owned())?;
    if scheme != "http" && scheme != "https" {
        return Err(format!("unsupported URL scheme: {scheme}"));
    }
    let without_fragment = rest.split('#').next().unwrap_or_default();
    let (without_query, query) = without_fragment
        .split_once('?')
        .map_or((without_fragment, ""), |(head, query)| (head, query));
    let (authority, path) = match without_query.split_once('/') {
        Some((authority, path)) => (authority, format!("/{path}")),
        None => (without_query, String::new()),
    };
    if authority.is_empty() {
        return Err("URL has no host".to_owned());
    }
    let host = authority
        .rsplit('@')
        .next()
        .unwrap_or_default()
        .to_ascii_lowercase();
    let mut params = BTreeMap::new();
    for pair in query.split('&') {
        if pair.is_empty() {
            continue;
        }
        let (key, value) = pair.split_once('=').unwrap_or((pair, ""));
        if TRACKING_PARAMS.contains(&&*key.to_ascii_lowercase()) {
            continue;
        }
        params.insert(key.to_owned(), decode(value));
    }
    let mut canonical = format!("{scheme}://{host}{path}");
    if !params.is_empty() {
        let joined = params
            .iter()
            .map(|(key, value)| format!("{key}={value}"))
            .collect::<Vec<_>>()
            .join("&");
        canonical.push('?');
        canonical.push_str(&joined);
    }
    Ok(canonical)
}

/// Content fields used to compute a stable job hash.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ContentFields<'a> {
    /// Canonical job URL.
    pub url: &'a str,
    /// Job title.
    pub title: &'a str,
    /// Company name.
    pub company: &'a str,
    /// Job description.
    pub description: &'a str,
}

/// Compute a stable, non-cryptographic hash for one job.
#[must_use]
pub fn content_hash(fields: &ContentFields<'_>) -> u64 {
    let mut hasher = DefaultHasher::new();
    fields.url.hash(&mut hasher);
    fields.title.trim().to_ascii_lowercase().hash(&mut hasher);
    fields.company.trim().to_ascii_lowercase().hash(&mut hasher);
    fields.description.hash(&mut hasher);
    hasher.finish()
}

/// Deduplicate keys while preserving first-seen order.
#[must_use]
pub fn dedup<I>(keys: I) -> Vec<String>
where
    I: IntoIterator<Item = String>,
{
    let mut seen = std::collections::BTreeSet::new();
    keys.into_iter()
        .filter(|key| seen.insert(key.clone()))
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn canonicalizes_tracking_and_fragment() {
        let first = canonical_url("https://Example.test/jobs/1?utm_source=x&b=2#top")
            .expect("first canonical URL");
        let second = canonical_url("https://example.test/jobs/1?b=2&utm_source=y")
            .expect("second canonical URL");
        assert_eq!(first, second);
        assert_eq!(first, "https://example.test/jobs/1?b=2");
    }

    #[test]
    fn rejects_invalid_url() {
        assert!(
            canonical_url("ftp://example.test")
                .expect_err("unsupported scheme")
                .contains("scheme")
        );
        assert!(
            canonical_url("https://")
                .expect_err("empty host")
                .contains("host")
        );
    }

    #[test]
    fn same_content_produces_same_hash() {
        let fields = ContentFields {
            url: "https://example.test/jobs/1",
            title: "Engineer",
            company: "Acme",
            description: "Build things",
        };
        assert_eq!(content_hash(&fields), content_hash(&fields));
    }

    #[test]
    fn different_content_produces_different_hash() {
        let first = ContentFields {
            url: "https://example.test/jobs/1",
            title: "Engineer",
            company: "Acme",
            description: "Build things",
        };
        let second = ContentFields {
            url: "https://example.test/jobs/2",
            title: "Engineer",
            company: "Acme",
            description: "Build things",
        };
        assert_ne!(content_hash(&first), content_hash(&second));
    }

    #[test]
    fn dedup_preserves_first_order() {
        let result = dedup(["b".to_owned(), "a".to_owned(), "b".to_owned()]);
        assert_eq!(result, ["b", "a"]);
    }

    #[test]
    fn dedup_is_idempotent() {
        let once = dedup(["a".to_owned(), "b".to_owned()]);
        assert_eq!(dedup(once.clone()), once);
    }
}
