// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Arin Balyan
// Last modified: 2026-11-27

//! Tracing field masking for command diagnostics.
//!
//! Values tagged as sensitive or URL-shaped are masked before they enter a
//! formatted diagnostic. See documentation/015-security.md.

// Rust guideline compliant 2026-10-09

use crate::cli::diagnostic::mask_field;

/// Render one tracing-style field after applying its mask.
#[must_use]
pub fn masked_field(name: &str, value: &str) -> String {
    mask_field(name, value)
}

/// Render a set of tracing-style fields as a stable diagnostic line.
#[must_use]
pub fn masked_event(target: &str, level: &str, fields: &[(&str, &str)]) -> String {
    let rendered: Vec<String> = fields
        .iter()
        .map(|(name, value)| format!("{name}={}", mask_field(name, value)))
        .collect();
    format!("{target} {level} {}", rendered.join(" "))
}

/// Render a request URL without userinfo, query text, or fragments.
#[must_use]
pub fn masked_url(url: &str) -> String {
    mask_field("url", url)
}
