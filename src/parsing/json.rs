//! JSON feed and array parsers.
//!
//! See documentation/006-architecture.md.

// Rust guideline compliant 2026-10-09

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

/// Build one record from a JSON object using common feed field names.
#[must_use]
pub fn record_from_value(value: &Value) -> Option<Record> {
    let object = value.as_object()?;
    let field = |names: &[&str]| -> Option<String> {
        names
            .iter()
            .find_map(|name| object.get(*name))
            .and_then(Value::as_str)
            .map(str::trim)
            .filter(|value| !value.is_empty())
            .map(str::to_owned)
    };
    let url = field(&["url", "link", "apply_url", "job_url"])?;
    let title = field(&["title", "position", "job_title"])?;
    let company = field(&["company", "company_name", "organization"])?;
    let location = field(&["location", "candidate_required_location", "city"]);
    let description = field(&["description", "text", "content"]);
    Record::new(url, title, company)
        .ok()
        .map(|record| record.with_location(location).with_description(description))
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::collections::BTreeMap;

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
        let fixture = response_from_bytes(body);
        let records = JsonParser.parse(&fixture).expect("fixture parsed");
        assert_ne!(records.len(), 0);
        assert!(records.iter().all(|record| !record.title.is_empty()));
    }

    fn response_from_bytes(body: Vec<u8>) -> Response {
        Response::new("https://remoteok.com/api", 200, BTreeMap::new(), body)
            .expect("bounded response")
    }
}
