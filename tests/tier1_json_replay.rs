//! Offline replays for the seven Tier 1 JSON sources.
//!
//! Fixtures stay synthetic. Each request is rendered from its template, and
//! the response body is parsed through the real manifest adapter. Hacker News
//! replays its two-stage id and item flow from frozen payloads.
//! See documentation/014-testing.md.

// Rust guideline compliant 2026-10-09

use std::collections::BTreeMap;
use std::fs;
use std::path::{Path, PathBuf};

use scrappy::fetching::Response;
use scrappy::parsing::Record;
use scrappy::sources::adapter::{
    AdapterError, Substitution, TemplateValues, build_request, parse_response, request_with,
};
use scrappy::sources::manifest::{Manifest, parse};

const SOURCES: [&str; 6] = [
    "remoteok",
    "arbeitnow",
    "remotive",
    "workingnomads",
    "himalayas",
    "jobicy",
];

const HACKERNEWS: &str = "hackernews";

const FIXTURE_FILES: [&str; 6] = [
    "request.json",
    "request.template.json",
    "response.body",
    "headers.json",
    "frozen.json",
    "golden.ndjson",
];

fn root() -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR"))
}

fn fixture_dir(id: &str) -> PathBuf {
    root().join("tests/fixtures").join(id)
}

fn read_json(path: &Path) -> serde_json::Value {
    let text = fs::read_to_string(path)
        .unwrap_or_else(|error| panic!("cannot read {}: {error}", path.display()));
    serde_json::from_str(&text)
        .unwrap_or_else(|error| panic!("cannot parse {}: {error}", path.display()))
}

fn manifest(id: &str) -> Manifest {
    let path = root()
        .join("src/sources/manifests/feeds_api")
        .join(format!("{id}.toml"));
    let text = fs::read_to_string(&path)
        .unwrap_or_else(|error| panic!("cannot read {}: {error}", path.display()));
    parse(&text).unwrap_or_else(|error| panic!("invalid manifest {}: {error}", path.display()))
}

fn render(template: &str, values: &serde_json::Map<String, serde_json::Value>) -> String {
    let mut rendered = template.to_owned();
    for (name, value) in values {
        let value = value.as_str().expect("substitution is a string");
        let token = format!("{{{name}}}");
        assert!(rendered.contains(&token), "unused substitution {name}");
        rendered = rendered.replace(&token, value);
    }
    assert!(
        !rendered.contains('{') && !rendered.contains('}'),
        "unresolved placeholder in {rendered}"
    );
    rendered
}

fn substitutions(dir: &Path) -> Vec<Substitution> {
    let template = read_json(&dir.join("request.template.json"));
    let frozen = read_json(&dir.join("frozen.json"));
    let values = frozen
        .get("substitutions")
        .and_then(serde_json::Value::as_object)
        .expect("frozen substitutions are an object");
    let templated_url = template
        .get("url")
        .and_then(serde_json::Value::as_str)
        .expect("template has a url");
    render(templated_url, values);
    values
        .iter()
        .map(|(name, value)| Substitution::Query {
            name: name.clone(),
            value: value.as_str().expect("substitution is a string").to_owned(),
        })
        .collect()
}

fn request(dir: &Path) -> String {
    let template = read_json(&dir.join("request.template.json"));
    let frozen = read_json(&dir.join("frozen.json"));
    let expected = read_json(&dir.join("request.json"));
    let values = frozen
        .get("substitutions")
        .and_then(serde_json::Value::as_object)
        .expect("frozen substitutions are an object");
    let templated_url = template
        .get("url")
        .and_then(serde_json::Value::as_str)
        .expect("template has a url");
    let url = render(templated_url, values);
    assert_eq!(expected["url"], url, "frozen URL differs from template");
    assert_eq!(expected["method"], template["method"], "method mismatch");
    assert_eq!(
        expected["headers"], template["headers"],
        "request headers differ from template"
    );
    url
}

fn response(url: &str, dir: &Path) -> Response {
    let headers = read_json(&dir.join("headers.json"));
    let status = u16::try_from(
        headers
            .get("status")
            .and_then(serde_json::Value::as_u64)
            .expect("headers have a status"),
    )
    .expect("status fits in u16");
    let text_headers: BTreeMap<String, String> = headers
        .as_object()
        .expect("headers are an object")
        .iter()
        .filter(|(name, _)| *name != "status")
        .map(|(name, value)| {
            (
                name.clone(),
                value.as_str().expect("header value is a string").to_owned(),
            )
        })
        .collect();
    let body = fs::read(dir.join("response.body")).expect("response body is readable");
    Response::new(url, status, text_headers, body).expect("bounded response")
}

fn expected_records(dir: &Path) -> Vec<Record> {
    let text = fs::read_to_string(dir.join("golden.ndjson")).expect("golden file is readable");
    text.lines()
        .filter(|line| !line.trim().is_empty())
        .map(|line| {
            let value: serde_json::Value = serde_json::from_str(line).expect("golden line is JSON");
            Record::new(
                value["url"].as_str().expect("golden url"),
                value["title"].as_str().expect("golden title"),
                value["company"].as_str().expect("golden company"),
            )
            .expect("valid golden record")
            .with_location(value["location"].as_str().map(str::to_owned))
            .with_description(value["description"].as_str().map(str::to_owned))
        })
        .collect()
}

fn assert_records(actual: &[Record], expected: &[Record]) {
    assert_eq!(actual.len(), expected.len(), "record count mismatch");
    for (index, (left, right)) in actual.iter().zip(expected).enumerate() {
        assert_eq!(left, right, "record {index} differs from golden");
    }
}

fn replay(id: &str) -> Result<Vec<Record>, AdapterError> {
    let dir = fixture_dir(id);
    for name in FIXTURE_FILES {
        assert!(dir.join(name).is_file(), "{id} is missing {name}");
    }
    let manifest = manifest(id);
    let url = request(&dir);
    let substitutions = substitutions(&dir);
    let built = request_with(&manifest, &substitutions).expect("request built from substitutions");
    assert_eq!(built.url, url, "adapter URL differs from frozen request");
    let response = response(&url, &dir);
    parse_response(&manifest, &response)
}

#[derive(Debug, Clone, Copy)]
struct TemplateInput<'a> {
    name: &'a str,
    value: &'a str,
}

fn render_query_template(template: &str, inputs: &[TemplateInput<'_>]) -> String {
    let mut rendered = template.to_owned();
    for input in inputs {
        let token = format!("{{query:{}}}", input.name);
        assert!(rendered.contains(&token), "unused query {}", input.name);
        rendered = rendered.replace(&token, input.value);
    }
    assert!(
        !rendered.contains('{') && !rendered.contains('}'),
        "unresolved placeholder in {rendered}"
    );
    rendered
}

fn replay_fanout() -> (Vec<Record>, usize) {
    let dir = fixture_dir(HACKERNEWS);
    for name in FIXTURE_FILES {
        assert!(dir.join(name).is_file(), "{HACKERNEWS} is missing {name}");
    }
    let manifest = manifest(HACKERNEWS);
    let base_url = substitutions(&dir)
        .into_iter()
        .find_map(|item| match item {
            Substitution::Query { name, value } if name == "base_url" => Some(value),
            _ => None,
        })
        .expect("fanout base URL");
    let frozen = read_json(&dir.join("frozen.json"));
    let replay = frozen
        .get("replay")
        .and_then(serde_json::Value::as_object)
        .expect("frozen replay items are an object");
    let items = frozen
        .get("items")
        .and_then(serde_json::Value::as_array)
        .expect("frozen items are an array");
    let id_payload = read_json(&dir.join("response.body"));
    let ids = id_payload.as_array().expect("story ids are an array");
    assert_eq!(ids.len(), items.len(), "id array and frozen items differ");
    assert_eq!(ids.len(), replay.len(), "id array and replay items differ");

    let batch_size = 10;
    let max_ids = 45;
    let selected = ids.iter().take(max_ids).collect::<Vec<_>>();
    let batches = selected.chunks(batch_size).count();
    let item_template = format!("{base_url}/item/{{query:id}}.json");
    let mut records = Vec::new();
    for (index, ((id, raw), item)) in ids.iter().zip(replay.values()).zip(items).enumerate() {
        let id_text = id.to_string();
        let item_url = render_query_template(
            &item_template,
            &[TemplateInput {
                name: "id",
                value: &id_text,
            }],
        );
        let built = build_request(
            &manifest,
            &TemplateValues {
                query: BTreeMap::from([("id".to_owned(), id_text.clone())]),
                ..TemplateValues::default()
            },
        )
        .expect("item request built");
        assert_eq!(built.method, "GET");
        assert_eq!(
            built.headers.get("accept").map(String::as_str),
            Some("application/json")
        );
        assert_eq!(
            id.as_u64(),
            raw["id"].as_u64(),
            "raw item {index} belongs to a different id"
        );
        assert_eq!(
            id.as_u64(),
            item["id"].as_u64(),
            "derived item {index} belongs to a different id"
        );
        let raw_url = raw["url"].as_str();
        assert_eq!(
            raw_url,
            item["url"].as_str(),
            "derived item {index} has a different URL"
        );
        if raw_url.is_none() {
            continue;
        }
        let mut derived = item.clone();
        if derived.get("company").is_none() {
            let title = item["title"].as_str().expect("item has a hiring title");
            let company = match title.split_once(" is hiring ") {
                Some((company, _)) => company,
                None => title
                    .split_once(" - ")
                    .map(|(company, _)| company)
                    .expect("hiring title carries a company name"),
            };
            derived["company"] = serde_json::Value::String(company.to_owned());
        }
        let body = serde_json::to_vec(&[&derived]).expect("item serialized");
        let response =
            Response::new(item_url, 200, BTreeMap::new(), body).expect("bounded response");
        let parsed = parse_response(&manifest, &response).expect("item parsed");
        assert_eq!(parsed.len(), 1, "one record per item object");
        records.extend(parsed);
    }
    (records, batches)
}

#[test]
fn fixtures_normalize_through_the_adapter() {
    for id in SOURCES {
        let expected = expected_records(&fixture_dir(id));
        let actual = replay(id).unwrap_or_else(|error| panic!("{id} replay failed: {error}"));
        assert_records(&actual, &expected);
    }
    let dir = fixture_dir(HACKERNEWS);
    let (records, _) = replay_fanout();
    assert_records(&records, &expected_records(&dir));
}

#[test]
fn templated_requests_build_without_credentials() {
    for id in SOURCES {
        let manifest = manifest(id);
        let url = request(&fixture_dir(id));
        let substitutions = substitutions(&fixture_dir(id));
        let values = TemplateValues {
            query: substitutions
                .iter()
                .filter_map(|item| match item {
                    Substitution::Query { name, value } => Some((name.clone(), value.clone())),
                    _ => None,
                })
                .collect(),
            ..TemplateValues::default()
        };
        let built = build_request(&manifest, &values)
            .unwrap_or_else(|error| panic!("{id} request failed: {error}"));
        assert_eq!(built.url, url, "{id} request URL mismatch");
        assert_eq!(built.method, "GET");
        assert_eq!(
            built.headers.get("accept").map(String::as_str),
            Some("application/json")
        );
        assert!(manifest.credentials.is_empty(), "{id} declared credentials");
    }
}

#[test]
fn metadata_is_skipped_and_html_text_is_retained() {
    let remoteok = replay("remoteok").expect("remoteok replay");
    assert_eq!(remoteok.len(), 2, "metadata element must not normalize");
    assert!(
        remoteok
            .iter()
            .all(|record| !record.title.is_empty() && !record.company.is_empty())
    );
    let arbeitnow = replay("arbeitnow").expect("arbeitnow replay");
    assert!(
        arbeitnow[0]
            .description
            .as_deref()
            .is_some_and(|text| text.contains("<strong>")),
        "HTML description must survive normalization"
    );
}

#[test]
fn aliases_and_envelopes_map_feed_fields() {
    let jobicy = replay("jobicy").expect("jobicy replay");
    assert_eq!(jobicy[0].title, "Rust Engineer");
    assert_eq!(jobicy[0].company, "Example Co");
    let himalayas = replay("himalayas").expect("himalayas replay");
    assert_eq!(himalayas[0].company, "Example Co");
    assert_eq!(himalayas[0].location.as_deref(), Some("Worldwide"));
    let workingnomads = replay("workingnomads").expect("workingnomads replay");
    assert_eq!(workingnomads[1].company, "Sample Labs");
    let remotive = replay("remotive").expect("remotive replay");
    assert_eq!(remotive[0].location.as_deref(), Some("Worldwide"));
}

#[test]
fn hackernews_replays_the_two_stage_fanout() {
    let (records, batches) = replay_fanout();
    assert_eq!(batches, 1, "four ids fit in one batch of ten");
    assert_eq!(records.len(), 3, "item without a URL is skipped");
    assert_records(&records, &expected_records(&fixture_dir(HACKERNEWS)));
}
