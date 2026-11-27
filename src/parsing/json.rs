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
                .or_else(|| object.get("items"))
                .and_then(Value::as_array)
                .ok_or("JSON object has no jobs or items array")?,
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
    record_from_aliases_with_default(value, aliases, None)
}

/// Build one record with a fallback company for envelope-level feeds.
#[must_use]
pub fn record_from_aliases_with_default(
    value: &Value,
    aliases: Option<&BTreeMap<String, Vec<String>>>,
    default_company: Option<&str>,
) -> Option<Record> {
    record_from_aliases_with_url(value, aliases, default_company, None, None)
}

/// Build one record with an optional derived URL field.
#[must_use]
pub fn record_from_aliases_with_url(
    value: &Value,
    aliases: Option<&BTreeMap<String, Vec<String>>>,
    default_company: Option<&str>,
    url_from: Option<&str>,
    url_prefix: Option<&str>,
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
    let derived = url_from
        .and_then(|path| nested(object, path))
        .and_then(Value::as_str)
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .map(|id| match url_prefix {
            Some(prefix) if !id.starts_with("https://") => format!("{prefix}{id}"),
            _ => id.to_owned(),
        });
    let url = derived.or_else(|| {
        field(
            "url",
            &["url", "link", "apply_url", "job_url", "public_url"],
        )
    })?;
    let title = field(
        "title",
        &["title", "position", "job_title", "jobTitle", "name"],
    )?;
    let company = field(
        "company",
        &["company", "company_name", "organization", "company.name"],
    )
    .or_else(|| {
        default_company
            .map(str::trim)
            .filter(|value| !value.is_empty())
            .map(str::to_owned)
    })
    .unwrap_or_else(|| "unknown".to_owned());
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
    let extra = extra_fields(object, aliases);
    Record::new(url, title, company).ok().map(|record| {
        record
            .with_location(location)
            .with_description(description)
            .with_date_posted(extra.date_posted)
            .with_compensation(extra.compensation)
            .with_apply_method(extra.apply_method)
            .with_job_url_direct(extra.job_url_direct)
            .with_company_url(extra.company_url)
            .with_industry(extra.industry)
            .with_is_remote(extra.is_remote)
    })
}

struct ExtraFields {
    date_posted: Option<String>,
    compensation: Option<String>,
    apply_method: Option<String>,
    job_url_direct: Option<String>,
    company_url: Option<String>,
    industry: Option<String>,
    is_remote: Option<bool>,
}

fn read_field(
    object: &serde_json::Map<String, Value>,
    aliases: Option<&BTreeMap<String, Vec<String>>>,
    name: &str,
    defaults: &[&str],
) -> Option<String> {
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
}

fn extra_fields(
    object: &serde_json::Map<String, Value>,
    aliases: Option<&BTreeMap<String, Vec<String>>>,
) -> ExtraFields {
    ExtraFields {
        date_posted: read_field(
            object,
            aliases,
            "date_posted",
            &[
                "date_posted",
                "datePosted",
                "posted_at",
                "postedAt",
                "publication_date",
                "publicationDate",
                "releasedDate",
                "created_at",
                "createdAt",
                "date",
                "PostedDate",
                "published",
            ],
        ),
        compensation: read_field(
            object,
            aliases,
            "compensation",
            &[
                "compensation",
                "salary",
                "salary_range",
                "salaryRange",
                "pay",
            ],
        ),
        apply_method: read_field(
            object,
            aliases,
            "apply_method",
            &["apply_method", "applyMethod", "how_to_apply", "howToApply"],
        ),
        job_url_direct: read_field(
            object,
            aliases,
            "job_url_direct",
            &[
                "job_url_direct",
                "apply_url",
                "applyUrl",
                "application_url",
                "url_active_page",
            ],
        ),
        company_url: read_field(
            object,
            aliases,
            "company_url",
            &[
                "company_url",
                "companyUrl",
                "company.website",
                "company.url",
            ],
        ),
        industry: read_field(
            object,
            aliases,
            "industry",
            &["industry", "company.industry", "category"],
        ),
        is_remote: bool_field(object, aliases, &["is_remote", "isRemote", "remote"]),
    }
}

fn bool_field(
    object: &serde_json::Map<String, Value>,
    aliases: Option<&BTreeMap<String, Vec<String>>>,
    defaults: &[&str],
) -> Option<bool> {
    let value = aliases
        .and_then(|map| map.get("is_remote"))
        .and_then(|names| names.iter().find_map(|name| nested(object, name)))
        .or_else(|| defaults.iter().find_map(|key| object.get(*key)))?;
    match value {
        Value::Bool(value) => Some(*value),
        Value::String(value) => match value.trim().to_ascii_lowercase().as_str() {
            "true" | "yes" | "1" | "remote" => Some(true),
            "false" | "no" | "0" | "onsite" => Some(false),
            _ => None,
        },
        _ => None,
    }
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
    resolve_refs: bool,
    item_type: Option<String>,
    url_from: Option<String>,
    url_prefix: Option<String>,
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
        Self::with_refs(element_id, variable, path, aliases, false)
    }

    /// Build a parser that resolves normalized object references.
    ///
    /// # Errors
    /// Returns a diagnostic when neither an element id nor a variable name
    /// is supplied or when an alias name is empty.
    pub fn with_refs(
        element_id: Option<String>,
        variable: Option<String>,
        path: Vec<String>,
        aliases: Option<BTreeMap<String, Vec<String>>>,
        resolve_refs: bool,
    ) -> Result<Self, String> {
        Self::with_options(
            element_id,
            variable,
            path,
            aliases,
            resolve_refs,
            None,
            None,
            None,
        )
    }

    /// Build a parser for normalized embedded objects with derived URLs.
    ///
    /// # Errors
    /// Returns a diagnostic when aliases or URL settings are invalid.
    #[allow(clippy::too_many_arguments, reason = "manifest options map directly")]
    pub fn with_options(
        element_id: Option<String>,
        variable: Option<String>,
        path: Vec<String>,
        aliases: Option<BTreeMap<String, Vec<String>>>,
        resolve_refs: bool,
        item_type: Option<String>,
        url_from: Option<String>,
        url_prefix: Option<String>,
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
        if url_from.is_some() && url_prefix.is_none() {
            return Err("derived URL needs a prefix".to_owned());
        }
        Ok(Self {
            element_id,
            variable,
            path,
            aliases,
            resolve_refs,
            item_type,
            url_from,
            url_prefix,
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
        let items: Vec<&Value> = if let Some(item_type) = &self.item_type {
            selected
                .as_object()
                .ok_or("embedded JSON envelope is not an object")?
                .values()
                .filter(|item| {
                    item.get("__typename")
                        .and_then(Value::as_str)
                        .is_some_and(|name| name == item_type)
                })
                .collect()
        } else {
            selected
                .as_array()
                .ok_or("embedded JSON envelope is not an array")?
                .iter()
                .collect()
        };
        let mut records = Vec::with_capacity(items.len());
        for item in &items {
            let resolved = if self.resolve_refs {
                resolve_refs(item, &value)
            } else {
                (*item).clone()
            };
            if let Some(record) = record_from_aliases_with_url(
                &resolved,
                self.aliases.as_ref(),
                None,
                self.url_from.as_deref(),
                self.url_prefix.as_deref(),
            ) {
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

fn resolve_refs(item: &Value, root: &Value) -> Value {
    match item {
        Value::Object(map) if map.len() == 1 && map.contains_key("__ref") => {
            let key = map["__ref"].as_str().unwrap_or_default();
            let escaped = key.replace('~', "~0").replace('/', "~1");
            root.pointer(&format!("/props/pageProps/apolloState/data/{escaped}"))
                .map_or_else(|| item.clone(), |target| resolve_refs(target, root))
        }
        Value::Object(map) => {
            let mut resolved = serde_json::Map::with_capacity(map.len());
            for (key, value) in map {
                resolved.insert(key.clone(), resolve_refs(value, root));
            }
            Value::Object(resolved)
        }
        Value::Array(items) => {
            Value::Array(items.iter().map(|item| resolve_refs(item, root)).collect())
        }
        _ => item.clone(),
    }
}

/// Parser for a configured JSON envelope path with field aliases.
#[derive(Debug, Clone, Default)]
pub struct ConfigurableJsonParser {
    path: Vec<String>,
    aliases: Option<BTreeMap<String, Vec<String>>>,
    default_company: Option<String>,
    url_from: Option<String>,
    url_prefix: Option<String>,
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
        Self::with_default_company(path, aliases, None)
    }

    /// Build a parser with a company fallback taken from the envelope.
    ///
    /// # Errors
    /// Returns a diagnostic when an alias name is empty.
    pub fn with_default_company(
        path: &[String],
        aliases: Option<BTreeMap<String, Vec<String>>>,
        default_company: Option<String>,
    ) -> Result<Self, String> {
        Self::with_options(path, aliases, default_company, None, None)
    }

    /// Build a parser with a derived URL field and company fallback.
    ///
    /// # Errors
    /// Returns a diagnostic when an alias or template is invalid.
    pub fn with_options(
        path: &[String],
        aliases: Option<BTreeMap<String, Vec<String>>>,
        default_company: Option<String>,
        url_from: Option<String>,
        url_prefix: Option<String>,
    ) -> Result<Self, String> {
        if let Some(map) = &aliases {
            for (field, names) in map {
                if field.trim().is_empty() || names.iter().any(|name| name.trim().is_empty()) {
                    return Err("field aliases must have non-empty names".to_owned());
                }
            }
        }
        if url_from.is_some() && url_prefix.is_none() {
            return Err("derived URL needs a prefix".to_owned());
        }
        Ok(Self {
            path: path.to_vec(),
            aliases,
            default_company,
            url_from,
            url_prefix,
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
            if let Some(record) = record_from_aliases_with_url(
                item,
                self.aliases.as_ref(),
                self.default_company.as_deref(),
                self.url_from.as_deref(),
                self.url_prefix.as_deref(),
            ) {
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
    fn keeps_posting_date_salary_and_apply_fields() {
        let body = r#"[{"title":"Engineer","company":"Acme","url":"https://example.test/1","date_posted":"2026-11-20","salary":"USD 120k","apply_url":"https://example.test/apply","industry":"Software","remote":true}]"#;
        let records = JsonParser.parse(&response(body)).expect("records parsed");
        assert_eq!(records[0].date_posted.as_deref(), Some("2026-11-20"));
        assert_eq!(records[0].compensation.as_deref(), Some("USD 120k"));
        assert_eq!(
            records[0].job_url_direct.as_deref(),
            Some("https://example.test/apply")
        );
        assert_eq!(records[0].industry.as_deref(), Some("Software"));
        assert_eq!(records[0].is_remote, Some(true));
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

    #[test]
    fn resolves_embedded_object_references() {
        let body = r#"<html><script id="__NEXT_DATA__" type="application/json">{"props":{"pageProps":{"apolloState":{"data":{"JobListing:1":{"title":"Engineer","company":{"__ref":"Startup:2"},"locationNames":["Berlin"],"url":"https://example.test/1"},"Startup:2":{"name":"Labs"}}},"jobs":[{"__ref":"JobListing:1"}]}}}</script></html>"#;
        let parser = EmbeddedJsonParser::with_refs(
            Some("__NEXT_DATA__".to_owned()),
            None,
            vec![
                "props".to_owned(),
                "pageProps".to_owned(),
                "jobs".to_owned(),
            ],
            Some(
                [("company".to_owned(), vec!["company.name".to_owned()])]
                    .into_iter()
                    .collect(),
            ),
            true,
        )
        .expect("parser built");
        let records = parser.parse(&response(body)).expect("records parsed");
        assert_eq!(records[0].company, "Labs");
    }

    #[test]
    fn selects_typed_embedded_objects_and_derives_urls() {
        let body = r#"<html><script id="__NEXT_DATA__" type="application/json">{"props":{"pageProps":{"apolloState":{"data":{"JobListing:1":{"__typename":"JobListing","title":"Engineer","slug":"engineer-1","company":{"__ref":"Startup:2"},"locationNames":["Berlin"]},"Startup:2":{"__typename":"Startup","name":"Labs"},"Other:3":{"__typename":"Other","title":"Ignored","slug":"ignored"}}}}}}</script></html>"#;
        let parser = EmbeddedJsonParser::with_options(
            Some("__NEXT_DATA__".to_owned()),
            None,
            vec![
                "props".to_owned(),
                "pageProps".to_owned(),
                "apolloState".to_owned(),
                "data".to_owned(),
            ],
            Some(
                [
                    ("company".to_owned(), vec!["company.name".to_owned()]),
                    ("location".to_owned(), vec!["locationNames.0".to_owned()]),
                ]
                .into_iter()
                .collect(),
            ),
            true,
            Some("JobListing".to_owned()),
            Some("slug".to_owned()),
            Some("https://example.test/jobs/".to_owned()),
        )
        .expect("parser built");
        let records = parser.parse(&response(body)).expect("records parsed");
        assert_eq!(records.len(), 1);
        assert_eq!(records[0].url, "https://example.test/jobs/engineer-1");
        assert_eq!(records[0].company, "Labs");
        assert_eq!(records[0].location.as_deref(), Some("Berlin"));
    }
}
