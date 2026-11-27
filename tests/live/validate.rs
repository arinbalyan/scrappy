// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Arin Balyan
// Last modified: 2026-11-27

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

use scrappy::config::secret::credential;
use scrappy::engine::governor::{Governor, SourceBudget};
use scrappy::fetching::Fetcher;
use scrappy::fetching::plain::PlainFetcher;
use scrappy::sources::adapter::{TemplateValues, build_request, parse_response};
use scrappy::sources::manifest::{Manifest, parse};
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
        if name == "slug" || name == "seed_slug" {
            if values.slug.is_none() {
                values.slug = Some(value.to_owned());
            }
        } else if let Some(key) = name.strip_prefix("key:") {
            values.credentials.insert(key.to_owned(), credential(value));
        } else if let Some(query) = name.strip_prefix("query:") {
            values.query.insert(query.to_owned(), value.to_owned());
        } else {
            values.query.insert(name.clone(), value.to_owned());
        }
    }
    values
}

fn probe_slug(dir: &Path, manifest: &Manifest) -> Option<String> {
    let text = fs::read_to_string(dir.join("frozen.json")).ok()?;
    let frozen: Value = serde_json::from_str(&text).ok()?;
    let object = frozen.get("substitutions")?.as_object()?;
    let value = object
        .get("seed_slug")
        .or_else(|| object.get("slug"))?
        .as_str()?;
    if value == "example-co" {
        return None;
    }
    if manifest.id == "ats-oracle" {
        let tenant = value.split('-').next().unwrap_or(value);
        let region = value.rsplit('-').next().unwrap_or("us");
        return Some(format!("{tenant}.fa.{region}.oraclecloud.com"));
    }
    Some(value.to_owned())
}

#[derive(Debug)]
enum Outcome {
    Working(usize),
    Blocked(String),
    Stale(String),
}

/// Fetch errors that describe the local environment rather than the source.
fn is_environment_error(message: &str) -> bool {
    [
        "target host resolution failed",
        "dns error",
        "error sending request",
        "connect timeout",
        "connect timed out",
    ]
    .iter()
    .any(|marker| message.contains(marker))
}

/// Read a source-specific live seed from the environment.
fn live_seed_slug(id: &str) -> Option<String> {
    let name = format!(
        "SCRAPPY_LIVE_SEED_{}",
        id.to_ascii_uppercase().replace('-', "_")
    );
    let value = std::env::var(name).ok()?;
    let value = value.trim();
    (!value.is_empty()).then(|| value.to_owned())
}

/// Read the optional source id filter for a targeted live run.
fn live_only_sources() -> Option<BTreeSet<String>> {
    let value = std::env::var("SCRAPPY_LIVE_ONLY").ok()?;
    let ids = value
        .split(',')
        .map(str::trim)
        .filter(|id| !id.is_empty())
        .map(str::to_owned)
        .collect::<BTreeSet<_>>();
    (!ids.is_empty()).then_some(ids)
}

/// Select manifests and register their budgets for one live run.
fn live_manifests() -> Vec<Manifest> {
    let all = manifests();
    let only = live_only_sources();
    if let Some(ids) = &only {
        let unknown: Vec<String> = ids
            .iter()
            .filter(|id| !all.iter().any(|manifest| &manifest.id == *id))
            .cloned()
            .collect();
        assert!(unknown.is_empty(), "unknown live source ids: {unknown:?}");
    }
    all.into_iter()
        .filter(|manifest| only.as_ref().is_none_or(|ids| ids.contains(&manifest.id)))
        .collect()
}

fn classify(status: u16, count: usize, parse_error: Option<&str>) -> Outcome {
    if status == 401 || status == 403 || status == 429 || status >= 500 {
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

fn classify_build_error(message: &str) -> Option<&'static str> {
    if message.contains("missing credential substitution")
        || message.contains("missing query substitution")
    {
        Some("keyed")
    } else if message.contains("missing slug substitution") {
        Some("skipped: no live slug seed")
    } else {
        None
    }
}

#[tokio::test]
#[ignore = "requires live network access and egress"]
async fn report_live_source_health() {
    let selected = live_manifests();
    let runtime_governor = {
        let mut governor = Governor::new();
        for manifest in &selected {
            let budget = SourceBudget::new(manifest.requests_per_minute, manifest.max_concurrency)
                .expect("manifest budget is valid");
            governor
                .add_source(manifest.id.as_str(), budget)
                .expect("source registered once");
        }
        governor
    };
    let fetcher = PlainFetcher::new().with_timeout(Duration::from_secs(15));
    let mut working = Vec::new();
    let mut keyed = Vec::new();
    let mut blocked = Vec::new();
    let mut environment = Vec::new();
    let mut stale = Vec::new();
    let mut skipped = Vec::new();
    let mut unregistered = Vec::new();

    let all = selected;
    let limit = std::env::var("SCRAPPY_LIVE_LIMIT")
        .ok()
        .and_then(|value| value.parse::<usize>().ok())
        .unwrap_or(all.len());
    for manifest in all.into_iter().take(limit) {
        println!("[live] probing {}", manifest.id);
        let dir = root().join("tests/fixtures").join(&manifest.id);
        let mut values = substitutions(&dir);
        if let Some(slug) = probe_slug(&dir, &manifest) {
            values.slug = Some(slug);
        }
        if let Some(slug) = live_seed_slug(&manifest.id) {
            values.slug = Some(slug);
        }
        let request = match build_request(&manifest, &values) {
            Ok(request) => request,
            Err(error) => {
                let line = format!("{}: {}", manifest.id, error.message);
                match classify_build_error(&error.message) {
                    Some("keyed") => keyed.push(line),
                    Some("skipped: no live slug seed") => skipped.push(line),
                    _ => stale.push(line),
                }
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
                let line = format!("{}: {error}", manifest.id);
                if is_environment_error(&error) {
                    environment.push(line);
                } else {
                    blocked.push(line);
                }
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
            Outcome::Blocked(reason) | Outcome::Stale(reason) => {
                format!("{}: {reason}", manifest.id)
            }
        };
        match classify(response.status, count, parse_error.as_deref()) {
            Outcome::Blocked(_) => blocked.push(line),
            Outcome::Stale(_) => stale.push(line),
            Outcome::Working(_) => unreachable!("working records already recorded"),
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
    print_bucket("environment", &environment);
    print_bucket("stale", &stale);
    print_bucket("skipped", &skipped);
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
