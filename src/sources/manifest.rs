//! Versioned source manifest schema.
//!
//! The manifest contract is specified in
//! openspec/changes/oculus-foundation/specs/source-registry/spec.md.

// Rust guideline compliant 2026-10-09

use std::collections::BTreeMap;
use std::fmt;

use serde::{Deserialize, Serialize};

/// Current manifest schema version.
pub const MANIFEST_VERSION: u32 = 1;

/// Maximum number of aliases or selector rules accepted in a manifest.
pub const MAX_MAPPING_ENTRIES: usize = 64;

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

/// Ordered field aliases for one normalized record field.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct FieldAliases {
    /// Field names tried in order against a parser payload.
    #[serde(default)]
    pub names: Vec<String>,
}

/// Selector rules used by HTML-style parsers.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct Selectors {
    /// CSS selector that matches one repeating item.
    #[serde(default)]
    pub item: Option<String>,
    /// CSS selector that matches the title within an item.
    #[serde(default)]
    pub title: Option<String>,
    /// CSS selector that matches the company within an item.
    #[serde(default)]
    pub company: Option<String>,
    /// CSS selector that matches the URL within an item.
    #[serde(default)]
    pub url: Option<String>,
    /// Attribute read from the URL selector.
    #[serde(default)]
    pub url_attribute: Option<String>,
}

/// Optional parser settings that stay opaque to the manifest schema.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct ParserOptions {
    /// Names of parser-specific options preserved for future adapters.
    #[serde(default)]
    pub enabled: Vec<String>,
}

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
    /// Named request template applied to `request`, if any.
    #[serde(default)]
    pub request_template: Option<String>,
    /// Parser kind selected for responses.
    pub parser: String,
    /// Ordered aliases for normalized record fields.
    #[serde(default)]
    pub field_aliases: Option<BTreeMap<String, FieldAliases>>,
    /// CSS selectors for HTML-style parsers.
    #[serde(default)]
    pub selectors: Option<Selectors>,
    /// Extra parser settings carried by the manifest.
    #[serde(default)]
    pub parser_options: Option<ParserOptions>,
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

impl Default for Manifest {
    fn default() -> Self {
        Self {
            version: MANIFEST_VERSION,
            id: String::new(),
            name: String::new(),
            family: Family::FeedsApi,
            health: Health::Working,
            request: String::new(),
            request_template: None,
            parser: String::new(),
            field_aliases: None,
            selectors: None,
            parser_options: None,
            fetcher: String::new(),
            pagination: String::new(),
            profile: String::new(),
            requires_js: false,
            requests_per_minute: 1,
            max_concurrency: 1,
            robots: String::new(),
            attribution: String::new(),
            credentials: Vec::new(),
            slugs: Vec::new(),
            fixtures: Vec::new(),
        }
    }
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
    if let Some(name) = &manifest.request_template
        && (name.trim().is_empty()
            || !name
                .bytes()
                .all(|byte| byte.is_ascii_lowercase() || byte.is_ascii_digit() || byte == b'-'))
    {
        return Err(ManifestError {
            message: format!(
                "manifest {} has an invalid request template name",
                manifest.id
            ),
        });
    }
    validate_adapter_fields(manifest)
}

fn validate_adapter_fields(manifest: &Manifest) -> Result<(), ManifestError> {
    if let Some(aliases) = &manifest.field_aliases {
        if aliases.len() > MAX_MAPPING_ENTRIES {
            return Err(ManifestError {
                message: format!("manifest {} has too many field alias entries", manifest.id),
            });
        }
        for (field, alias) in aliases {
            if field.trim().is_empty()
                || !field
                    .bytes()
                    .all(|byte| byte.is_ascii_lowercase() || byte.is_ascii_digit() || byte == b'_')
            {
                return Err(ManifestError {
                    message: format!("manifest {} has an invalid alias field", manifest.id),
                });
            }
            if alias.names.is_empty() || alias.names.len() > MAX_MAPPING_ENTRIES {
                return Err(ManifestError {
                    message: format!(
                        "manifest {} has an invalid alias count for {field}",
                        manifest.id
                    ),
                });
            }
            if alias.names.iter().any(|name| name.trim().is_empty()) {
                return Err(ManifestError {
                    message: format!("manifest {} has an empty alias for {field}", manifest.id),
                });
            }
        }
    }
    if let Some(selectors) = &manifest.selectors {
        for (name, value) in [
            ("item", &selectors.item),
            ("title", &selectors.title),
            ("company", &selectors.company),
            ("url", &selectors.url),
            ("url_attribute", &selectors.url_attribute),
        ] {
            if value.as_ref().is_some_and(|value| value.trim().is_empty()) {
                return Err(ManifestError {
                    message: format!("manifest {} has an empty {name} selector", manifest.id),
                });
            }
        }
    }
    if let Some(options) = &manifest.parser_options
        && (options.enabled.len() > MAX_MAPPING_ENTRIES
            || options.enabled.iter().any(|name| name.trim().is_empty()))
    {
        return Err(ManifestError {
            message: format!("manifest {} has invalid parser options", manifest.id),
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

    #[test]
    fn parses_optional_adapter_fields() {
        let text = format!(
            "{VALID}\nrequest_template = \"json-api\"\n\
             [field_aliases.title]\nnames = [\"position\", \"job_title\"]\n\
             [selectors]\nitem = \".job\"\ntitle = \"a\"\n\
             [parser_options]\nenabled = [\"strict\"]\n"
        );
        let manifest = parse(&text).expect("manifest with adapter fields");
        assert_eq!(manifest.request_template.as_deref(), Some("json-api"));
        assert_eq!(
            manifest
                .field_aliases
                .as_ref()
                .and_then(|aliases| aliases.get("title"))
                .map(|alias| alias.names.as_slice()),
            Some(["position".to_owned(), "job_title".to_owned()].as_slice())
        );
        assert_eq!(
            manifest.selectors.as_ref().and_then(|s| s.item.as_deref()),
            Some(".job")
        );
        assert_eq!(
            manifest
                .parser_options
                .as_ref()
                .map(|options| options.enabled.as_slice()),
            Some(["strict".to_owned()].as_slice())
        );
    }

    #[test]
    fn leaves_adapter_fields_unset_by_default() {
        let manifest = parse(VALID).expect("manifest without adapter fields");
        assert!(manifest.request_template.is_none());
        assert!(manifest.field_aliases.is_none());
        assert!(manifest.selectors.is_none());
        assert!(manifest.parser_options.is_none());
    }

    #[test]
    fn rejects_invalid_request_template_name() {
        let text = format!("{VALID}\nrequest_template = \"Bad Template\"\n");
        let error = parse(&text).unwrap_err();
        assert!(error.message.contains("request template"));
    }

    #[test]
    fn rejects_empty_field_alias() {
        let text = format!("{VALID}\n[field_aliases.title]\nnames = []\n");
        let error = parse(&text).unwrap_err();
        assert!(error.message.contains("alias count"));
    }

    #[test]
    fn rejects_empty_selector() {
        let text = format!("{VALID}\n[selectors]\nitem = \"\"\n");
        let error = parse(&text).unwrap_err();
        assert!(error.message.contains("selector"));
    }
}
