// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Arin Balyan
// Last modified: 2026-11-27

//! Offline replay of Tier 2 C fixtures through the real adapters.
//!
//! Each frozen response is parsed with `scrappy::sources::adapter::parse_response`
//! and compared to the committed golden records. No network access is used.

// Rust guideline compliant 2026-10-09

use std::collections::BTreeMap;
use std::fs;
use std::path::{Path, PathBuf};

use scrappy::fetching::Response;
use scrappy::sources::adapter::{TemplateValues, build_request, parse_response};
use scrappy::sources::manifest::{Family, Health, Manifest, parse};

const SOURCES: [(&str, Family, Health, &str, &str); 2] = [
    (
        "simplyhired",
        Family::HtmlStatic,
        Health::Working,
        "html_static",
        "html",
    ),
    (
        "ukvisajobs",
        Family::FeedsApi,
        Health::Working,
        "feeds_api",
        "json",
    ),
];

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

fn read_text(path: &Path) -> String {
    fs::read_to_string(path)
        .unwrap_or_else(|error| panic!("cannot read {}: {error}", path.display()))
}

fn read_json(path: &Path) -> serde_json::Value {
    let text = read_text(path);
    serde_json::from_str(&text)
        .unwrap_or_else(|error| panic!("cannot parse {}: {error}", path.display()))
}

fn manifest_for(id: &str, family: &str) -> Manifest {
    let path = root()
        .join("src/sources/manifests")
        .join(family)
        .join(format!("{id}.toml"));
    parse(&read_text(&path)).unwrap_or_else(|error| panic!("{}: {error}", path.display()))
}

fn render_template(template: &str, values: &serde_json::Map<String, serde_json::Value>) -> String {
    let mut rendered = template.to_owned();
    for (name, value) in values {
        let value = value.as_str().expect("substitution is a string");
        let token = format!("{{{name}}}");
        rendered = rendered.replace(&token, value);
        for alias in aliases_for(name) {
            rendered = rendered.replace(&format!("{{{alias}}}"), value);
        }
    }
    assert!(
        !rendered.contains('{') && !rendered.contains('}'),
        "unresolved placeholder in {rendered}"
    );
    rendered
}

fn aliases_for(name: &str) -> Vec<&'static str> {
    match name {
        "q" => vec!["query:q"],
        "location" => vec!["l", "query:l"],
        "page" => vec!["pn", "query:pn"],
        _ => Vec::new(),
    }
}

fn golden_records(path: &Path) -> Vec<serde_json::Value> {
    read_text(path)
        .lines()
        .filter(|line| !line.trim().is_empty())
        .map(|line| {
            serde_json::from_str(line)
                .unwrap_or_else(|error| panic!("cannot parse {}: {error}", path.display()))
        })
        .collect()
}

fn normalized_records(manifest: &Manifest, fixture: &Path) -> Vec<serde_json::Value> {
    let request = read_json(&fixture.join("request.json"));
    let headers = read_json(&fixture.join("headers.json"));
    let body = fs::read(fixture.join("response.body")).expect("response body exists");
    let status = u16::try_from(
        headers
            .get("status")
            .and_then(serde_json::Value::as_u64)
            .expect("headers have a status"),
    )
    .expect("status fits in u16");
    let header_map = headers.as_object().expect("headers object");
    let response_headers: BTreeMap<String, String> = header_map
        .iter()
        .filter(|(name, _)| *name != "status")
        .map(|(name, value)| {
            (
                name.clone(),
                value.as_str().expect("header value is a string").to_owned(),
            )
        })
        .collect();
    let url = request
        .get("url")
        .and_then(serde_json::Value::as_str)
        .expect("request has a url")
        .to_owned();
    let response = Response::new(url, status, response_headers, body).expect("bounded response");
    let records = parse_response(manifest, &response)
        .unwrap_or_else(|error| panic!("{}: {error}", manifest.id));
    records
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
        .collect()
}

fn adapter_values(values: &serde_json::Map<String, serde_json::Value>) -> TemplateValues {
    let mut result = TemplateValues::default();
    for (name, value) in values {
        let name = match name.as_str() {
            "q" => "q",
            "location" => "l",
            "page" => "pn",
            _ => continue,
        };
        let value = value.as_str().expect("substitution is a string").to_owned();
        result.query.insert(name.to_owned(), value);
    }
    result
}

#[test]
fn tier2_c_fixtures_replay_through_adapter() {
    for (id, family, health, family_dir, parser) in SOURCES {
        let manifest = manifest_for(id, family_dir);
        assert_eq!(manifest.id, id);
        assert_eq!(manifest.family, family);
        assert_eq!(manifest.health, health);
        assert_eq!(manifest.parser, parser);
        assert_eq!(manifest.fixtures, vec![id.to_owned()]);

        let fixture = root().join("tests/fixtures").join(id);
        for name in REQUIRED_FILES {
            assert!(fixture.join(name).is_file(), "{id} is missing {name}");
        }

        let request = read_json(&fixture.join("request.json"));
        let template = read_json(&fixture.join("request.template.json"));
        let frozen = read_json(&fixture.join("frozen.json"));
        let substitutions = frozen
            .get("substitutions")
            .and_then(serde_json::Value::as_object)
            .expect("frozen substitutions are an object");
        let template_url = template
            .get("url")
            .and_then(serde_json::Value::as_str)
            .expect("template has a url");
        let mut expected = template.clone();
        expected["url"] = serde_json::Value::String(render_template(template_url, substitutions));
        assert_eq!(
            request, expected,
            "{id} frozen request differs from template"
        );

        let rendered_manifest = render_template(&manifest.request, substitutions);
        let built = build_request(&manifest, &adapter_values(substitutions))
            .unwrap_or_else(|error| panic!("{id} request failed: {}", error.message));
        assert_eq!(built.method, "GET");
        assert_eq!(
            built.url, rendered_manifest,
            "{id} adapter URL differs from manifest template"
        );
        assert_eq!(
            built.url, request["url"],
            "{id} built URL differs from frozen"
        );
        for (name, value) in &manifest
            .request_options
            .as_ref()
            .expect("request options")
            .headers
        {
            assert_eq!(
                built.headers.get(name).map(String::as_str),
                Some(render_template(value, substitutions).as_str()),
                "{id} built header {name} differs from manifest"
            );
        }

        let actual = normalized_records(&manifest, &fixture);
        let expected_records = golden_records(&fixture.join("golden.ndjson"));
        assert_eq!(
            actual, expected_records,
            "{id} replay differs from golden: {actual:?}"
        );
    }
}
