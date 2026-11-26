//! Versioned source manifest schema.
//!
//! The manifest contract is specified in
//! openspec/changes/oculus-foundation/specs/source-registry/spec.md.

// Rust guideline compliant 2026-10-09

use std::fmt;

use serde::{Deserialize, Serialize};

/// Current manifest schema version.
pub const MANIFEST_VERSION: u32 = 1;

/// Adapter family that controls how a source is loaded and fetched.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum Family {
    /// Public JSON or RSS feeds.
    FeedsApi,
    /// Applicant tracking systems with slug-based endpoints.
    Ats,
    /// Static HTML pages parsed without a browser.
    HtmlStatic,
    /// HTML pages that require a browser.
    HtmlRendered,
    /// Sources that require a credential.
    Keyed,
}

/// Error produced when a manifest is invalid.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ManifestError {
    /// Human-readable diagnostic.
    pub message: String,
}

impl fmt::Display for ManifestError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(&self.message)
    }
}

impl std::error::Error for ManifestError {}

/// Declarative description of one source.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct Manifest {
    /// Schema version for forward compatibility.
    pub version: u32,
    /// Stable lowercase source identifier.
    pub id: String,
    /// Human-readable source name.
    pub name: String,
    /// Adapter family for this source.
    pub family: Family,
    /// Request template used by the fetcher.
    pub request: String,
    /// Parser kind selected for responses.
    pub parser: String,
    /// Fetcher kind selected for requests.
    pub fetcher: String,
    /// Credential names required by this source, if any.
    #[serde(default)]
    pub credentials: Vec<String>,
    /// Fixture identifiers used by offline tests.
    #[serde(default)]
    pub fixtures: Vec<String>,
}

/// Parse and validate one manifest document.
///
/// # Errors
/// Returns `ManifestError` for an unsupported version, a malformed document,
/// an invalid identifier, or a missing required field.
pub fn parse(text: &str) -> Result<Manifest, ManifestError> {
    let manifest: Manifest = toml::from_str(text).map_err(|error| ManifestError {
        message: format!("cannot parse manifest: {error}"),
    })?;
    validate(&manifest)?;
    Ok(manifest)
}

/// Validate a manifest after deserialization.
///
/// # Errors
/// Returns `ManifestError` for an unsupported version, an invalid identifier,
/// or an empty required value.
pub fn validate(manifest: &Manifest) -> Result<(), ManifestError> {
    if manifest.version != MANIFEST_VERSION {
        return Err(ManifestError {
            message: format!(
                "unsupported manifest version {}; expected {MANIFEST_VERSION}",
                manifest.version
            ),
        });
    }
    if manifest.id.is_empty()
        || !manifest
            .id
            .bytes()
            .all(|byte| byte.is_ascii_lowercase() || byte.is_ascii_digit() || byte == b'-')
    {
        return Err(ManifestError {
            message: "manifest id must use lowercase letters, digits, or hyphens".to_owned(),
        });
    }
    for (field, value) in [
        ("name", &manifest.name),
        ("request", &manifest.request),
        ("parser", &manifest.parser),
        ("fetcher", &manifest.fetcher),
    ] {
        if value.trim().is_empty() {
            return Err(ManifestError {
                message: format!("manifest {field} must not be empty"),
            });
        }
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    const VALID: &str = r#"
version = 1
id = "sample"
name = "Sample"
family = "feeds_api"
request = "https://example.test/jobs"
parser = "json"
fetcher = "plain"
"#;

    #[test]
    fn parses_valid_manifest() {
        let manifest = parse(VALID).expect("valid manifest");
        assert_eq!(manifest.id, "sample");
    }

    #[test]
    fn rejects_uppercase_id() {
        let text = VALID.replace("sample", "Sample");
        let error = parse(&text).unwrap_err();
        assert_ne!(error.message, "");
    }

    #[test]
    fn rejects_unknown_field() {
        let text = format!("{VALID}\nunexpected = true\n");
        let error = parse(&text).unwrap_err();
        assert_ne!(error.message, "");
    }
}
