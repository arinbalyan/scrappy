//! Challenge detection with sanitized evidence.
//!
//! See documentation/015-security.md.

// Rust guideline compliant 2026-10-09

use std::collections::BTreeMap;

/// Maximum number of evidence characters retained from a challenge body.
pub const MAX_EVIDENCE_CHARS: usize = 120;
/// Length at which a run of identifier characters is treated as a secret.
const REDACTION_THRESHOLD: usize = 24;

/// Markers that identify a bot or WAF challenge.
pub const CHALLENGE_MARKERS: &[&str] = &[
    "cf-browser-verification",
    "challenge-running",
    "datadome",
    "verify you are human",
    "captcha",
    "js challenge",
];

/// Kind of challenge observed for a source.
#[derive(Debug, Clone, Copy, PartialEq, Eq, serde::Serialize)]
pub enum ChallengeKind {
    /// Captcha or human-verification page.
    Captcha,
    /// Cloudflare interstitial.
    Cloudflare,
    /// `DataDome` interstitial.
    DataDome,
    /// JavaScript-required interstitial.
    JsRequired,
    /// Repeated forbidden responses.
    RepeatedForbidden,
    /// Repeated throttled responses.
    RepeatedThrottled,
}

/// A detected challenge with evidence safe to record.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Challenge {
    /// Challenge classification.
    pub kind: ChallengeKind,
    /// HTTP status that carried the challenge.
    pub status: u16,
    /// Sanitized body sample.
    pub evidence: String,
}

fn classify(marker: &str) -> ChallengeKind {
    match marker {
        "cf-browser-verification" | "challenge-running" => ChallengeKind::Cloudflare,
        "datadome" => ChallengeKind::DataDome,
        "captcha" | "verify you are human" => ChallengeKind::Captcha,
        _ => ChallengeKind::JsRequired,
    }
}

fn flush_token(token: &mut String, out: &mut String) {
    if token.len() >= REDACTION_THRESHOLD {
        out.push_str("[redacted]");
    } else {
        out.push_str(token);
    }
    token.clear();
}

/// Remove query strings and long tokens from untrusted text.
#[must_use]
pub fn scrub(text: &str) -> String {
    let mut out = String::with_capacity(text.len().min(MAX_EVIDENCE_CHARS));
    let mut token = String::new();
    let mut in_query = false;
    for ch in text.chars() {
        if in_query {
            if ch.is_whitespace() || matches!(ch, '"' | '\'') {
                in_query = false;
                out.push(ch);
            }
            continue;
        }
        if ch == '?' {
            flush_token(&mut token, &mut out);
            in_query = true;
            out.push_str("[query-redacted]");
        } else if ch.is_ascii_alphanumeric() || matches!(ch, '_' | '-') {
            token.push(ch);
        } else {
            flush_token(&mut token, &mut out);
            out.push(if ch.is_control() { ' ' } else { ch });
        }
    }
    flush_token(&mut token, &mut out);
    out.chars().take(MAX_EVIDENCE_CHARS).collect()
}

/// Detect a challenge in one response without retaining sensitive values.
#[must_use]
pub fn detect(status: u16, headers: &BTreeMap<String, String>, body: &str) -> Option<Challenge> {
    let lower = body.to_ascii_lowercase();
    if let Some(marker) = CHALLENGE_MARKERS
        .iter()
        .find(|marker| lower.contains(**marker))
    {
        return Some(Challenge {
            kind: classify(marker),
            status,
            evidence: scrub(body),
        });
    }
    if status == 403 && headers.get("x-datadome-status").is_some() {
        return Some(Challenge {
            kind: ChallengeKind::DataDome,
            status,
            evidence: scrub(body),
        });
    }
    None
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn detects_cloudflare_challenge() {
        let headers = BTreeMap::new();
        let challenge = detect(403, &headers, "<title>cf-browser-verification</title>")
            .expect("challenge detected");
        assert_eq!(challenge.kind, ChallengeKind::Cloudflare);
        assert_eq!(challenge.status, 403);
    }

    #[test]
    fn detects_datadome_header() {
        let mut headers = BTreeMap::new();
        headers.insert("x-datadome-status".to_owned(), "blocked".to_owned());
        let challenge = detect(403, &headers, "blocked").expect("challenge detected");
        assert_eq!(challenge.kind, ChallengeKind::DataDome);
    }

    #[test]
    fn clean_response_has_no_challenge() {
        let headers = BTreeMap::new();
        assert!(detect(200, &headers, "[{\"title\":\"Engineer\"}]").is_none());
    }

    #[test]
    fn evidence_drops_secrets_and_queries() {
        let secret = "abcdefghijklmnopqrstuvwxyz1234";
        let body = format!("captcha?token={secret} please verify");
        let challenge = detect(200, &BTreeMap::new(), &body).expect("challenge detected");
        assert!(!challenge.evidence.contains(secret));
        assert!(challenge.evidence.contains("query-redacted"));
        assert!(challenge.evidence.len() <= MAX_EVIDENCE_CHARS);
    }
}
