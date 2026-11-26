//! Manifest-driven request building and response parsing.
//!
//! Adapters turn a validated manifest plus runtime substitutions into a
//! `Request`, then turn a bounded `Response` into normalized records. They
//! never perform network I/O; callers supply the response. Placeholder names
//! and credential names appear in diagnostics, while credential values never
//! do. See documentation/006-architecture.md.

// Rust guideline compliant 2026-10-09

use std::collections::BTreeMap;

use crate::fetching::{Request, Response};
use crate::parsing::Parser;
use crate::parsing::json::JsonParser;
use crate::parsing::rss::RssParser;
use crate::sources::manifest::{Family, Manifest};

/// Runtime value used while expanding a manifest request template.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Substitution {
    /// Company slug for a `{slug}` placeholder.
    Slug(String),
    /// Credential name and value for a `{key:<name>}` placeholder.
    Key { name: String, value: String },
    /// Query name and value for a `{query:<name>}` placeholder.
    Query { name: String, value: String },
}

/// Runtime inputs that complete a request template.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct TemplateValues {
    /// Slug substituted into `{slug}` placeholders.
    pub slug: Option<String>,
    /// Credential values keyed by declared credential name.
    pub credentials: BTreeMap<String, String>,
    /// Query values keyed by placeholder name.
    pub query: BTreeMap<String, String>,
}

/// Error produced while building a request or parsing a response.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct AdapterError {
    /// Human-readable diagnostic with no credential values.
    pub message: String,
}

impl std::fmt::Display for AdapterError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(&self.message)
    }
}

impl std::error::Error for AdapterError {}

/// Build a validated request from a manifest and runtime substitutions.
///
/// The manifest `request` field supplies the base URL. When
/// `request_template` is set, placeholders in the base URL are expanded:
/// `{slug}` uses the runtime slug, `{key:<name>}` uses a named credential,
/// and `{query:<name>}` uses a named query value. A missing substitution is
/// an error. Credential values are never included in the error message.
///
/// # Errors
/// Returns `AdapterError` for an unsupported family, a missing substitution,
/// a placeholder with no value, or a request rejected by `Request::get`.
pub fn build_request(
    manifest: &Manifest,
    values: &TemplateValues,
) -> Result<Request, AdapterError> {
    if manifest.family != Family::FeedsApi {
        return Err(error(format!(
            "source {} has unsupported adapter family",
            manifest.id
        )));
    }
    let url = render(&manifest.request, values)?;
    let mut request = Request::get(url).map_err(|reason| {
        error(format!(
            "source {} has invalid request URL: {reason}",
            manifest.id
        ))
    })?;
    let accept = if manifest.parser == "rss" {
        "application/rss+xml, application/xml"
    } else {
        "application/json"
    };
    request = request
        .with_header("accept", accept)
        .map_err(|reason| error(format!("cannot add accept header: {reason}")))?;
    Ok(request)
}

/// Build a request using explicit substitutions.
///
/// Each substitution is converted into the matching field of
/// `TemplateValues`; values are never copied into diagnostics.
///
/// # Errors
/// Returns `AdapterError` when `build_request` rejects the manifest or URL.
pub fn request_with(
    manifest: &Manifest,
    substitutions: &[Substitution],
) -> Result<Request, AdapterError> {
    let mut values = TemplateValues::default();
    for item in substitutions {
        match item {
            Substitution::Slug(slug) => values.slug = Some(slug.clone()),
            Substitution::Key { name, value } => {
                values.credentials.insert(name.clone(), value.clone());
            }
            Substitution::Query { name, value } => {
                values.query.insert(name.clone(), value.clone());
            }
        }
    }
    build_request(manifest, &values)
}

/// Parse a bounded response into normalized records using the manifest.
///
/// The manifest selects a `json` or `rss` parser. Parser options are
/// reserved for future parser settings and are not interpreted here. The
/// adapter does not inspect the response URL or status; callers enforce
/// transport policy before passing a response.
///
/// # Errors
/// Returns `AdapterError` for an unsupported parser kind or a parser failure.
pub fn parse_response(
    manifest: &Manifest,
    response: &Response,
) -> Result<Vec<crate::parsing::Record>, AdapterError> {
    match manifest.parser.as_str() {
        "json" => JsonParser
            .parse(response)
            .map_err(|reason| adapter_error(&reason)),
        "rss" => RssParser
            .parse(response)
            .map_err(|reason| adapter_error(&reason)),
        other => Err(error(format!(
            "source {} has unsupported parser kind: {other}",
            manifest.id
        ))),
    }
}

fn adapter_error(reason: &str) -> AdapterError {
    error(format!("response parse failed: {reason}"))
}

fn error(message: String) -> AdapterError {
    AdapterError { message }
}

fn render(template: &str, values: &TemplateValues) -> Result<String, AdapterError> {
    let mut url = String::with_capacity(template.len());
    let mut rest = template;
    while let Some(start) = rest.find('{') {
        let tail = &rest[start + 1..];
        let Some(end) = tail.find('}') else {
            return Err(error("unterminated request placeholder".to_owned()));
        };
        url.push_str(&rest[..start]);
        let name = &tail[..end];
        let value = if name == "slug" {
            values
                .slug
                .as_deref()
                .ok_or_else(|| error("missing slug substitution".to_owned()))?
        } else if let Some(key) = name.strip_prefix("key:") {
            values
                .credentials
                .get(key)
                .map(String::as_str)
                .ok_or_else(|| error(format!("missing credential substitution for {key}")))?
        } else if let Some(query) = name.strip_prefix("query:") {
            values
                .query
                .get(query)
                .map(String::as_str)
                .ok_or_else(|| error(format!("missing query substitution for {query}")))?
        } else {
            return Err(error(format!("unknown request placeholder: {name}")));
        };
        if value
            .bytes()
            .any(|byte| byte.is_ascii_whitespace() || byte == b'{')
        {
            return Err(error(format!(
                "unsafe value for request placeholder: {name}"
            )));
        }
        url.push_str(value);
        rest = &tail[end + 1..];
    }
    url.push_str(rest);
    Ok(url)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn substitutes_slug_and_key_without_leaking_value() {
        let manifest = crate::sources::manifest::parse(
            r#"
version = 1
id = "sample"
name = "Sample"
family = "feeds_api"
health = "working"
request = "https://example.test/{slug}/jobs?key={key:api_key}"
parser = "json"
fetcher = "plain"
pagination = "none"
profile = "none"
requires_js = false
requests_per_minute = 30
max_concurrency = 2
robots = "allow"
attribution = "Example"
"#,
        )
        .expect("manifest parsed");
        let values = TemplateValues {
            slug: Some("acme".to_owned()),
            credentials: BTreeMap::from([("api_key".to_owned(), "secret-value".to_owned())]),
            query: BTreeMap::new(),
        };
        let request = build_request(&manifest, &values).expect("request built");
        assert_eq!(
            request.url,
            "https://example.test/acme/jobs?key=secret-value"
        );
    }
}
