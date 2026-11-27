// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Arin Balyan
// Last modified: 2026-11-27

//! Offline checks for the six-file fixture contract.
//!
//! Each source directory under `tests/fixtures` stores a frozen request,
//! its template, the response body and headers, substitution values, and the
//! expected normalized output. The remoteok check renders the template with
//! frozen substitutions and compares it to the frozen request.

// Rust guideline compliant 2026-10-09

use std::fs;
use std::path::{Path, PathBuf};

use serde_json::{Map, Value};

const REQUIRED_FILES: [&str; 6] = [
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

fn fixture_sets() -> Vec<PathBuf> {
    let fixture_root = root().join("tests/fixtures");
    let mut sets: Vec<PathBuf> = fs::read_dir(fixture_root)
        .expect("fixtures directory exists")
        .filter_map(Result::ok)
        .map(|entry| entry.path())
        .filter(|path| path.is_dir())
        .collect();
    sets.sort();
    sets
}

fn read_json(path: &Path) -> Value {
    let text = fs::read_to_string(path)
        .unwrap_or_else(|error| panic!("cannot read {}: {error}", path.display()));
    serde_json::from_str(&text)
        .unwrap_or_else(|error| panic!("cannot parse {}: {error}", path.display()))
}

fn substitutions(frozen: &Value) -> &Map<String, Value> {
    frozen
        .get("substitutions")
        .and_then(Value::as_object)
        .expect("frozen substitutions are an object")
}

fn render(template: &str, values: &Map<String, Value>) -> String {
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

#[test]
fn every_fixture_set_has_the_six_contract_files() {
    let sets = fixture_sets();
    assert!(!sets.is_empty(), "no fixture sets found");
    for set in sets {
        for name in REQUIRED_FILES {
            let path = set.join(name);
            assert!(path.is_file(), "missing {}", path.display());
        }
    }
}

#[test]
fn remoteok_request_matches_template_and_frozen_substitutions() {
    let dir = root().join("tests/fixtures/remoteok");
    let template = read_json(&dir.join("request.template.json"));
    let frozen = read_json(&dir.join("frozen.json"));
    let request = read_json(&dir.join("request.json"));

    assert!(
        template
            .get("method")
            .and_then(Value::as_str)
            .is_some_and(|method| !method.is_empty()),
        "template has no method"
    );
    assert!(
        template
            .get("headers")
            .and_then(Value::as_object)
            .is_some_and(|headers| !headers.is_empty()),
        "template has no headers"
    );
    let template_url = template
        .get("url")
        .and_then(Value::as_str)
        .expect("template has a url");
    assert!(
        template_url.contains('{'),
        "template url has no placeholder"
    );
    assert!(
        frozen
            .get("fetched_at")
            .and_then(Value::as_str)
            .is_some_and(|timestamp| !timestamp.is_empty()),
        "frozen set has no fetched_at"
    );

    let mut expected = template.clone();
    expected["url"] = Value::String(render(template_url, substitutions(&frozen)));
    assert_eq!(
        request, expected,
        "frozen request differs from rendered template"
    );
}
