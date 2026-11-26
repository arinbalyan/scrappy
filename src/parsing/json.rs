//! JSON feed and array parsers.
//!
//! See documentation/006-architecture.md.

// Rust guideline compliant 2026-10-09

use std::collections::BTreeMap;

use serde_json::Value;

use super::{Parser, Record};
use crate::fetching::Response;

/// Parser for JSON arrays and feed objects.
#[derive(Debug, Clone, Copy, Default)]
pub struct JsonParser;

impl JsonParser {
    /// Create a JSON parser.
    #[must_use]
    pub fn new() -> Self {
        Self
    }
}

impl Parser for JsonParser {
    fn parse(&self, response: &Response) -> Result<Vec<Record>, String> {
        let text = response.text()?;
        let value: Value =
            serde_json::from_str(text).map_err(|error| format!("invalid JSON payload: {error}"))?;
        let items = match &value {
            Value::Array(items) => items.as_slice(),
            Value::Object(object) => object
                .get("jobs")
                .and_then(Value::as_array)
                .ok_or("JSON object has no jobs array")?,
            _ => return Err("JSON payload is not an array or object".to_owned()),
        };
        let mut records = Vec::with_capacity(items.len());
        for item in items {
            let Some(record) = record_from_value(item) else {
                continue;
            };
            records.push(record);
        }
        if records.is_empty() && !items.is_empty() {
            return Err("no records with title, company, and URL".to_owned());
        }
        Ok(records)
    }
}

fn nested<'a>(object: &'a serde_json::Map<String, Value>, path: &str) -> Option<&'a Value> {
    let mut segments = path.split('.');
    let mut current: &Value = object.get(segments.next()?)?;
    for segment in segments {
        current = current.get(segment)?;
    }
    Some(current)
}

fn alias_field(object: &serde_json::Map<String, Value>, names: &[String]) -> Option<String> {
    names.iter().find_map(|name| {
        nested(object, name)
            .and_then(Value::as_str)
            .map(str::trim)
            .filter(|value| !value.is_empty())
            .map(str::to_owned)
    })
}

/// Build one record from ordered aliases, including dotted nested paths.
#[must_use]
pub fn record_from_aliases(
    value: &Value,
    aliases: Option<&BTreeMap<String, Vec<String>>>,
) -> Option<Record> {
    let object = value.as_object()?;
    let field = |name: &str, defaults: &[&str]| -> Option<String> {
        aliases
            .and_then(|map| map.get(name))
            .and_then(|names| alias_field(object, names))
            .or_else(|| {
                defaults
                    .iter()
                    .find_map(|key| object.get(*key))
                    .and_then(Value::as_str)
                    .map(str::trim)
                    .filter(|value| !value.is_empty())
                    .map(str::to_owned)
            })
    };
    let url = field(
        "url",
        &["url", "link", "apply_url", "job_url", "public_url"],
    )?;
    let title = field(
        "title",
        &["title", "position", "job_title", "jobTitle", "name"],
    )?;
    let company = field(
        "company",
        &["company", "company_name", "organization", "company.name"],
    )?;
    let location = field(
        "location",
        &[
            "location",
            "candidate_required_location",
            "city",
            "locations.0.name",
        ],
    );
    let description = field(
        "description",
        &["description", "text", "content", "jobDescription"],
    );
    Record::new(url, title, company)
        .ok()
        .map(|record| record.with_location(location).with_description(description))
}

/// Build one record from a JSON object using common feed field names.
#[must_use]
pub fn record_from_value(value: &Value) -> Option<Record> {
    record_from_aliases(value, None)
}

fn envelope<'a>(value: &'a Value, path: &[String]) -> Option<&'a Value> {
    let mut current = value;
    for segment in path {
        current = current.get(segment)?;
    }
    Some(current)
}

/// Parser for a configured JSON envelope path with field aliases.
#[derive(Debug, Clone, Default)]
pub struct ConfigurableJsonParser {
    path: Vec<String>,
    aliases: Option<BTreeMap<String, Vec<String>>>,
}

impl ConfigurableJsonParser {
    /// Build a parser for a dot-separated envelope path and field aliases.
    ///
    /// # Errors
    /// Returns a diagnostic when an alias name is empty.
    pub fn new(
        path: &[String],
        aliases: Option<BTreeMap<String, Vec<String>>>,
    ) -> Result<Self, String> {
        if let Some(map) = &aliases {
            for (field, names) in map {
                if field.trim().is_empty() || names.iter().any(|name| name.trim().is_empty()) {
                    return Err("field aliases must have non-empty names".to_owned());
                }
            }
        }
        Ok(Self {
            path: path.to_vec(),
            aliases,
        })
    }
}

impl Parser for ConfigurableJsonParser {
    fn parse(&self, response: &Response) -> Result<Vec<Record>, String> {
        let text = response.text()?;
        let value: Value =
            serde_json::from_str(text).map_err(|error| format!("invalid JSON payload: {error}"))?;
        let selected = envelope(&value, &self.path)
            .ok_or_else(|| format!("JSON envelope {} not found", self.path.join(".")))?;
        let items = selected.as_array().ok_or("JSON envelope is not an array")?;
        let mut records = Vec::with_capacity(items.len());
        for item in items {
            if let Some(record) = record_from_aliases(item, self.aliases.as_ref()) {
                records.push(record);
            }
        }
        if records.is_empty() && !items.is_empty() {
            return Err("no records with title, company, and URL".to_owned());
        }
        Ok(records)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn response(text: &str) -> Response {
        Response::new(
            "https://example.test/jobs",
            200,
            BTreeMap::new(),
            text.as_bytes().to_vec(),
        )
        .expect("bounded response")
    }

    #[test]
    fn parses_json_array() {
        let body = r#"[{"title":"Engineer","company":"Acme","url":"https://example.test/1"}]"#;
        let records = JsonParser.parse(&response(body)).expect("records parsed");
        assert_eq!(records.len(), 1);
        assert_eq!(records[0].title, "Engineer");
    }

    #[test]
    fn parses_jobs_object() {
        let body = r#"{"jobs":[{"position":"AI Engineer","company_name":"Labs","link":"https://example.test/2"}]}"#;
        let records = JsonParser.parse(&response(body)).expect("records parsed");
        assert_eq!(records.len(), 1);
        assert_eq!(records[0].company, "Labs");
    }

    #[test]
    fn rejects_missing_required_fields() {
        let body = r#"[{"title":"Engineer","company":"Acme"}]"#;
        assert!(
            JsonParser
                .parse(&response(body))
                .unwrap_err()
                .contains("no records")
        );
    }

    #[test]
    fn parses_committed_remoteok_fixture() {
        let root = std::path::Path::new(env!("CARGO_MANIFEST_DIR"));
        let body = std::fs::read(root.join("tests/fixtures/remoteok/response.body"))
            .expect("fixture body exists");
        let fixture = Response::new("https://remoteok.com/api", 200, BTreeMap::new(), body)
            .expect("bounded response");
        let records = JsonParser.parse(&fixture).expect("fixture parsed");
        assert_ne!(records.len(), 0);
        assert!(records.iter().all(|record| !record.title.is_empty()));
    }

    #[test]
    fn parses_nested_envelope_with_aliases() {
        let body = r#"{"data":{"jobs":[{"jobTitle":"Rust Engineer","company":{"name":"Labs"},"public_url":"https://example.test/3"}]}}"#;
        let mut aliases = BTreeMap::new();
        aliases.insert("title".to_owned(), vec!["jobTitle".to_owned()]);
        aliases.insert("company".to_owned(), vec!["company.name".to_owned()]);
        aliases.insert("url".to_owned(), vec!["public_url".to_owned()]);
        let parser =
            ConfigurableJsonParser::new(&["data".to_owned(), "jobs".to_owned()], Some(aliases))
                .expect("parser built");
        let records = parser.parse(&response(body)).expect("records parsed");
        assert_eq!(records[0].company, "Labs");
        assert_eq!(records[0].title, "Rust Engineer");
    }
}
