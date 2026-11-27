// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Arin Balyan
// Last modified: 2026-11-27

//! Offline golden replay for search and tenant API sources.
//!
//! Frozen responses are parsed through the real manifest adapters. No
//! network access or credentials are involved.

// Rust guideline compliant 2026-10-09

use std::collections::BTreeMap;
use std::fs;
use std::path::{Path, PathBuf};

use scrappy::fetching::Response;
use scrappy::sources::adapter::{TemplateValues, build_request, parse_response};
use scrappy::sources::manifest::{Family, Health, parse};
use serde_json::Value;

const FIXTURE_FILES: [&str; 6] = [
    "request.json",
    "request.template.json",
    "response.body",
    "headers.json",
    "frozen.json",
    "golden.ndjson",
];

struct Case {
    id: &'static str,
    family: Family,
    health: Health,
    dir: &'static str,
}

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

fn replay(case: &Case) {
    let manifest_path = root()
        .join("src/sources/manifests")
        .join(case.dir)
        .join(format!("{}.toml", case.id));
    let manifest = parse(&read_text(&manifest_path)).expect("valid manifest");
    assert_eq!(manifest.id, case.id);
    assert_eq!(manifest.family, case.family);
    assert_eq!(manifest.health, case.health);
    assert_eq!(manifest.fixtures, vec![case.id.to_owned()]);

    let dir = root().join("tests/fixtures").join(case.id);
    for name in FIXTURE_FILES {
        assert!(dir.join(name).is_file(), "missing {name}");
    }
    let request_json = read_json(&dir.join("request.json"));
    let template_json = read_json(&dir.join("request.template.json"));
    assert_eq!(request_json["method"], template_json["method"]);

    let frozen = read_json(&dir.join("frozen.json"));
    let mut values = TemplateValues::default();
    let substitutions = frozen["substitutions"]
        .as_object()
        .expect("frozen substitutions are an object");
    if let Some(slug) = substitutions.get("slug").and_then(Value::as_str) {
        values.slug = Some(slug.to_owned());
    }
    for (name, value) in substitutions {
        if name == "slug" {
            continue;
        }
        values.query.insert(
            name.clone(),
            value.as_str().expect("substitution is a string").to_owned(),
        );
    }

    let built = build_request(&manifest, &values).expect("valid request");
    let url = request_json["url"].as_str().expect("request has a url");
    assert_eq!(built.url, url);
    for (name, value) in request_json["headers"]
        .as_object()
        .expect("request headers are an object")
    {
        assert_eq!(
            built.headers.get(name).map(String::as_str),
            Some(value.as_str().expect("header value is a string"))
        );
    }

    let response = response(url, &dir);
    let actual: Vec<Value> = parse_response(&manifest, &response)
        .expect("replay succeeds")
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
    assert_eq!(actual, expected);
}

#[test]
fn search_and_tenant_sources_replay_into_golden_records() {
    replay(&Case {
        id: "careeronestop",
        family: Family::FeedsApi,
        health: Health::Working,
        dir: "feeds_api",
    });
    replay(&Case {
        id: "headhunter",
        family: Family::FeedsApi,
        health: Health::Blocked,
        dir: "feeds_api",
    });
    replay(&Case {
        id: "loxo",
        family: Family::Ats,
        health: Health::Blocked,
        dir: "ats",
    });
}
