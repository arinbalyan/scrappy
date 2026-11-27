//! Offline checks for credential redaction and diagnostic masking.

// Rust guideline compliant 2026-10-09

use scrappy::cli::diagnostic::{mask_field, safe_text, safe_url};
use scrappy::cli::trace::{masked_event, masked_field, masked_url};
use scrappy::config::secret::{credential, is_credential_name};

const SECRET: &str = "SyntheticSecretAbcd1234Efgh5678";

#[test]
fn credential_never_renders_its_value() {
    let value = credential(SECRET);
    assert_eq!(value.as_str(), SECRET);
    let debug = format!("{value:?}");
    let display = value.to_string();
    assert!(debug.contains("[redacted]"));
    assert!(display.contains("[redacted]"));
    assert!(!debug.contains(SECRET));
    assert!(!display.contains(SECRET));
}

#[test]
fn sensitive_fields_are_masked() {
    for name in [
        "token",
        "authorization",
        "cookie",
        "email",
        "phone",
        "credential",
        "secret",
    ] {
        let rendered = masked_field(name, SECRET);
        assert!(rendered.contains("[redacted]"), "{name}");
        assert!(!rendered.contains(SECRET), "{name}");
    }
}

#[test]
fn url_fields_lose_query_and_userinfo() {
    let url = format!("https://user:{SECRET}@example.test/jobs?token={SECRET}#frag");
    let rendered = masked_url(&url);
    assert!(!rendered.contains(SECRET));
    assert!(!rendered.contains("frag"));
    assert!(rendered.contains("[query-redacted]"));
    assert!(rendered.contains("example.test/jobs"));
    assert_eq!(mask_field("request.url", &url), rendered);
}

#[test]
fn events_and_text_never_emit_the_secret() {
    let query = format!("https://example.test/?key={SECRET}");
    let event = masked_event(
        "http.request",
        "info",
        &[("api_token", SECRET), ("url", &query)],
    );
    assert!(!event.contains(SECRET));
    assert!(!safe_text(&format!("error with {SECRET}")).contains(SECRET));
    assert!(!safe_url(&query).contains(SECRET));
}

#[test]
fn credential_names_are_recognized() {
    assert!(is_credential_name("SCRAPPY_API_KEY"));
    assert!(is_credential_name("SCRAPPY_EXA_API_TOKEN"));
    assert!(!is_credential_name("SCRAPPY_FORMAT"));
}
