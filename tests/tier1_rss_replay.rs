// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Arin Balyan
// Last modified: 2026-11-27

//! Offline replays for the Tier 1 feed sources in this port.
//!
//! Every fixture is rendered from its template and parsed through
//! `scrappy::sources::adapter::parse_response`. The normalized records are
//! compared with the committed golden lines. No network access is used.

// Rust guideline compliant 2026-10-09

use std::collections::BTreeMap;
use std::fs;
use std::path::{Path, PathBuf};

use scrappy::fetching::Response;
use scrappy::sources::adapter::{TemplateValues, build_request, parse_response};
use scrappy::sources::manifest::{Family, Health, Manifest, parse};
use serde_json::Value;

const SOURCES: [(&str, Health, &str); 11] = [
    ("cryptocurrencyjobs", Health::Working, "rss"),
    ("fossjobs", Health::Working, "rss"),
    ("pythonjobs", Health::Working, "rss"),
    ("railsjobs", Health::Working, "rss"),
    ("golangjobs", Health::Working, "rss"),
    ("devopsjobs", Health::Working, "rss"),
    ("crunchboard", Health::Blocked, "rss"),
    ("jobspresso", Health::Working, "rss"),
    ("hasjob", Health::Working, "rss"),
    ("landingjobs", Health::Working, "json"),
    ("realworkfromanywhere", Health::Working, "rss"),
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

fn manifest(id: &str) -> Manifest {
    let path = root()
        .join("src/sources/manifests/feeds_api")
        .join(format!("{id}.toml"));
    parse(&read_text(&path))
        .unwrap_or_else(|error| panic!("invalid manifest {}: {error}", path.display()))
}

fn render(template: &str, values: &serde_json::Map<String, Value>) -> String {
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

fn template_values(id: &str, dir: &Path) -> (String, TemplateValues) {
    let frozen = read_json(&dir.join("frozen.json"));
    let substitutions = frozen
        .get("substitutions")
        .and_then(Value::as_object)
        .expect("frozen substitutions are an object");
    let template = read_json(&dir.join("request.template.json"));
    let template_url = template
        .get("url")
        .and_then(Value::as_str)
        .expect("template has a url");
    let request = read_json(&dir.join("request.json"));
    let url = render(template_url, substitutions);
    assert_eq!(request["url"], url, "{id} frozen URL differs from template");
    assert_eq!(
        request["method"], template["method"],
        "{id} method mismatch"
    );
    assert_eq!(
        request["headers"], template["headers"],
        "{id} request headers differ from template"
    );
    let values = TemplateValues {
        query: substitutions
            .iter()
            .filter_map(|(name, value)| {
                let value = value.as_str().expect("substitution is a string");
                name.strip_prefix("query:")
                    .map(|key| (key.to_owned(), value.to_owned()))
            })
            .collect(),
        ..TemplateValues::default()
    };
    (url, values)
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
    let response_headers: BTreeMap<String, String> = headers
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
    Response::new(url, status, response_headers, body).expect("bounded response")
}

fn actual_records(id: &str, manifest: &Manifest, dir: &Path) -> Vec<Value> {
    let (url, values) = template_values(id, dir);
    let request = build_request(manifest, &values)
        .unwrap_or_else(|error| panic!("{id} build failed: {error}"));
    assert_eq!(request.url, url, "{id} adapter URL differs");
    assert_eq!(request.method, "GET");
    let response = response(&url, dir);
    let records = parse_response(manifest, &response)
        .unwrap_or_else(|error| panic!("{id} replay failed: {error}"));
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

fn golden_records(dir: &Path) -> Vec<Value> {
    read_text(&dir.join("golden.ndjson"))
        .lines()
        .filter(|line| !line.trim().is_empty())
        .map(|line| serde_json::from_str(line).expect("golden line is JSON"))
        .collect()
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

fn replay(id: &str) -> (Manifest, Vec<Value>) {
    let manifest = manifest(id);
    let dir = fixture_dir(id);
    for name in FIXTURE_FILES {
        assert!(dir.join(name).is_file(), "{id} is missing {name}");
    }
    let actual = actual_records(id, &manifest, &dir);
    (manifest, actual)
}

#[test]
fn tier1_fixtures_replay_into_golden_records() {
    for (id, health, parser) in SOURCES {
        let (manifest, actual) = replay(id);
        assert_eq!(manifest.id, id);
        assert_eq!(manifest.family, Family::FeedsApi);
        assert_eq!(manifest.health, health);
        assert_eq!(manifest.parser, parser);
        assert_eq!(manifest.fixtures, vec![id.to_owned()]);
        assert!(manifest.credentials.is_empty(), "{id} declared credentials");
        let expected: Vec<Value> = golden_records(&fixture_dir(id))
            .into_iter()
            .map(|value| canonical(&value))
            .collect();
        let actual: Vec<Value> = actual.into_iter().map(|value| canonical(&value)).collect();
        assert_eq!(actual, expected, "{id} golden mismatch");
    }
}

#[test]
fn title_patterns_split_company_and_location() {
    let (_, crypto) = replay("cryptocurrencyjobs");
    assert_eq!(crypto[0]["title"], "Backend Engineer");
    assert_eq!(crypto[0]["company"], "Northstar Labs");
    assert!(crypto.iter().all(|record| {
        !record["title"]
            .as_str()
            .expect("title is a string")
            .contains(" at ")
    }));

    let (_, devops) = replay("devopsjobs");
    assert_eq!(devops[0]["title"], "Platform Engineer");
    assert_eq!(devops[0]["company"], "Cedar Systems");
    assert_eq!(devops[0]["location"], "Austin");

    let (_, crunchboard) = replay("crunchboard");
    assert_eq!(crunchboard[0]["title"], "Product Designer");
    assert_eq!(crunchboard[0]["company"], "Wavelength Studio");

    let (_, jobspresso) = replay("jobspresso");
    assert_eq!(jobspresso[0]["company"], "Summit Works");
    assert_eq!(jobspresso[0]["title"], "Senior Support Engineer");
}

#[test]
fn host_derived_companies_and_feed_descriptions_survive() {
    for id in [
        "fossjobs",
        "pythonjobs",
        "railsjobs",
        "golangjobs",
        "realworkfromanywhere",
    ] {
        let (_, records) = replay(id);
        assert_eq!(records.len(), 2, "{id} record count");
        assert!(
            records
                .iter()
                .all(|record| !record["company"].as_str().expect("company").is_empty()),
            "{id} company is empty"
        );
        assert!(
            records
                .iter()
                .all(|record| record["description"].as_str().expect("description").len() > 10),
            "{id} description is missing"
        );
    }

    let (_, foss) = replay("fossjobs");
    assert_eq!(
        foss[1]["description"],
        "Plain description with an ampersand & a bracket."
    );
    let (_, rails) = replay("railsjobs");
    assert_eq!(rails[0]["url"], "https://jobs.rubyonrails.org/jobs/5101");
    assert_eq!(rails[1]["url"], "https://jobs.rubyonrails.org/jobs/5102");
    let (_, golang) = replay("golangjobs");
    assert_eq!(golang[0]["title"], "Go Systems Engineer");
    assert_eq!(golang[1]["title"], "Unrelated Category Role");
    let (_, remote) = replay("realworkfromanywhere");
    assert!(
        remote[0]["description"]
            .as_str()
            .expect("description")
            .contains("remote"),
        "remote metadata was dropped"
    );
}

#[test]
fn atom_and_json_fixtures_normalize() {
    let (_, hasjob) = replay("hasjob");
    assert_eq!(hasjob.len(), 2);
    assert_eq!(hasjob[0]["location"], "Bengaluru");
    assert_eq!(hasjob[1]["location"], "Pune");
    assert!(
        hasjob[0]["description"]
            .as_str()
            .expect("description")
            .contains("responsive")
    );

    let (_, landing) = replay("landingjobs");
    assert_eq!(landing.len(), 2);
    assert!(landing.iter().all(|record| {
        record["url"]
            .as_str()
            .expect("url")
            .starts_with("https://landing.jobs/at/")
    }));
    assert_eq!(landing[0]["company"], "Maple Labs");
}
