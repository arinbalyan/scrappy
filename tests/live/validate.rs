//! Opt-in live source validation.
//!
//! Run with `cargo test --test live_validate -- --ignored --nocapture`.
//! The report goes to stdout so it can be redirected to a non-committed
//! file. Normal `cargo test` never executes this suite.

// Rust guideline compliant 2026-10-09

use std::collections::BTreeSet;
use std::fs;
use std::path::{Path, PathBuf};
use std::time::Duration;

use scrappy::engine::governor::{Governor, SourceBudget};
use scrappy::fetching::Fetcher;
use scrappy::fetching::plain::PlainFetcher;
use scrappy::sources::adapter::{TemplateValues, build_request, parse_response};
use scrappy::sources::manifest::{Health, Manifest, parse};
use scrappy::sources::registry::Registry;
use serde_json::Value;

fn root() -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR"))
}

fn manifests() -> Vec<Manifest> {
    let base = root().join("src/sources/manifests");
    let mut files = Vec::new();
    for family in ["feeds_api", "ats", "html_static", "html_rendered", "keyed"] {
        let dir = base.join(family);
        for entry in fs::read_dir(&dir)
            .expect("manifest family directory")
            .flatten()
        {
            let path = entry.path();
            if path.extension().is_some_and(|ext| ext == "toml") {
                files.push(path);
            }
        }
    }
    files.sort();
    let mut registry = Registry::new();
    for path in files {
        let text = fs::read_to_string(&path).expect("manifest readable");
        let manifest = parse(&text).expect("manifest parses");
        registry.insert(manifest);
    }
    registry.iter().cloned().collect()
}

fn substitutions(dir: &Path) -> TemplateValues {
    let mut values = TemplateValues::default();
    let Ok(text) = fs::read_to_string(dir.join("frozen.json")) else {
        return values;
    };
    let Ok(frozen) = serde_json::from_str::<Value>(&text) else {
        return values;
    };
    let Some(object) = frozen.get("substitutions").and_then(Value::as_object) else {
        return values;
    };
    for (name, value) in object {
        let Some(value) = value.as_str() else {
            continue;
        };
        if name == "slug" {
            values.slug = Some(value.to_owned());
        } else if let Some(key) = name.strip_prefix("key:") {
            values.credentials.insert(key.to_owned(), value.to_owned());
        } else if let Some(query) = name.strip_prefix("query:") {
            values.query.insert(query.to_owned(), value.to_owned());
        } else {
            values.query.insert(name.clone(), value.to_owned());
        }
    }
    values
}

#[derive(Debug)]
enum Outcome {
    Working(usize),
    Keyed(String),
    Blocked(String),
    Stale(String),
}

fn classify(status: u16, count: usize, parse_error: Option<&str>) -> Outcome {
    if (400..500).contains(&status) && status != 404 {
        Outcome::Keyed(format!("http {status}"))
    } else if status == 429 || status >= 500 {
        Outcome::Blocked(format!("http {status}"))
    } else if status == 404 {
        Outcome::Stale("http 404".to_owned())
    } else if count > 0 {
        Outcome::Working(count)
    } else if let Some(error) = parse_error {
        Outcome::Stale(format!("http {status}; parse failed: {error}"))
    } else {
        Outcome::Stale(format!("http {status} but no records"))
    }
}

#[tokio::test]
#[ignore = "requires live network access"]
async fn report_live_source_health() {
    let runtime_governor = {
        let mut governor = Governor::new();
        for manifest in manifests() {
            let budget = SourceBudget::new(manifest.requests_per_minute, manifest.max_concurrency)
                .expect("manifest budget is valid");
            governor
                .add_source(&manifest.id, budget)
                .expect("source registered once");
        }
        governor
    };
    let fetcher = PlainFetcher::new().with_timeout(Duration::from_secs(15));
    let mut working = Vec::new();
    let mut keyed = Vec::new();
    let mut blocked = Vec::new();
    let mut stale = Vec::new();
    let mut unregistered = Vec::new();

    for manifest in manifests() {
        let dir = root().join("tests/fixtures").join(&manifest.id);
        let values = substitutions(&dir);
        let request = match build_request(&manifest, &values) {
            Ok(request) => request,
            Err(error) => {
                blocked.push(format!("{}: request build failed: {error}", manifest.id));
                continue;
            }
        };
        let _permit = runtime_governor
            .acquire(&manifest.id)
            .await
            .expect("rate permit acquired");
        let response = match fetcher.fetch(&request).await {
            Ok(response) => response,
            Err(error) => {
                blocked.push(format!("{}: fetch failed: {error}", manifest.id));
                continue;
            }
        };
        let parsed = parse_response(&manifest, &response);
        let count = parsed.as_ref().map_or(0, Vec::len);
        let parse_error = parsed.err().map(|error| error.message);
        let line = match classify(response.status, count, parse_error.as_deref()) {
            Outcome::Working(count) => {
                working.push(format!("{}: {count} records", manifest.id));
                continue;
            }
            Outcome::Keyed(reason) | Outcome::Blocked(reason) => {
                format!("{}: {reason}", manifest.id)
            }
            Outcome::Stale(reason) => format!("{}: {reason}", manifest.id),
        };
        match manifest.health {
            Health::Keyed => keyed.push(line),
            Health::Blocked => blocked.push(line),
            Health::Stale | Health::Working | Health::RequiresSlug | Health::BrowserOnly => {
                stale.push(line);
            }
        }
    }

    let historical = git_live_sources();
    let registered: BTreeSet<String> = manifests()
        .into_iter()
        .map(|manifest| manifest.id)
        .collect();
    for source in historical {
        if !registered.contains(&source) {
            unregistered.push(source);
        }
    }

    println!("== live validation report ==");
    print_bucket("working", &working);
    print_bucket("keyed", &keyed);
    print_bucket("blocked", &blocked);
    print_bucket("stale", &stale);
    print_bucket("unregistered", &unregistered);
}

fn print_bucket(label: &str, entries: &[String]) {
    println!("{label}:");
    for entry in entries {
        println!("  - {entry}");
    }
}

fn git_live_sources() -> Vec<String> {
    let output = std::process::Command::new("git")
        .args(["ls-tree", "-r", "--name-only", "origin/main"])
        .output()
        .expect("git available");
    if !output.status.success() {
        return Vec::new();
    }
    let text = String::from_utf8_lossy(&output.stdout);
    let mut sources = Vec::new();
    for line in text.lines() {
        let Some(rest) = line.strip_prefix("internal/scraper/") else {
            continue;
        };
        let Some(source) = rest.strip_suffix("/scraper.go") else {
            continue;
        };
        if !source.contains('/') {
            sources.push(source.to_owned());
        }
    }
    sources.sort();
    sources.dedup();
    sources
}
