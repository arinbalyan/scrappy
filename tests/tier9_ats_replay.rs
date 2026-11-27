// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Arin Balyan
// Last modified: 2026-11-27

//! Offline golden replays for seed-based ATS API sources.
//!
//! Frozen responses are parsed through the real manifest adapter. Slug values
//! come from fixtures and are never read from the environment.

// Rust guideline compliant 2026-10-09

use std::collections::BTreeMap;
use std::fs;
use std::path::{Path, PathBuf};

use scrappy::config::secret::credential;
use scrappy::fetching::Response;
use scrappy::sources::adapter::{TemplateValues, build_request, parse_response};
use scrappy::sources::manifest::{Family, Health, Manifest, parse};
use serde_json::Value;

const SOURCES: [(&str, Family, Health); 5] = [
    ("bamboohr", Family::Ats, Health::Working),
    ("breezyhr", Family::Ats, Health::Working),
    ("comeet", Family::Keyed, Health::Keyed),
    ("jobscore", Family::Ats, Health::Working),
    ("manatal", Family::Ats, Health::Stale),
];

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

fn fixture_dir(id: &str) -> PathBuf {
    root().join("tests/fixtures").join(id)
}

fn manifest(id: &str, family_dir: &str) -> Manifest {
    let path = root()
        .join("src/sources/manifests")
        .join(family_dir)
        .join(format!("{id}.toml"));
    parse(&read_text(&path)).unwrap_or_else(|error| panic!("{id}: {error}"))
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
fn seed_based_sources_replay_into_golden_records() {
    for (id, family, health) in SOURCES {
        let family_dir = match family {
            Family::Keyed => "keyed",
            _ => "ats",
        };
        let manifest = manifest(id, family_dir);
        assert_eq!(manifest.id, id);
        assert_eq!(manifest.family, family);
        assert_eq!(manifest.health, health);
        assert_eq!(manifest.fixtures, vec![id.to_owned()]);

        let dir = fixture_dir(id);
        for name in FIXTURE_FILES {
            assert!(dir.join(name).is_file(), "{id} is missing {name}");
        }

        let request_json = read_json(&dir.join("request.json"));
        let template_json = read_json(&dir.join("request.template.json"));
        assert_eq!(request_json["method"], template_json["method"]);
        let frozen = read_json(&dir.join("frozen.json"));
        let slug = frozen
            .get("substitutions")
            .and_then(|value| value.get("slug"))
            .and_then(Value::as_str)
            .expect("frozen request has a slug");
        let mut values = TemplateValues {
            slug: Some(slug.to_owned()),
            ..Default::default()
        };
        for (name, value) in frozen
            .get("substitutions")
            .and_then(Value::as_object)
            .expect("frozen substitutions are an object")
        {
            if name != "slug" {
                values.credentials.insert(
                    name.to_ascii_uppercase(),
                    credential(value.as_str().expect("credential value is a string")),
                );
            }
        }
        let built = build_request(&manifest, &values)
            .unwrap_or_else(|error| panic!("{id} build failed: {error}"));
        let url = request_json["url"].as_str().expect("request has a url");
        assert_eq!(built.url, url, "{id} adapter URL differs");
        assert_eq!(
            built.method,
            request_json["method"].as_str().unwrap_or("GET")
        );
        for (name, value) in request_json["headers"]
            .as_object()
            .expect("request headers are an object")
        {
            assert_eq!(
                built.headers.get(name).map(String::as_str),
                Some(value.as_str().expect("header value is a string")),
                "{id} header {name} differs"
            );
        }

        let response = response(url, &dir);
        let actual: Vec<Value> = parse_response(&manifest, &response)
            .unwrap_or_else(|error| panic!("{id} replay failed: {error}"))
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
        assert_eq!(actual, expected, "{id} golden mismatch");
    }
}
