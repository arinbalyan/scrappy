// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Arin Balyan
// Last modified: 2026-11-27

//! Offline replay of the Tier 3 B fixtures through the real adapters.
//!
//! Each frozen response is parsed through `scrappy::sources::adapter` and
//! compared to the committed golden records. No network access is used.

// Rust guideline compliant 2026-10-09

use std::collections::BTreeMap;
use std::fs;
use std::path::{Path, PathBuf};

use serde_json::{Map, Value};

use scrappy::fetching::Response;
use scrappy::sources::adapter::{
    Substitution, TemplateValues, build_request, parse_response, request_with,
};
use scrappy::sources::manifest::{Family, Health, Manifest, parse};

const SOURCES: [(&str, Family, Health, &str, &str); 5] = [
    (
        "ats-rippling",
        Family::HtmlStatic,
        Health::Blocked,
        "html_static",
        "json",
    ),
    (
        "ats-talentlyft",
        Family::Ats,
        Health::RequiresSlug,
        "ats",
        "json",
    ),
    ("adzuna", Family::Keyed, Health::Keyed, "keyed", "json"),
    ("careerjet", Family::Keyed, Health::Keyed, "keyed", "json"),
    ("findwork", Family::Keyed, Health::Keyed, "keyed", "json"),
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

fn read_json(path: &Path) -> Value {
    let text = read_text(path);
    serde_json::from_str(&text)
        .unwrap_or_else(|error| panic!("cannot parse {}: {error}", path.display()))
}

fn manifest_for(id: &str, family_dir: &str) -> Manifest {
    let path = root()
        .join("src/sources/manifests")
        .join(family_dir)
        .join(format!("{id}.toml"));
    parse(&read_text(&path)).unwrap_or_else(|error| panic!("{}: {error}", path.display()))
}

fn render(template: &str, values: &Map<String, Value>) -> String {
    let mut rendered = template.to_owned();
    for (name, value) in values {
        let value = value.as_str().expect("substitution is a string");
        let token = format!("{{{name}}}");
        if rendered.contains(&token) {
            rendered = rendered.replace(&token, value);
        }
    }
    assert!(
        !rendered.contains('{') && !rendered.contains('}'),
        "unresolved placeholder in {rendered}"
    );
    rendered
}

fn golden_records(path: &Path) -> Vec<Value> {
    read_text(path)
        .lines()
        .filter(|line| !line.trim().is_empty())
        .map(|line| {
            serde_json::from_str(line)
                .unwrap_or_else(|error| panic!("cannot parse {}: {error}", path.display()))
        })
        .collect()
}

fn substitutions(manifest: &Manifest, frozen: &Value) -> Vec<Substitution> {
    let values = frozen
        .get("substitutions")
        .and_then(Value::as_object)
        .expect("frozen substitutions are an object");
    values
        .iter()
        .map(|(name, value)| {
            let value = value.as_str().expect("substitution is a string").to_owned();
            if name == "slug" {
                Substitution::Slug(value)
            } else if manifest.credentials.contains(name) {
                Substitution::Key {
                    name: name.clone(),
                    value: scrappy::config::secret::credential(value),
                }
            } else {
                Substitution::Query {
                    name: name.clone(),
                    value,
                }
            }
        })
        .collect()
}

fn request_fields(request: &Value) -> (String, String, BTreeMap<String, String>) {
    let url = request
        .get("url")
        .and_then(Value::as_str)
        .expect("request has a url")
        .to_owned();
    let method = request
        .get("method")
        .and_then(Value::as_str)
        .expect("request has a method")
        .to_owned();
    let headers = request
        .get("headers")
        .and_then(Value::as_object)
        .expect("request has headers")
        .iter()
        .map(|(name, value)| {
            (
                name.clone(),
                value.as_str().expect("header value is a string").to_owned(),
            )
        })
        .collect();
    (url, method, headers)
}

fn normalized_records(manifest: &Manifest, fixture: &Path, url: &str) -> Vec<Value> {
    let headers = read_json(&fixture.join("headers.json"));
    let status = u16::try_from(
        headers
            .get("status")
            .and_then(Value::as_u64)
            .expect("headers have a status"),
    )
    .expect("status fits in u16");
    let response_headers: BTreeMap<String, String> = headers
        .as_object()
        .expect("headers object")
        .iter()
        .filter(|(name, _)| *name != "status")
        .map(|(name, value)| {
            (
                name.clone(),
                value.as_str().expect("header value is a string").to_owned(),
            )
        })
        .collect();
    let body = fs::read(fixture.join("response.body")).expect("response body exists");
    let response =
        Response::new(url.to_owned(), status, response_headers, body).expect("bounded response");
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

fn render_request(template: &Value, frozen: &Value) -> Value {
    let values = frozen
        .get("substitutions")
        .and_then(Value::as_object)
        .expect("frozen substitutions are an object");
    let mut rendered = template.clone();
    for field in ["url", "body", "body_template"] {
        if let Some(text) = rendered.get(field).and_then(Value::as_str)
            && text.contains('{')
        {
            rendered[field] = Value::String(render(text, values));
        }
    }
    if let Some(headers) = rendered.get_mut("headers").and_then(Value::as_object_mut) {
        for value in headers.values_mut() {
            if let Some(text) = value.as_str()
                && text.contains('{')
            {
                *value = Value::String(render(text, values));
            }
        }
    }
    rendered
}

#[test]
fn tier3_b_fixtures_replay_through_adapter() {
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

        let template = read_json(&fixture.join("request.template.json"));
        let frozen = read_json(&fixture.join("frozen.json"));
        let request = read_json(&fixture.join("request.json"));
        assert_eq!(
            request,
            render_request(&template, &frozen),
            "{id} frozen request differs from template"
        );

        let values = substitutions(&manifest, &frozen);
        let built = request_with(&manifest, &values)
            .unwrap_or_else(|error| panic!("{id} request failed: {error}"));
        let (url, method, headers) = request_fields(&request);
        assert_eq!(built.url, url, "{id} adapter URL differs from fixture");
        assert_eq!(built.method, method, "{id} adapter method differs");
        for (name, value) in &headers {
            assert_eq!(
                built.headers.get(name).map(String::as_str),
                Some(value.as_str()),
                "{id} adapter header {name} differs"
            );
        }
        assert_eq!(built.body, Vec::<u8>::new(), "{id} has an unexpected body");

        let actual = normalized_records(&manifest, &fixture, &url);
        let expected = golden_records(&fixture.join("golden.ndjson"));
        assert_eq!(actual, expected, "{id} replay differs from golden");
    }
}

#[test]
fn keyed_manifests_build_requests_without_embedded_secret_values() {
    for (id, _, health, family_dir, _) in SOURCES {
        let manifest = manifest_for(id, family_dir);
        if health != Health::Keyed {
            continue;
        }
        assert!(!manifest.credentials.is_empty(), "{id} has no credentials");
        let manifest_text = read_text(
            &root()
                .join("src/sources/manifests")
                .join(family_dir)
                .join(format!("{id}.toml")),
        );
        for credential in &manifest.credentials {
            assert!(
                manifest_text.contains(credential),
                "{id} does not declare {credential}"
            );
        }
        let empty = build_request(&manifest, &TemplateValues::default());
        assert!(empty.is_err(), "{id} built without substitutions");
    }
}
