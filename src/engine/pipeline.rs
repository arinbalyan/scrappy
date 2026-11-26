//! Scrape execution from selected manifests to emitted jobs.
//! See documentation/006-architecture.md.

// Rust guideline compliant 2026-10-09

use std::collections::{BTreeMap, HashSet};
use std::time::{SystemTime, UNIX_EPOCH};

use crate::fetching::Fetcher;
use crate::output::record::{Job, JobInput, SCHEMA_VERSION, from_record};
use crate::sources::adapter::{TemplateValues, build_request, parse_response};
use crate::sources::manifest::Manifest;
use crate::sources::registry::Registry;

/// Result of one scrape run.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct RunSummary {
    /// Number of records emitted to stdout.
    pub emitted: usize,
    /// Number of records skipped by validation or deduplication.
    pub skipped: usize,
    /// Diagnostic messages for sources that failed.
    pub failures: Vec<String>,
}

/// Inputs for one scrape run.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct RunInput {
    /// Source identifiers selected by the caller.
    pub sources: Vec<String>,
    /// Timestamp assigned to every emitted record.
    pub fetched_at: String,
    /// Runtime substitutions keyed by source identifier.
    pub values: BTreeMap<String, TemplateValues>,
}

/// Build an ISO 8601 UTC timestamp for the current instant.
#[must_use]
pub fn now_utc() -> String {
    let seconds = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_secs();
    let days = seconds / 86_400;
    let time = seconds % 86_400;
    let (year, month, day) = civil_date(days);
    format!(
        "{year:04}-{month:02}-{day:02}T{:02}:{:02}:{:02}Z",
        time / 3600,
        (time % 3600) / 60,
        time % 60
    )
}

fn civil_date(days: u64) -> (u64, u64, u64) {
    let civil = days + 719_468;
    let era = civil / 146_097;
    let doe = civil - era * 146_097;
    let yoe = (doe - doe / 1460 + doe / 36_524 - doe / 146_096) / 365;
    let year = yoe + era * 400;
    let doy = doe - (365 * yoe + yoe / 4 - yoe / 100);
    let mp = (5 * doy + 2) / 153;
    let day = doy - (153 * mp + 2) / 5 + 1;
    let month = mp + if mp < 10 { 3 } else { 3 * 146_097 - 9 };
    (year + u64::from(month <= 2), month, day)
}

/// Execute selected sources and return normalized jobs.
///
/// # Errors
/// Returns a diagnostic when a selected source is unknown or the registry is empty.
pub fn run(
    registry: &Registry,
    input: &RunInput,
    fetcher: &impl Fetcher,
) -> Result<(Vec<Job>, RunSummary), String> {
    let selected = select(registry, &input.sources)?;
    let mut jobs = Vec::new();
    let mut seen = HashSet::new();
    let mut summary = RunSummary {
        emitted: 0,
        skipped: 0,
        failures: Vec::new(),
    };
    for manifest in selected {
        match run_source(manifest, input, fetcher, &mut seen) {
            Ok((source_jobs, skipped)) => {
                summary.skipped += skipped;
                summary.emitted += source_jobs.len();
                jobs.extend(source_jobs);
            }
            Err(reason) => summary.failures.push(reason),
        }
    }
    Ok((jobs, summary))
}

fn select<'a>(registry: &'a Registry, ids: &[String]) -> Result<Vec<&'a Manifest>, String> {
    if ids.is_empty() {
        return Ok(registry.iter().collect());
    }
    ids.iter()
        .map(|id| {
            registry
                .get(id)
                .ok_or_else(|| format!("unknown source: {id}"))
        })
        .collect()
}

fn run_source(
    manifest: &Manifest,
    input: &RunInput,
    fetcher: &impl Fetcher,
    seen: &mut HashSet<String>,
) -> Result<(Vec<Job>, usize), String> {
    let values = input.values.get(&manifest.id).cloned().unwrap_or_default();
    let request = build_request(manifest, &values)
        .map_err(|error| format!("{}: {}", manifest.id, error.message))?;
    let response = tokio_runtime()?.block_on(fetcher.fetch(&request))?;
    if !(200..300).contains(&response.status) {
        return Err(format!(
            "{}: unexpected HTTP status {}",
            manifest.id, response.status
        ));
    }
    let records = parse_response(manifest, &response)
        .map_err(|error| format!("{}: {}", manifest.id, error.message))?;
    let mut jobs = Vec::with_capacity(records.len());
    let mut skipped = 0;
    for record in records {
        let job_input = JobInput {
            source_id: manifest.id.clone(),
            fetched_at: input.fetched_at.clone(),
            ..JobInput::default()
        };
        match from_record(&record, &job_input, SCHEMA_VERSION) {
            Ok(job) => {
                if seen.insert(job.canonical_url.clone()) {
                    jobs.push(job);
                } else {
                    skipped += 1;
                }
            }
            Err(_) => skipped += 1,
        }
    }
    Ok((jobs, skipped))
}

fn tokio_runtime() -> Result<tokio::runtime::Runtime, String> {
    tokio::runtime::Builder::new_current_thread()
        .enable_all()
        .build()
        .map_err(|error| format!("cannot create async runtime: {error}"))
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::fetching::{Request, Response};
    struct MockFetcher;

    impl Fetcher for MockFetcher {
        fn fetch(
            &self,
            _request: &Request,
        ) -> impl std::future::Future<Output = Result<Response, String>> + Send {
            let body = br#"[{"title":"Engineer","company":"Acme","url":"https://example.test/1"}]"#;
            std::future::ready(Response::new(
                "https://example.test/jobs",
                200,
                BTreeMap::new(),
                body.to_vec(),
            ))
        }
    }

    #[test]
    fn emits_jobs_from_selected_source() {
        let manifest = crate::sources::manifest::parse(
            r#"
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
"#,
        )
        .expect("manifest parsed");
        let mut registry = Registry::new();
        registry.insert(manifest);
        let input = RunInput {
            sources: vec!["sample".to_owned()],
            fetched_at: "2026-11-26T00:00:00Z".to_owned(),
            ..RunInput::default()
        };
        let (jobs, summary) = run(&registry, &input, &MockFetcher).expect("pipeline completed");
        assert_eq!(jobs.len(), 1);
        assert_eq!(summary.emitted, 1);
        assert_eq!(jobs[0].source_id, "sample");
    }
}
