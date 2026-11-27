// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Arin Balyan
// Last modified: 2026-11-27

//! Offline golden replays for the public API sources.
//!
//! Each frozen response is parsed through the real manifest adapter and
//! compared with its golden records. No network access is used.

// Rust guideline compliant 2026-10-09

use std::collections::BTreeMap;
use std::fs;
use std::path::{Path, PathBuf};

use scrappy::fetching::Response;
use scrappy::sources::adapter::{TemplateValues, build_request, parse_response};
use scrappy::sources::manifest::{Family, Health, Manifest, parse};
use serde_json::Value;

const SOURCES: [(&str, Family, Health, &str); 5] = [
    (
        "canadajobbank",
        Family::FeedsApi,
        Health::Stale,
        "feeds_api",
    ),
    ("coroflot", Family::FeedsApi, Health::Working, "feeds_api"),
    ("joinrise", Family::FeedsApi, Health::Stale, "feeds_api"),
    (
        "startupjobs",
        Family::FeedsApi,
        Health::Blocked,
        "feeds_api",
    ),
    ("web3career", Family::FeedsApi, Health::Keyed, "feeds_api"),
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
fn public_sources_replay_into_golden_records() {
    for (id, family, health, family_dir) in SOURCES {
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
        let url = request_json["url"].as_str().expect("request has a url");
        let built = build_request(&manifest, &TemplateValues::default())
            .unwrap_or_else(|error| panic!("{id} build failed: {error}"));
        assert_eq!(built.url, url, "{id} adapter URL differs");
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
