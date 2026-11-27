//! Offline golden replay for the newly ported unregistered source.
//!
//! The frozen response is parsed through the real manifest adapter. The slug
//! substitution is checked before parsing. No network access is involved.

// Rust guideline compliant 2026-11-27

use std::collections::BTreeMap;
use std::fs;
use std::path::{Path, PathBuf};

use scrappy::fetching::Response;
use scrappy::sources::adapter::{TemplateValues, build_request, parse_response};
use scrappy::sources::manifest::{Family, Health, parse};
use serde_json::Value;

const SOURCE_ID: &str = "ats-gem";
const FAMILY_DIR: &str = "ats";

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

fn read_text(path: &Path) -> String {
    fs::read_to_string(path)
        .unwrap_or_else(|error| panic!("cannot read {}: {error}", path.display()))
}

fn read_json(path: &Path) -> Value {
    let text = read_text(path);
    serde_json::from_str(&text)
        .unwrap_or_else(|error| panic!("cannot parse {}: {error}", path.display()))
}

fn manifest() -> scrappy::sources::manifest::Manifest {
    let path = root()
        .join("src/sources/manifests")
        .join(FAMILY_DIR)
        .join(format!("{SOURCE_ID}.toml"));
    parse(&read_text(&path)).unwrap_or_else(|error| panic!("{SOURCE_ID}: {error}"))
}

fn values(dir: &Path) -> TemplateValues {
    let frozen = read_json(&dir.join("frozen.json"));
    let slug = frozen
        .get("substitutions")
        .and_then(Value::as_object)
        .and_then(|object| object.get("slug"))
        .and_then(Value::as_str)
        .expect("frozen substitutions have a slug")
        .to_owned();
    TemplateValues {
        slug: Some(slug),
        ..TemplateValues::default()
    }
}

fn response(url: &str, dir: &Path) -> Response {
    let headers = read_json(&dir.join("headers.json"));
    let status = u16::try_from(
        headers
            .get("status")
            .and_then(Value::as_u64)
            .expect("headers have a status"),
    )
    .expect("status fits in u16");
    let table: BTreeMap<String, String> = headers
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
    Response::new(url, status, table, body).expect("bounded response")
}

fn canonical(value: &Value) -> Value {
    let mut object = value.as_object().expect("record is an object").clone();
    for key in ["location", "description"] {
        if object.get(key).is_some_and(Value::is_null) {
            object.remove(key);
        }
    }
    Value::Object(object)
}

#[test]
fn ats_gem_replays_into_golden_records() {
    let manifest = manifest();
    assert_eq!(manifest.id, SOURCE_ID);
    assert_eq!(manifest.family, Family::Ats);
    assert_eq!(manifest.health, Health::RequiresSlug);
    assert_eq!(manifest.fixtures, vec![SOURCE_ID.to_owned()]);

    let dir = root().join("tests/fixtures").join(SOURCE_ID);
    for name in FIXTURE_FILES {
        assert!(dir.join(name).is_file(), "{SOURCE_ID} is missing {name}");
    }
    let request = read_json(&dir.join("request.json"));
    let url = request["url"].as_str().expect("request has a url");
    let built = build_request(&manifest, &values(&dir))
        .unwrap_or_else(|error| panic!("{SOURCE_ID} build failed: {error}"));
    assert_eq!(built.url, url, "{SOURCE_ID} adapter URL differs");
    for (name, value) in request["headers"]
        .as_object()
        .expect("request headers are an object")
    {
        assert_eq!(
            built.headers.get(name).map(String::as_str),
            Some(value.as_str().expect("header value is a string")),
            "{SOURCE_ID} header {name} differs"
        );
    }

    let response = response(url, &dir);
    let actual: Vec<Value> = parse_response(&manifest, &response)
        .unwrap_or_else(|error| panic!("{SOURCE_ID} replay failed: {error}"))
        .into_iter()
        .map(|record| {
            serde_json::json!({
                "source_id": manifest.id,
                "title": record.title,
                "company": record.company,
                "url": record.url,
                "location": record.location,
                "description": record.description,
            })
        })
        .map(|value| canonical(&value))
        .collect();
    let expected: Vec<Value> = read_text(&dir.join("golden.ndjson"))
        .lines()
        .filter(|line| !line.trim().is_empty())
        .map(|line| serde_json::from_str(line).expect("golden line is JSON"))
        .map(|value| canonical(&value))
        .collect();
    assert_eq!(actual, expected, "{SOURCE_ID} golden mismatch");
}
