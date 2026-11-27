// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Arin Balyan
// Last modified: 2026-11-27

//! Redaction of fields and URLs in command diagnostics.
//!
//! Tracing values pass through `mask_field` by field name, while URL values
//! use the redaction helpers in src/security/mod.rs.

// Rust guideline compliant 2026-10-09

use crate::security::{Secret, redact_text, redact_url};

/// Names whose tracing values must never be emitted verbatim.
const SENSITIVE_FIELDS: [&str; 8] = [
    "token",
    "authorization",
    "cookie",
    "email",
    "phone",
    "credential",
    "secret",
    "url",
];

/// Mask a tracing field by its name.
///
/// Sensitive values render as a redaction marker. URL values keep their
/// scheme, host, and path but lose userinfo, query text, and fragments.
#[must_use]
pub fn mask_field(name: &str, value: &str) -> String {
    let field = name.rsplit('.').next().unwrap_or(name).to_ascii_lowercase();
    if field == "url" || field.ends_with("_url") || field.contains(".http.url") {
        return redact_url(value);
    }
    if SENSITIVE_FIELDS
        .iter()
        .any(|sensitive| field.contains(sensitive) || name.to_ascii_lowercase().contains(sensitive))
    {
        return Secret::from(value).to_string();
    }
    value.to_owned()
}

/// Redact a URL before it enters an error or log line.
#[must_use]
pub fn safe_url(url: &str) -> String {
    redact_url(url)
}

/// Redact untrusted text that may carry query tokens or long identifiers.
#[must_use]
pub fn safe_text(text: &str) -> String {
    redact_text(text)
}
