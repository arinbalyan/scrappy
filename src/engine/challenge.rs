// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Arin Balyan
// Last modified: 2026-11-27

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
///
/// Generic words such as `captcha` are omitted because they also appear in
/// ordinary job text.
pub const CHALLENGE_MARKERS: &[&str] = &[
    "cf-browser-verification",
    "challenge-running",
    "datadome",
    "verify you are human",
    "js challenge",
];

/// Markers specific enough to identify a WAF without HTML scaffolding.
const WAF_MARKERS: &[&str] = &["cf-browser-verification", "challenge-running", "datadome"];

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
        "verify you are human" => ChallengeKind::Captcha,
        _ => ChallengeKind::JsRequired,
    }
}

fn header_value<'a>(headers: &'a BTreeMap<String, String>, name: &str) -> Option<&'a str> {
    headers
        .iter()
        .find(|(key, _)| key.eq_ignore_ascii_case(name))
        .map(|(_, value)| value.as_str())
}

fn waf_header_kind(headers: &BTreeMap<String, String>) -> Option<ChallengeKind> {
    if header_value(headers, "x-datadome-status").is_some() {
        return Some(ChallengeKind::DataDome);
    }
    match header_value(headers, "cf-mitigated") {
        Some(value) if value.eq_ignore_ascii_case("challenge") => Some(ChallengeKind::Cloudflare),
        _ => None,
    }
}

fn is_html_content_type(value: &str) -> bool {
    let lower = value.to_ascii_lowercase();
    lower.contains("text/html") || lower.contains("application/xhtml")
}

fn looks_like_html(body: &str) -> bool {
    let trimmed = body.trim_start();
    trimmed.starts_with('<') && trimmed.contains('>')
}

fn starts_with_json(body: &str) -> bool {
    matches!(body.trim_start().as_bytes().first(), Some(b'{' | b'['))
}

fn is_json_response(headers: &BTreeMap<String, String>, body: &str) -> bool {
    let json_content_type = header_value(headers, "content-type")
        .is_some_and(|value| value.to_ascii_lowercase().contains("json"));
    json_content_type || starts_with_json(body)
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
///
/// Body markers are accepted for HTML responses and for WAF-specific markers
/// on error statuses. JSON payloads return `None` unless a known WAF header is
/// present.
#[must_use]
pub fn detect(status: u16, headers: &BTreeMap<String, String>, body: &str) -> Option<Challenge> {
    if let Some(kind) = waf_header_kind(headers) {
        return Some(Challenge {
            kind,
            status,
            evidence: scrub(body),
        });
    }
    if is_json_response(headers, body) {
        return None;
    }
    let lower = body.to_ascii_lowercase();
    let marker = CHALLENGE_MARKERS
        .iter()
        .find(|marker| lower.contains(**marker))?;
    let html = header_value(headers, "content-type").is_some_and(is_html_content_type)
        || looks_like_html(body);
    // WAF-specific markers justify an error response even without HTML.
    let waf_fallback = (400..=599).contains(&status) && WAF_MARKERS.contains(marker);
    if html || waf_fallback {
        return Some(Challenge {
            kind: classify(marker),
            status,
            evidence: scrub(body),
        });
    }
    None
}

#[cfg(test)]
mod tests {
    use super::*;

    fn map_headers(pairs: &[(&str, &str)]) -> BTreeMap<String, String> {
        pairs
            .iter()
            .map(|(name, value)| ((*name).to_owned(), (*value).to_owned()))
            .collect()
    }

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

    struct Case {
        name: &'static str,
        status: u16,
        headers: &'static [(&'static str, &'static str)],
        body: &'static str,
        expected: Option<ChallengeKind>,
    }

    #[test]
    fn table_separates_job_payloads_from_challenges() {
        let cases = [
            Case {
                name: "getonboard json listing",
                status: 200,
                headers: &[("content-type", "application/json; charset=utf-8")],
                body: r#"{"data":[{"id":"301","type":"job","attributes":{"title":"Backend Engineer","description":"We solve CAPTCHA challenges for clients.","company":"Example Org"},"links":{"public_url":"https://jobs.example.test/postings/301"}}]}"#,
                expected: None,
            },
            Case {
                name: "getonboard json without content type",
                status: 200,
                headers: &[],
                body: r#"{"data":[{"id":"301","attributes":{"description":"captcha"}}]}"#,
                expected: None,
            },
            Case {
                name: "gunio html listing",
                status: 200,
                headers: &[("content-type", "text/html; charset=utf-8")],
                body: "<!doctype html><html lang=\"en\"><body><h1>High-stakes engineering roles</h1><div class=\"job-tile\"><h2><a href=\"https://app.gun.io/jobs/ai-ml-backend-engineer\">AI/ML backend engineer</a></h2><p>Work on CAPTCHA mitigation.</p><span class=\"job-tile__company\">Gun.io</span></div></body></html>",
                expected: None,
            },
            Case {
                name: "internshala html listing",
                status: 200,
                headers: &[("content-type", "text/html; charset=utf-8")],
                body: "<!doctype html><html><body><div class=\"individual_internship\"><a class=\"job-title-href\" href=\"/jobs/801/backend-engineer\">Backend Engineer</a><p>Review CAPTCHA flows.</p><p class=\"company-name\">Example Co</p></div></body></html>",
                expected: None,
            },
            Case {
                name: "plain human-check text outside html",
                status: 200,
                headers: &[],
                body: "verify you are human",
                expected: None,
            },
            Case {
                name: "cloudflare interstitial",
                status: 403,
                headers: &[("content-type", "text/html; charset=utf-8")],
                body: "<!DOCTYPE html><html><head><title>Just a moment...</title></head><body><div id=\"cf-browser-verification\"></div></body></html>",
                expected: Some(ChallengeKind::Cloudflare),
            },
            Case {
                name: "captcha interstitial",
                status: 403,
                headers: &[("content-type", "text/html")],
                body: "<!DOCTYPE html><html><body><h1>Verify you are human</h1></body></html>",
                expected: Some(ChallengeKind::Captcha),
            },
            Case {
                name: "datadome header",
                status: 403,
                headers: &[("x-datadome-status", "blocked")],
                body: "blocked",
                expected: Some(ChallengeKind::DataDome),
            },
        ];

        for case in cases {
            let name = case.name;
            let actual = detect(case.status, &map_headers(case.headers), case.body);
            assert_eq!(
                actual.map(|challenge| challenge.kind),
                case.expected,
                "{name}"
            );
        }
    }

    #[test]
    fn evidence_drops_secrets_and_queries() {
        let secret = "abcdefghijklmnopqrstuvwxyz1234";
        let body =
            format!("<html><body>verify you are human?token={secret} please verify</body></html>");
        let headers = map_headers(&[("content-type", "text/html; charset=utf-8")]);
        let challenge = detect(200, &headers, &body).expect("challenge detected");
        assert!(!challenge.evidence.contains(secret));
        assert!(challenge.evidence.contains("query-redacted"));
        assert!(challenge.evidence.len() <= MAX_EVIDENCE_CHARS);
    }
}
