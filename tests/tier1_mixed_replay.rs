//! Offline replay of Tier 1 fixtures through the real adapters.
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

const SOURCES: [(&str, Family, Health, &str, &str); 9] = [
    (
        "weworkremotely",
        Family::FeedsApi,
        Health::Working,
        "feeds_api",
        "rss",
    ),
    (
        "androidjobs",
        Family::FeedsApi,
        Health::Working,
        "feeds_api",
        "rss",
    ),
    (
        "ecojobs",
        Family::FeedsApi,
        Health::Working,
        "feeds_api",
        "rss",
    ),
    (
        "getonboard",
        Family::FeedsApi,
        Health::Working,
        "feeds_api",
        "json",
    ),
    (
        "4dayweek",
        Family::FeedsApi,
        Health::Working,
        "feeds_api",
        "json",
    ),
    (
        "huggingfacejobs",
        Family::HtmlStatic,
        Health::Working,
        "html_static",
        "json",
    ),
    (
        "gunio",
        Family::HtmlStatic,
        Health::Working,
        "html_static",
        "html",
    ),
    (
        "aijobs",
        Family::HtmlStatic,
        Health::Working,
        "html_static",
        "html",
    ),
    ("ycjobs", Family::Keyed, Health::Blocked, "keyed", "html"),
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
    let mut output = String::with_capacity(template.len());
    let bytes = template.as_bytes();
    let mut index = 0;
    while index < bytes.len() {
        if bytes[index] == b'{' && bytes.get(index + 1) == Some(&b'{') {
            output.push('{');
            index += 2;
            continue;
        }
        if bytes[index] == b'}' && bytes.get(index + 1) == Some(&b'}') {
            output.push('}');
            index += 2;
            continue;
        }
        if bytes[index] != b'{' {
            let ch = template[index..]
                .chars()
                .next()
                .expect("character at byte index");
            output.push(ch);
            index += ch.len_utf8();
            continue;
        }
        let end = template[index + 1..].find('}').unwrap() + index + 1;
        let name = &template[index + 1..end];
        let value = values
            .get(name)
            .unwrap_or_else(|| panic!("missing substitution {name} in {template}"));
        output.push_str(value.as_str().expect("substitution is a string"));
        index = end + 1;
    }
    output
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
fn tier1_fixtures_replay_through_adapter() {
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
        if let Some(template_body) = template.get("body").and_then(serde_json::Value::as_str) {
            expected["body"] =
                serde_json::Value::String(render_template(template_body, substitutions));
        }
        assert_eq!(
            request, expected,
            "{id} frozen request differs from template"
        );

        let mut values = TemplateValues::default();
        for (name, value) in substitutions {
            let rendered = value.as_str().expect("substitution is a string").to_owned();
            if template_url.contains(&format!("{{{name}}}")) {
                values.query.insert(name.clone(), rendered.clone());
            }
            if name == "slug" {
                values.slug = Some(rendered);
            }
        }
        if family == Family::FeedsApi {
            let built = build_request(&manifest, &values).expect("manifest builds a request");
            let expected_method = request
                .get("method")
                .and_then(serde_json::Value::as_str)
                .expect("request has a method");
            assert_eq!(built.method, expected_method);
            if let Some(options) = &manifest.request_options
                && let Some(body_template) = &options.body
            {
                let rendered = render_template(body_template, substitutions);
                assert_eq!(built.body, rendered.as_bytes());
                let fixture_body = request
                    .get("body")
                    .and_then(serde_json::Value::as_str)
                    .expect("request has a body");
                assert_eq!(rendered, fixture_body);
            }
        }

        let actual = normalized_records(&manifest, &fixture);
        let expected_records = golden_records(&fixture.join("golden.ndjson"));
        assert_eq!(
            actual, expected_records,
            "{id} replay differs from golden: {actual:?}"
        );
    }
}
