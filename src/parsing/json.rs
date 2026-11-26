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
        current = match current {
            Value::Object(map) => map.get(segment)?,
            Value::Array(items) => {
                let index = segment.parse::<usize>().ok()?;
                items.get(index)?
            }
            _ => return None,
        };
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

/// Select a nested value by object key or numeric array index.
fn envelope<'a>(value: &'a Value, path: &[String]) -> Option<&'a Value> {
    let mut current = value;
    for segment in path {
        current = match current {
            Value::Object(map) => map.get(segment)?,
            Value::Array(items) => {
                let index = segment.parse::<usize>().ok()?;
                items.get(index)?
            }
            _ => return None,
        };
    }
    Some(current)
}

/// Parser for a configured JSON envelope path with field aliases.
/// Parser for a JSON blob embedded in an HTML response.
#[derive(Debug, Clone, Default)]
pub struct EmbeddedJsonParser {
    element_id: Option<String>,
    variable: Option<String>,
    path: Vec<String>,
    aliases: Option<BTreeMap<String, Vec<String>>>,
}

impl EmbeddedJsonParser {
    /// Build a parser for an element id or variable assignment.
    ///
    /// # Errors
    /// Returns a diagnostic when neither an element id nor a variable name
    /// is supplied or when an alias name is empty.
    pub fn new(
        element_id: Option<String>,
        variable: Option<String>,
        path: Vec<String>,
        aliases: Option<BTreeMap<String, Vec<String>>>,
    ) -> Result<Self, String> {
        if element_id.is_none() && variable.is_none() {
            return Err("embedded parser needs an element id or variable".to_owned());
        }
        if let Some(map) = &aliases {
            for (field, names) in map {
                if field.trim().is_empty() || names.iter().any(|name| name.trim().is_empty()) {
                    return Err("field aliases must have non-empty names".to_owned());
                }
            }
        }
        Ok(Self {
            element_id,
            variable,
            path,
            aliases,
        })
    }
}

impl Parser for EmbeddedJsonParser {
    fn parse(&self, response: &Response) -> Result<Vec<Record>, String> {
        let text = response.text()?;
        let blob = self
            .element_id
            .as_deref()
            .and_then(|id| element_json(text, id))
            .or_else(|| {
                self.variable
                    .as_deref()
                    .and_then(|name| variable_json(text, name))
            })
            .ok_or("embedded JSON source not found")?;
        let value: Value = serde_json::from_str(&blob)
            .map_err(|error| format!("invalid embedded JSON: {error}"))?;
        let selected = if self.path.is_empty() {
            &value
        } else {
            envelope(&value, &self.path).ok_or("embedded JSON envelope not found")?
        };
        let items = selected
            .as_array()
            .ok_or("embedded JSON envelope is not an array")?;
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

fn element_json(text: &str, id: &str) -> Option<String> {
    let marker = format!("id=\"{id}\"");
    let index = text.find(&marker)?;
    let tail = &text[index + marker.len()..];
    let start = tail.find('>')? + 1;
    let content = &tail[start..];
    let end = content.find("</script>").unwrap_or(content.len());
    let chunk = content[..end].trim();
    (chunk.starts_with('{') || chunk.starts_with('[')).then(|| chunk.to_owned())
}

fn variable_json(text: &str, name: &str) -> Option<String> {
    let marker = format!("var {name} =");
    let index = text
        .find(&marker)
        .or_else(|| text.find(&format!("let {name} =")))?;
    let tail = &text[index..];
    let start = tail.find(['{', '['])?;
    let mut depth = 0usize;
    let mut in_string = false;
    let mut escaped = false;
    for (offset, byte) in tail.as_bytes()[start..].iter().copied().enumerate() {
        if in_string {
            if escaped {
                escaped = false;
            } else if byte == b'\\' {
                escaped = true;
            } else if byte == b'"' {
                in_string = false;
            }
            continue;
        }
        match byte {
            b'"' => in_string = true,
            b'{' | b'[' => depth += 1,
            b'}' | b']' => {
                depth = depth.checked_sub(1)?;
                if depth == 0 {
                    let end = start + offset + 1;
                    return Some(tail[start..end].to_owned());
                }
            }
            _ => {}
        }
    }
    None
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

    #[test]
    fn parses_element_embedded_json() {
        let body = r#"<html><script id="__NEXT_DATA__" type="application/json">{"props":{"jobs":[{"title":"Engineer","company":"Acme","url":"https://example.test/1"}]}}</script></html>"#;
        let parser = EmbeddedJsonParser::new(
            Some("__NEXT_DATA__".to_owned()),
            None,
            vec!["props".to_owned(), "jobs".to_owned()],
            None,
        )
        .expect("parser built");
        let records = parser.parse(&response(body)).expect("records parsed");
        assert_eq!(records[0].company, "Acme");
    }

    #[test]
    fn parses_variable_embedded_json() {
        let body = r#"<html><script>var Stash = {"jobs":[{"title":"Engineer","company":"Acme","url":"https://example.test/2"}]};</script></html>"#;
        let parser = EmbeddedJsonParser::new(
            None,
            Some("Stash".to_owned()),
            vec!["jobs".to_owned()],
            None,
        )
        .expect("parser built");
        let records = parser.parse(&response(body)).expect("records parsed");
        assert_eq!(records[0].title, "Engineer");
    }
}
