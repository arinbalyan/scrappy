// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Arin Balyan
// Last modified: 2026-11-27

//! Opt-in live harvest with record and contact counts.
//!
//! Run with `SCRAPPY_LIVE_REPORT=/tmp/report.json` cargo test --test `live_harvest` -- --ignored --nocapture.
//! The report contains counts only, never addresses or descriptions.

// Rust guideline compliant 2026-10-09

use std::collections::{BTreeMap, BTreeSet};
use std::fs;
use std::path::{Path, PathBuf};
use std::time::Duration;

use scrappy::config::Config;
use scrappy::config::secret::credential;
use scrappy::engine::pipeline::{self, RunInput, now_utc};
use scrappy::fetching::plain::PlainFetcher;
use scrappy::sources::adapter::TemplateValues;
use scrappy::sources::manifest::{Manifest, parse};
use scrappy::sources::registry::Registry;
use serde::Serialize;
use serde_json::Value;

const DEFAULT_REPORT: &str = "/tmp/scrappy-live-report.json";

/// One source row in the harvest report.
#[derive(Debug, Serialize)]
struct SourceResult {
    id: String,
    name: String,
    family: String,
    health: String,
    outcome: String,
    http_status: Option<u16>,
    records: usize,
    skipped: usize,
    emails: usize,
    domains: usize,
    requests: usize,
    pages: usize,
    elapsed_ms: u64,
    bytes: usize,
    detail: String,
}

/// Aggregate counts in the harvest report.
#[derive(Debug, Serialize, Clone, Copy, Default)]
struct Totals {
    sources_probed: usize,
    sources_with_records: usize,
    records: usize,
    skipped: usize,
    observed_emails: usize,
    unique_addresses: usize,
    unique_domains: usize,
    sources_with_emails: usize,
}

/// Observed addresses found per source.
#[derive(Debug, Serialize, Clone)]
struct EmailSourceCount {
    source_id: String,
    emails: usize,
    records_with_emails: usize,
}

/// Most frequent address domains in the harvest.
#[derive(Debug, Serialize)]
struct DomainCount {
    domain: String,
    count: usize,
}

/// Non-identifying live harvest report.
#[derive(Debug, Serialize)]
struct Report {
    generated_at: String,
    version: u32,
    totals: Totals,
    sources: Vec<SourceResult>,
    emails_by_source: Vec<EmailSourceCount>,
    top_domains: Vec<DomainCount>,
    timing: TimingSummary,
}

/// Latency and yield summary over working sources.
#[derive(Debug, Serialize)]
struct TimingSummary {
    working_samples: usize,
    median_ms: u64,
    p95_ms: u64,
    max_ms: u64,
    fastest: Vec<TimedSource>,
    slowest: Vec<TimedSource>,
}

/// One source latency row.
#[derive(Debug, Serialize)]
struct TimedSource {
    source_id: String,
    elapsed_ms: u64,
}

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
        registry.insert(parse(&text).expect("manifest parses"));
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
            continue;
        }
        if let Some(key) = name.strip_prefix("key:") {
            values.credentials.insert(key.to_owned(), credential(value));
        } else if let Some(query) = name.strip_prefix("query:") {
            values.query.insert(query.to_owned(), value.to_owned());
        } else {
            values.query.insert(name.clone(), value.to_owned());
        }
    }
    values
}

fn frozen_slug(dir: &Path, manifest: &Manifest) -> Option<String> {
    let text = fs::read_to_string(dir.join("frozen.json")).ok()?;
    let frozen: Value = serde_json::from_str(&text).ok()?;
    let substitutions = frozen.get("substitutions")?;
    let value = substitutions
        .get("seed_slug")
        .or_else(|| substitutions.get("slug"))?
        .as_str()?;
    if value == "example-co" {
        return None;
    }
    if manifest.id == "ats-oracle" && !value.contains('.') {
        let tenant = value.split('-').next().unwrap_or(value);
        let region = value.rsplit('-').next().unwrap_or("us");
        return Some(format!("{tenant}.fa.{region}.oraclecloud.com"));
    }
    Some(value.to_owned())
}

fn live_slug(manifest: &Manifest) -> Option<String> {
    let name = format!(
        "SCRAPPY_LIVE_SEED_{}",
        manifest.id.to_ascii_uppercase().replace('-', "_")
    );
    std::env::var(name)
        .ok()
        .map(|value| value.trim().to_owned())
        .filter(|value| !value.is_empty())
}

fn selected_manifests() -> Vec<Manifest> {
    let all = manifests();
    let Some(ids) = live_only_sources() else {
        return all;
    };
    let unknown: Vec<String> = ids
        .iter()
        .filter(|id| !all.iter().any(|manifest| &manifest.id == *id))
        .cloned()
        .collect();
    assert!(unknown.is_empty(), "unknown live source ids: {unknown:?}");
    let selected: Vec<Manifest> = all
        .into_iter()
        .filter(|manifest| ids.iter().any(|id| id == &manifest.id))
        .collect();
    selected
}

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

fn address_domain(address: &str) -> String {
    address.rsplit_once('@').map_or_else(
        || "unknown".to_owned(),
        |(_, domain)| domain.to_ascii_lowercase(),
    )
}

fn new_result(manifest: &Manifest) -> SourceResult {
    SourceResult {
        id: manifest.id.clone(),
        name: manifest.name.clone(),
        family: format!("{:?}", manifest.family),
        health: format!("{:?}", manifest.health),
        outcome: "skipped".to_owned(),
        http_status: None,
        records: 0,
        skipped: 0,
        emails: 0,
        domains: 0,
        requests: 0,
        pages: 0,
        elapsed_ms: 0,
        bytes: 0,
        detail: String::new(),
    }
}

/// Accumulates counts without retaining addresses or descriptions.
#[derive(Default)]
struct Harvest {
    results: Vec<SourceResult>,
    email_counts: Vec<EmailSourceCount>,
    addresses: BTreeSet<String>,
    domains: BTreeMap<String, usize>,
    totals: Totals,
}

impl Harvest {
    /// Fetches one manifest through the shared pipeline executor.
    #[expect(
        clippy::too_many_lines,
        reason = "live report keeps outcome and counters together"
    )]
    async fn probe(
        &mut self,
        manifest: &Manifest,
        registry: &Registry,
        fetcher: &PlainFetcher,
    ) -> SourceResult {
        let mut result = new_result(manifest);
        let dir = root().join("tests/fixtures").join(&manifest.id);
        let mut values = substitutions(&dir);
        if let Some(slug) = frozen_slug(&dir, manifest).or_else(|| live_slug(manifest)) {
            values.slug = Some(slug);
        }
        for (name, value) in &manifest.query_defaults {
            values
                .query
                .entry(name.clone())
                .or_insert_with(|| value.clone());
        }
        let mut all_values = BTreeMap::new();
        all_values.insert(manifest.id.clone(), values);
        let input = RunInput {
            sources: vec![manifest.id.clone()],
            fetched_at: now_utc(),
            values: all_values,
            config: Config {
                email_discovery: Some(true),
                ..Config::default()
            },
            ..RunInput::default()
        };
        let started = std::time::Instant::now();
        let (jobs, summary) = match pipeline::run(registry, &input, fetcher).await {
            Ok(result) => result,
            Err(error) => {
                result.outcome = "environment".to_owned();
                result.detail = error;
                result.elapsed_ms =
                    u64::try_from(started.elapsed().as_millis()).unwrap_or(u64::MAX);
                return result;
            }
        };
        result.elapsed_ms = u64::try_from(started.elapsed().as_millis()).unwrap_or(u64::MAX);
        if let Some(report) = summary.reports.first() {
            result.outcome.clone_from(&report.outcome);
            result.http_status = report.http_status;
            result.requests = report.requests;
            result.pages = report.pages;
            result.bytes = report.bytes;
            result.skipped = report.skipped_invalid + report.skipped_duplicate;
            result.detail = report.stop_reason.clone();
        }
        if let Some(failure) = summary.failures.first() {
            result.outcome = if failure.contains("403")
                || failure.contains("429")
                || failure.contains("challenge")
            {
                "blocked".to_owned()
            } else if failure.contains("resolution")
                || failure.contains("request failed")
                || failure.contains("timeout")
            {
                "environment".to_owned()
            } else {
                "stale".to_owned()
            };
            result.detail = failure.clone();
        }
        if result.outcome == "working" && jobs.is_empty() {
            result.outcome = "stale".to_owned();
            result.detail = "no emitted records".to_owned();
        }
        result.records = jobs.len();
        let mut source_addresses = BTreeSet::new();
        let mut source_domains = BTreeSet::new();
        let mut records_with_emails = 0;
        for job in &jobs {
            if !job.emails.is_empty() {
                records_with_emails += 1;
            }
            for email in &job.emails {
                source_addresses.insert(email.addr.clone());
                let domain = address_domain(&email.addr);
                source_domains.insert(domain.clone());
                *self.domains.entry(domain).or_insert(0) += 1;
            }
        }
        result.emails = jobs.iter().map(|job| job.emails.len()).sum();
        result.domains = source_domains.len();
        self.totals.sources_probed += 1;
        self.totals.records += jobs.len();
        self.totals.skipped += result.skipped;
        self.totals.observed_emails += result.emails;
        if !jobs.is_empty() {
            self.totals.sources_with_records += 1;
        }
        if !source_addresses.is_empty() {
            self.totals.sources_with_emails += 1;
        }
        self.addresses.extend(source_addresses);
        self.email_counts.push(EmailSourceCount {
            source_id: manifest.id.clone(),
            emails: result.emails,
            records_with_emails,
        });
        println!(
            "[harvest] {}: {} records, {} emails, {} requests, {}",
            manifest.id, result.records, result.emails, result.requests, result.detail
        );
        result
    }

    /// Writes the counts-only JSON report.
    fn write_report(mut self) {
        self.totals.unique_addresses = self.addresses.len();
        self.totals.unique_domains = self.domains.len();
        let mut top_domains: Vec<DomainCount> = self
            .domains
            .into_iter()
            .map(|(domain, count)| DomainCount { domain, count })
            .collect();
        top_domains.sort_by(|left, right| {
            right
                .count
                .cmp(&left.count)
                .then_with(|| left.domain.cmp(&right.domain))
        });
        top_domains.truncate(20);
        self.email_counts
            .sort_by_key(|left| std::cmp::Reverse(left.emails));
        let timing = timing_summary(&self.results);
        let report = Report {
            generated_at: now_utc(),
            version: 2,
            totals: self.totals,
            sources: self.results,
            emails_by_source: self.email_counts,
            top_domains,
            timing,
        };
        let path =
            std::env::var("SCRAPPY_LIVE_REPORT").unwrap_or_else(|_| DEFAULT_REPORT.to_owned());
        let encoded = serde_json::to_string_pretty(&report).expect("report serializes");
        let mut output = encoded;
        output.push(10 as char);
        fs::write(&path, output).expect("report written");
        println!("[harvest] report written to {path}");
        println!(
            "[harvest] totals: {} records, {} observed emails, {} unique addresses across {} sources",
            report.totals.records,
            report.totals.observed_emails,
            report.totals.unique_addresses,
            report.totals.sources_with_records
        );
    }
}

fn timing_summary(results: &[SourceResult]) -> TimingSummary {
    let mut timed: Vec<&SourceResult> = results
        .iter()
        .filter(|result| result.outcome == "working" && result.elapsed_ms > 0)
        .collect();
    timed.sort_by_key(|result| result.elapsed_ms);
    let percentile = |percent: usize| -> u64 {
        if timed.is_empty() {
            return 0;
        }
        let index = ((timed.len() * percent).div_ceil(100)).saturating_sub(1);
        timed[index.min(timed.len() - 1)].elapsed_ms
    };
    let mut fastest: Vec<TimedSource> = timed
        .iter()
        .take(10)
        .map(|result| TimedSource {
            source_id: result.id.clone(),
            elapsed_ms: result.elapsed_ms,
        })
        .collect();
    fastest.sort_by_key(|row| row.elapsed_ms);
    let mut slowest: Vec<TimedSource> = timed
        .iter()
        .rev()
        .take(10)
        .map(|result| TimedSource {
            source_id: result.id.clone(),
            elapsed_ms: result.elapsed_ms,
        })
        .collect();
    slowest.sort_by_key(|row| std::cmp::Reverse(row.elapsed_ms));
    TimingSummary {
        working_samples: timed.len(),
        median_ms: percentile(50),
        p95_ms: percentile(95),
        max_ms: timed.last().map_or(0, |result| result.elapsed_ms),
        fastest,
        slowest,
    }
}

/// Run one live harvest and write a non-identifying JSON report.
#[tokio::test]
#[ignore = "requires live network access and egress"]
async fn harvest_live_records_and_contacts() {
    let selected = selected_manifests();
    let mut registry = Registry::new();
    for manifest in &selected {
        registry.insert(manifest.clone());
    }
    let fetcher = PlainFetcher::new().with_timeout(Duration::from_secs(15));
    let mut harvest = Harvest::default();
    for manifest in &selected {
        let result = harvest.probe(manifest, &registry, &fetcher).await;
        harvest.results.push(result);
    }
    harvest.write_report();
}
