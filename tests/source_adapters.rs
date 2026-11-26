//! Offline tests for the manifest-to-request-to-record path.

// Rust guideline compliant 2026-10-09

use std::collections::BTreeMap;

use scrappy::fetching::Response;
use scrappy::sources::adapter::{
    AdapterError, Substitution, TemplateValues, build_request, parse_response, request_with,
};
use scrappy::sources::manifest::parse;

const TEMPLATED: &str = r#"
version = 1
id = "templed"
name = "Templated"
family = "feeds_api"
health = "working"
request = "https://example.test/{slug}/jobs?key={key:api_key}&page={query:page}"
request_template = "json-api"
parser = "json"
fetcher = "plain"
pagination = "none"
profile = "none"
requires_js = false
requests_per_minute = 30
max_concurrency = 2
robots = "allow"
attribution = "Example"
credentials = ["api_key"]
"#;

const SIMPLE: &str = r#"
version = 1
id = "sample"
name = "Sample"
family = "feeds_api"
health = "working"
request = "https://example.test/jobs"
parser = "json"
fetcher = "plain"
pagination = "none"
profile = "none"
requires_js = false
requests_per_minute = 30
max_concurrency = 2
robots = "allow"
attribution = "Example"
"#;

#[test]
fn builds_request_with_slug_key_and_query_substitutions() {
    let manifest = parse(TEMPLATED).expect("manifest parsed");
    let substitutions = [
        Substitution::Slug("acme".to_owned()),
        Substitution::Key {
            name: "api_key".to_owned(),
            value: "top-secret".to_owned(),
        },
        Substitution::Query {
            name: "page".to_owned(),
            value: "2".to_owned(),
        },
    ];
    let request = request_with(&manifest, &substitutions).expect("request built");
    assert_eq!(
        request.url,
        "https://example.test/acme/jobs?key=top-secret&page=2"
    );
    assert_eq!(request.method, "GET");
    assert_eq!(
        request.headers.get("accept").map(String::as_str),
        Some("application/json")
    );
}

#[test]
fn missing_required_substitution_is_an_error() {
    let manifest = parse(TEMPLATED).expect("manifest parsed");
    let substitutions = [
        Substitution::Slug("acme".to_owned()),
        Substitution::Key {
            name: "api_key".to_owned(),
            value: "top-secret".to_owned(),
        },
    ];
    let error = request_with(&manifest, &substitutions).unwrap_err();
    assert_eq!(
        error,
        AdapterError {
            message: "missing query substitution for page".to_owned(),
        }
    );
    assert!(!error.message.contains("top-secret"));
}

#[test]
fn parses_records_through_adapter() {
    let manifest = parse(SIMPLE).expect("manifest parsed");
    let request = build_request(&manifest, &TemplateValues::default()).expect("request built");
    let body = br#"[{"title":"Engineer","company":"Acme","url":"https://example.test/1"}]"#;
    let response =
        Response::new(request.url, 200, BTreeMap::new(), body.to_vec()).expect("bounded response");
    let records = parse_response(&manifest, &response).expect("records parsed");
    assert_eq!(records.len(), 1);
    assert_eq!(records[0].title, "Engineer");
    assert_eq!(records[0].company, "Acme");
}

#[test]
fn credential_value_never_appears_in_errors() {
    let manifest = parse(TEMPLATED).expect("manifest parsed");
    let mut values = TemplateValues::default();
    values
        .credentials
        .insert("api_key".to_owned(), "top-secret".to_owned());
    let error = build_request(&manifest, &values).unwrap_err();
    assert_eq!(
        error,
        AdapterError {
            message: "missing slug substitution".to_owned(),
        }
    );
    assert!(!error.message.contains("top-secret"));
}
