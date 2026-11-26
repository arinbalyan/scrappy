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

/// Health state reported by the source listing.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum Health {
    /// Source returns records in normal operation.
    Working,
    /// Source needs a credential that may be absent.
    Keyed,
    /// Source is blocked by a challenge or policy.
    Blocked,
    /// Source endpoint is known to be stale.
    Stale,
    /// Source needs a slug seed to run.
    RequiresSlug,
    /// Source needs a browser to render.
    BrowserOnly,
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
    /// Expected health state before live validation.
    pub health: Health,
    /// Request template used by the fetcher.
    pub request: String,
    /// Parser kind selected for responses.
    pub parser: String,
    /// Fetcher kind selected for requests.
    pub fetcher: String,
    /// Pagination strategy, or `none`.
    pub pagination: String,
    /// Emulation profile name, or `none`.
    pub profile: String,
    /// Whether the source needs a rendered page.
    pub requires_js: bool,
    /// Requests per minute allowed for this source.
    pub requests_per_minute: usize,
    /// Maximum concurrent requests for this source.
    pub max_concurrency: usize,
    /// Robots policy decision.
    pub robots: String,
    /// Attribution text for the source.
    pub attribution: String,
    /// Credential names required by this source, if any.
    #[serde(default)]
    pub credentials: Vec<String>,
    /// Slug seeds for slug-dependent sources.
    #[serde(default)]
    pub slugs: Vec<String>,
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
/// an empty required value, or an invalid limit.
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
        ("pagination", &manifest.pagination),
        ("profile", &manifest.profile),
        ("robots", &manifest.robots),
        ("attribution", &manifest.attribution),
    ] {
        if value.trim().is_empty() {
            return Err(ManifestError {
                message: format!("manifest {field} must not be empty"),
            });
        }
    }
    if manifest.requests_per_minute == 0 || manifest.max_concurrency == 0 {
        return Err(ManifestError {
            message: format!(
                "manifest {} has invalid rate or concurrency limits",
                manifest.id
            ),
        });
    }
    if manifest.health == Health::Keyed && manifest.credentials.is_empty() {
        return Err(ManifestError {
            message: format!(
                "manifest {} is keyed but declares no credentials",
                manifest.id
            ),
        });
    }
    if manifest.health == Health::RequiresSlug && manifest.slugs.is_empty() {
        return Err(ManifestError {
            message: format!("manifest {} requires slugs but declares none", manifest.id),
        });
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
health = "working"
request = "https://example.test/jobs"
parser = "json"
fetcher = "plain"
pagination = "none"
profile = "none"
requires_js = false
requests_per_minute = 30
max_concurrency = 2
robots = "allow"
attribution = "Example"
fixtures = ["sample"]
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

    #[test]
    fn keyed_source_requires_credential_name() {
        let text = VALID.replace("health = \"working\"", "health = \"keyed\"");
        let error = parse(&text).unwrap_err();
        assert!(error.message.contains("credentials"));
    }
}
