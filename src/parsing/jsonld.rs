//! JSON-LD job posting parsers.
//!
//! See documentation/006-architecture.md.

// Rust guideline compliant 2026-10-09

use serde_json::Value;

use super::json::record_from_value;
use super::{Parser, Record};
use crate::fetching::Response;

/// Parser for JSON-LD `JobPosting` blocks embedded in HTML or JSON.
#[derive(Debug, Clone, Copy, Default)]
pub struct JsonLdParser;

impl JsonLdParser {
    /// Create a JSON-LD parser.
    #[must_use]
    pub fn new() -> Self {
        Self
    }
}

impl Parser for JsonLdParser {
    fn parse(&self, response: &Response) -> Result<Vec<Record>, String> {
        let text = response.text()?;
        let mut records = Vec::new();
        for block in blocks(text) {
            let value: Value = serde_json::from_str(&block)
                .map_err(|error| format!("invalid JSON-LD: {error}"))?;
            collect_postings(&value, &mut records);
        }
        if records.is_empty() {
            return Err("no JobPosting blocks found".to_owned());
        }
        Ok(records)
    }
}

fn blocks(text: &str) -> Vec<String> {
    let marker = "application/ld+json";
    let mut remaining = text;
    let mut blocks = Vec::new();
    while let Some(index) = remaining.find(marker) {
        let tail = &remaining[index + marker.len()..];
        let Some(start) = tail.find('>') else { break };
        let content = &tail[start + 1..];
        let Some(end) = content.find("</script>") else {
            break;
        };
        let chunk = content[..end].trim();
        if chunk.starts_with('{') || chunk.starts_with('[') {
            blocks.push(chunk.to_owned());
        }
        remaining = &content[end + "</script>".len()..];
    }
    if blocks.is_empty()
        && (text.trim_start().starts_with('{') || text.trim_start().starts_with('['))
    {
        blocks.push(text.trim().to_owned());
    }
    blocks
}

fn collect_postings(value: &Value, records: &mut Vec<Record>) {
    match value {
        Value::Array(items) => {
            for item in items {
                collect_postings(item, records);
            }
        }
        Value::Object(object) => {
            let kind = object.get("@type").and_then(Value::as_str);
            let kinds = object.get("@type").and_then(Value::as_array);
            let is_job = kind == Some("JobPosting")
                || kinds.is_some_and(|items| {
                    items.iter().any(|item| item.as_str() == Some("JobPosting"))
                });
            if is_job {
                let company = object
                    .get("hiringOrganization")
                    .and_then(|value| value.get("name"))
                    .and_then(Value::as_str)
                    .unwrap_or_default();
                let mut item = object.clone();
                item.insert("company".to_owned(), Value::String(company.to_owned()));
                if let Some(record) = record_from_value(&Value::Object(item)) {
                    records.push(record);
                }
            }
            for (key, item) in object {
                if key == "@graph" || key == "itemListElement" {
                    collect_postings(item, records);
                }
            }
        }
        _ => {}
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::collections::BTreeMap;

    #[test]
    fn parses_embedded_job_posting() {
        let body = r#"<html><script type="application/ld+json">{"@type":"JobPosting","title":"Engineer","url":"https://example.test/1","hiringOrganization":{"name":"Acme"}}</script></html>"#;
        let response = Response::new(
            "https://example.test/jobs",
            200,
            BTreeMap::new(),
            body.as_bytes().to_vec(),
        )
        .expect("bounded response");
        let records = JsonLdParser.parse(&response).expect("records parsed");
        assert_eq!(records[0].company, "Acme");
    }
}
