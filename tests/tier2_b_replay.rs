//! Offline replay of Tier 2 B fixtures through the real adapters.
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

const SOURCES: [(&str, Family, Health, &str, &str); 5] = [
    (
        "jobsdb",
        Family::FeedsApi,
        Health::Working,
        "feeds_api",
        "json",
    ),
    (
        "jobstreet",
        Family::FeedsApi,
        Health::Working,
        "feeds_api",
        "json",
    ),
    (
        "mycareersfuture",
        Family::FeedsApi,
        Health::Working,
        "feeds_api",
        "json",
    ),
    (
        "reed",
        Family::HtmlStatic,
        Health::Working,
        "html_static",
        "json",
    ),
    (
        "hiringcafe",
        Family::HtmlStatic,
        Health::Working,
        "html_static",
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
        let token = format!("{{query:{name}}}");
        rendered = rendered.replace(&token, value);
    }
    assert!(
        !rendered.contains("{query:"),
        "unresolved placeholder in {rendered}"
    );
    rendered
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

#[test]
fn tier2_b_fixtures_replay_through_adapter() {
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
        let template_body = template.get("body").and_then(serde_json::Value::as_str);
        for name in substitutions.keys() {
            let token = format!("{{query:{name}}}");
            assert!(
                template_url.contains(&token)
                    || template_body.is_some_and(|body| body.contains(&token)),
                "{id} template does not use substitution {name}"
            );
        }
        let mut expected = template.clone();
        expected["url"] = serde_json::Value::String(render_template(template_url, substitutions));
        if let Some(body_template) = template.get("body").and_then(serde_json::Value::as_str) {
            expected["body"] =
                serde_json::Value::String(render_template(body_template, substitutions));
        }
        assert_eq!(
            request, expected,
            "{id} frozen request differs from template"
        );

        let mut values = TemplateValues::default();
        for (name, value) in substitutions {
            values.query.insert(
                name.clone(),
                value.as_str().expect("substitution is a string").to_owned(),
            );
        }
        let built =
            build_request(&manifest, &values).unwrap_or_else(|error| panic!("{id}: {error}"));
        let expected_method = request
            .get("method")
            .and_then(serde_json::Value::as_str)
            .expect("request has a method");
        assert_eq!(built.method, expected_method);
        if let Some(body_template) = template.get("body").and_then(serde_json::Value::as_str) {
            let expected_body = render_template(body_template, substitutions);
            assert_eq!(
                built.body,
                expected_body.as_bytes(),
                "{id} request body differs from template"
            );
            assert_eq!(
                request.get("body").and_then(serde_json::Value::as_str),
                Some(expected_body.as_str()),
                "{id} frozen body differs from template"
            );
        }
        assert_eq!(built.url, request["url"].as_str().expect("request url"));

        let actual = normalized_records(&manifest, &fixture);
        let expected_records = golden_records(&fixture.join("golden.ndjson"));
        assert_eq!(
            actual, expected_records,
            "{id} replay differs from golden: {actual:?}"
        );
    }
}
