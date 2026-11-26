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
use crate::parsing::html::HtmlParser;
use crate::parsing::json::{
    ConfigurableJsonParser, EmbeddedJsonParser, JsonParser, record_from_aliases,
};
use crate::parsing::rss::{PatternRssParser, RssParser};
use crate::sources::manifest::Manifest;

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
    let url = render(&manifest.request, values)?;
    let method = manifest
        .request_options
        .as_ref()
        .and_then(|options| options.method.as_deref())
        .unwrap_or("GET");
    let mut request = Request::with_method(url, method).map_err(|reason| {
        error(format!(
            "source {} has invalid request URL: {reason}",
            manifest.id
        ))
    })?;
    if let Some(options) = &manifest.request_options {
        for (name, template) in &options.headers {
            let value = render(template, values)?;
            request = request
                .with_header(name, &value)
                .map_err(|reason| error(format!("cannot add {name} header: {reason}")))?;
        }
        if let Some(body) = &options.body {
            let rendered = render(body, values)?;
            request = request
                .with_body(rendered.into_bytes())
                .map_err(|reason| error(format!("cannot add request body: {reason}")))?;
        }
    }
    let accept = if manifest.parser == "rss" {
        "application/rss+xml, application/xml"
    } else {
        "application/json"
    };
    request = request
        .with_header("accept", accept)
        .map_err(|reason| error(format!("cannot add accept header: {reason}")))?;
    if !request.headers.contains_key("content-type") && !request.body.is_empty() {
        request = request
            .with_header("content-type", "application/json")
            .map_err(|reason| error(format!("cannot add content-type header: {reason}")))?;
    }
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
    let options = manifest.parser_options.as_ref();
    let result = match manifest.parser.as_str() {
        "json" if options.is_some_and(|o| o.embedded.is_some()) => {
            let opts = options.ok_or_else(|| error("missing parser options".to_owned()))?;
            let embedded = opts
                .embedded
                .as_ref()
                .ok_or_else(|| error("missing embedded options".to_owned()))?;
            let aliases = manifest.field_aliases.as_ref().map(|map| {
                map.iter()
                    .map(|(field, alias)| (field.clone(), alias.names.clone()))
                    .collect()
            });
            EmbeddedJsonParser::new(
                embedded.element_id.clone(),
                embedded.variable.clone(),
                embedded.envelope.clone(),
                aliases,
            )
            .map_err(error)?
            .parse(response)
        }
        "json" if options.is_some_and(|o| !o.envelope.is_empty()) => {
            let aliases = manifest.field_aliases.as_ref().map(|map| {
                map.iter()
                    .map(|(field, alias)| (field.clone(), alias.names.clone()))
                    .collect()
            });
            let envelope = &options
                .ok_or_else(|| error("missing parser options".to_owned()))?
                .envelope;
            ConfigurableJsonParser::new(envelope, aliases)
                .map_err(error)?
                .parse(response)
        }
        "json" if manifest.field_aliases.is_some() => {
            let value: serde_json::Value =
                serde_json::from_str(response.text().unwrap_or_default())
                    .map_err(|reason| error(format!("invalid JSON payload: {reason}")))?;
            let items = value
                .as_array()
                .ok_or_else(|| error("JSON payload is not an array".to_owned()))?;
            let names: std::collections::BTreeMap<String, Vec<String>> = manifest
                .field_aliases
                .as_ref()
                .ok_or_else(|| error("missing field aliases".to_owned()))?
                .iter()
                .map(|(field, alias)| (field.clone(), alias.names.clone()))
                .collect();
            Ok(items
                .iter()
                .filter_map(|item| record_from_aliases(item, Some(&names)))
                .collect())
        }
        "json" => JsonParser.parse(response),
        "rss" if options.is_some_and(|o| o.title_separator.is_some()) => {
            let opts = options.ok_or_else(|| error("missing parser options".to_owned()))?;
            let separator = opts.title_separator.as_deref().unwrap_or_default();
            PatternRssParser::new(separator, opts.company_first)
                .map_err(error)?
                .parse(response)
        }
        "rss" => RssParser.parse(response),
        "html" if manifest.selectors.is_some() => {
            let selectors = manifest
                .selectors
                .as_ref()
                .ok_or_else(|| error("missing selectors".to_owned()))?;
            HtmlParser::new(
                selectors.item.clone().unwrap_or_default(),
                selectors.title.clone().unwrap_or_default(),
                selectors.company.clone().unwrap_or_default(),
                selectors.url.clone().unwrap_or_default(),
                selectors.url_attribute.clone().unwrap_or_default(),
            )
            .map_err(error)?
            .parse(response)
        }
        other => {
            return Err(error(format!(
                "source {} has unsupported parser kind: {other}",
                manifest.id
            )));
        }
    };
    result.map_err(|reason| adapter_error(&reason))
}

fn adapter_error(reason: &str) -> AdapterError {
    error(format!("response parse failed: {reason}"))
}

fn error(message: String) -> AdapterError {
    AdapterError { message }
}

fn render(template: &str, values: &TemplateValues) -> Result<String, AdapterError> {
    let mut output = String::with_capacity(template.len());
    let bytes = template.as_bytes();
    let mut index = 0;
    while index < bytes.len() {
        if bytes[index] == b'{' && bytes.get(index + 1) == Some(&b'{') {
            output.push('{');
            index += 2;
            continue;
        }
        if bytes[index] == b'}' && bytes.get(index + 1) == Some(&b'}') {
            output.push('}');
            index += 2;
            continue;
        }
        if bytes[index] != b'{' && bytes[index] != b'}' {
            let ch = template[index..]
                .chars()
                .next()
                .expect("character at byte index");
            output.push(ch);
            index += ch.len_utf8();
            continue;
        }
        if bytes[index] == b'}' {
            return Err(error(
                "unmatched closing brace in request template".to_owned(),
            ));
        }
        let end = template[index + 1..]
            .find('}')
            .ok_or_else(|| error("unterminated request placeholder".to_owned()))?
            + index
            + 1;
        let name = &template[index + 1..end];
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
        output.push_str(value);
        index = end + 1;
    }
    Ok(output)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::sources::manifest::Family;

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

    #[test]
    fn builds_post_request_with_rendered_headers_and_body() {
        let text = [
            "version = 1",
            "id = \"sample\"",
            "name = \"Sample\"",
            "family = \"feeds_api\"",
            "health = \"working\"",
            "request = \"https://example.test/{slug}/jobs\"",
            "parser = \"json\"",
            "fetcher = \"plain\"",
            "pagination = \"none\"",
            "profile = \"none\"",
            "requires_js = false",
            "requests_per_minute = 30",
            "max_concurrency = 2",
            "robots = \"allow\"",
            "attribution = \"Example\"",
            "",
            "[request_options]",
            "method = \"post\"",
            "body = \"query={slug}\"",
            "",
            "[request_options.headers]",
            "x-source = \"{slug}\"",
            "x-key = \"{key:api_key}\"",
        ]
        .join("\n");
        let manifest = crate::sources::manifest::parse(&text).expect("manifest parsed");
        assert_eq!(manifest.family, Family::FeedsApi);
        let values = TemplateValues {
            slug: Some("acme".to_owned()),
            credentials: BTreeMap::from([("api_key".to_owned(), "secret-value".to_owned())]),
            query: BTreeMap::new(),
        };
        let request = build_request(&manifest, &values).expect("request built");
        assert_eq!(request.method, "POST");
        assert_eq!(request.body, b"query=acme");
        assert_eq!(
            request.headers.get("x-source").map(String::as_str),
            Some("acme")
        );
        assert_eq!(
            request.headers.get("content-type").map(String::as_str),
            Some("application/json")
        );
        let missing = TemplateValues {
            slug: Some("acme".to_owned()),
            ..TemplateValues::default()
        };
        let error = build_request(&manifest, &missing).unwrap_err();
        assert!(!error.message.contains("secret-value"));
    }

    #[test]
    fn keeps_escaped_json_braces_in_request_bodies() {
        let manifest = crate::sources::manifest::parse(
            r#"
version = 1
id = "sample"
name = "Sample"
family = "feeds_api"
health = "working"
request = "https://example.test/graphql"
parser = "json"
fetcher = "plain"
pagination = "none"
profile = "none"
requires_js = false
requests_per_minute = 30
max_concurrency = 2
robots = "allow"
attribution = "Example"

[request_options]
method = "POST"
body = "{{\"query\":\"jobs(page:1)\"}},{slug}"
"#,
        )
        .expect("manifest parsed");
        let request = build_request(
            &manifest,
            &TemplateValues {
                slug: Some("rust".to_owned()),
                ..TemplateValues::default()
            },
        )
        .expect("request built");
        assert_eq!(request.body, br#"{"query":"jobs(page:1)"},rust"#);
    }

    #[test]
    fn rejects_unmatched_closing_brace_in_request_template() {
        let manifest = crate::sources::manifest::parse(
            r#"
version = 1
id = "sample"
name = "Sample"
family = "feeds_api"
health = "working"
request = "https://example.test/graphql"
parser = "json"
fetcher = "plain"
pagination = "none"
profile = "none"
requires_js = false
requests_per_minute = 30
max_concurrency = 2
robots = "allow"
attribution = "Example"

[request_options]
method = "POST"
body = "bad}body"
"#,
        )
        .expect("manifest parsed");
        let error = build_request(&manifest, &TemplateValues::default()).unwrap_err();
        assert!(error.message.contains("unmatched closing brace"));
    }
}
