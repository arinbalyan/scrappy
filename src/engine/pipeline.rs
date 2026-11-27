//! Scrape execution from selected manifests to emitted jobs.
//! See documentation/006-architecture.md.

// Rust guideline compliant 2026-10-09

use std::collections::{BTreeMap, HashSet};
use std::io;
use std::time::{Duration, SystemTime, UNIX_EPOCH};

use crate::config::Config;
use crate::engine::attention::{AttentionEvent, attention_event};
use crate::engine::breaker::CircuitBreaker;
use crate::engine::challenge::detect;
use crate::engine::contract::{ProgressEvent, write_event};
use crate::engine::governor::{Governor, SourceBudget};
use crate::engine::limits::ResultPolicy;
use crate::engine::memory::MemoryCeiling;
use crate::engine::retry::{CancelToken, Failure, RetryOutcome, RetryPolicy, retry_async};
use crate::fetching::Fetcher;
use crate::output::record::{
    Email, EmailSource, Job, JobInput, Provenance, SCHEMA_VERSION, from_record,
};
use crate::sources::adapter::{TemplateValues, build_request, parse_response};
use crate::sources::manifest::Manifest;
use crate::sources::registry::Registry;

/// Outcome of one source run before jobs are merged.
enum SourceOutcome {
    /// Parsed jobs and skipped record count.
    Jobs(Vec<Job>, usize),
    /// Challenge classified for human attention.
    Challenge(AttentionEvent),
}

/// Result of one scrape run.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct RunSummary {
    /// Number of records emitted to stdout.
    pub emitted: usize,
    /// Number of records skipped by validation or deduplication.
    pub skipped: usize,
    /// Diagnostic messages for sources that failed.
    pub failures: Vec<String>,
    /// Machine-readable challenges that need human attention.
    pub attention: Vec<AttentionEvent>,
    /// Per-source progress events for machine-readable stderr output.
    pub events: Vec<ProgressEvent>,
    /// Identifier shared by all events in one run.
    pub run_id: String,
    /// True when the run was stopped by cancellation.
    pub cancelled: bool,
}

/// Inputs for one scrape run.
#[derive(Debug, Default)]
pub struct RunInput {
    /// Source identifiers selected by the caller.
    pub sources: Vec<String>,
    /// Timestamp assigned to every emitted record.
    pub fetched_at: String,
    /// Runtime substitutions keyed by source identifier.
    pub values: BTreeMap<String, TemplateValues>,
    /// Active configuration used to bind attention resume state.
    pub config: Config,
    /// Cap and fail-open policy for the run.
    pub policy: ResultPolicy,
    /// Cancellation token for retry waits and the run loop.
    pub cancel: CancelToken,
    /// Stream that receives structured progress events.
    pub progress: Option<io::Stderr>,
}

/// Maximum length retained when scanning a description for an address.
const MAX_EMAIL_SCAN_CHARS: usize = 32_000;

fn run_identifier(fetched_at: &str) -> String {
    let stamp: u64 = fetched_at
        .bytes()
        .filter(u8::is_ascii_digit)
        .fold(0, |value, byte| {
            value.wrapping_mul(31).wrapping_add(u64::from(byte))
        });
    format!("run-{stamp:016x}")
}

fn robots_allows(policy: &str) -> bool {
    let value = policy.trim().to_ascii_lowercase();
    value == "allow" || value == "allowed" || value == "permit"
}

/// Extract addresses observed directly in a job description.
///
/// Addresses are deduplicated in first-seen order and tagged as observed.
/// No inferred or harvested address is produced by this function.
#[must_use]
pub fn observed_emails(description: Option<&str>) -> Vec<Email> {
    let Some(description) = description else {
        return Vec::new();
    };
    let mut emails = Vec::new();
    let mut seen = HashSet::new();
    let bytes = description.as_bytes();
    let mut start = 0;
    while start < bytes.len().min(MAX_EMAIL_SCAN_CHARS) {
        while start < bytes.len() && !is_local_part(bytes[start]) {
            start += 1;
        }
        let mut end = start;
        while end < bytes.len() && is_local_part(bytes[end]) {
            end += 1;
        }
        if end < bytes.len() && bytes[end] == b'@' {
            let mut domain_end = end + 1;
            while domain_end < bytes.len() && is_domain_part(bytes[domain_end]) {
                domain_end += 1;
            }
            let domain = &description[end + 1..domain_end];
            if domain.contains('.') && seen.insert(&description[start..domain_end]) {
                emails.push(Email {
                    addr: description[start..domain_end].to_owned(),
                    verified: false,
                    source: EmailSource::Description,
                    role: None,
                    provenance: Provenance::Observed,
                });
            }
            start = domain_end;
        } else {
            start = end.saturating_add(1);
        }
    }
    emails
}

fn is_local_part(byte: u8) -> bool {
    byte.is_ascii_alphanumeric() || matches!(byte, b'.' | b'_' | b'+' | b'-')
}

fn is_domain_part(byte: u8) -> bool {
    byte.is_ascii_alphanumeric() || matches!(byte, b'.' | b'-')
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
pub async fn run(
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
        attention: Vec::new(),
        events: Vec::new(),
        run_id: run_identifier(&input.fetched_at),
        cancelled: false,
    };
    let mut governor = Governor::new();
    let breaker = CircuitBreaker::default();
    let cancel_receiver = input.cancel.receiver();
    for manifest in selected {
        if *cancel_receiver.borrow() {
            summary.cancelled = true;
            break;
        }
        if let Some(megabytes) = input.config.memory_mb
            && !MemoryCeiling::from_mb(megabytes).allows_start()
        {
            return Err(format!(
                "memory ceiling of {megabytes} MB reached before source {}",
                manifest.id
            ));
        }
        if !robots_allows(&manifest.robots) {
            push_event(
                &mut summary,
                input,
                Some(&manifest.id),
                "policy_skip",
                &seen,
            );
            continue;
        }
        let budget = overrides(manifest, &input.config, &mut governor)?;
        let _permit = match governor.acquire(&manifest.id).await {
            Ok(permit) => permit,
            Err(error) => {
                summary.failures.push(format!("{}: {error}", manifest.id));
                continue;
            }
        };
        if breaker.is_open(&manifest.id) {
            summary.failures.push(format!(
                "{}: circuit open after repeated challenges",
                manifest.id
            ));
            continue;
        }
        match run_source(manifest, input, fetcher, &mut seen, &breaker, &budget).await {
            Ok(SourceOutcome::Jobs(source_jobs, skipped)) => {
                summary.skipped += skipped;
                summary.emitted += source_jobs.len();
                jobs.extend(source_jobs);
                push_event(&mut summary, input, Some(&manifest.id), "success", &seen);
            }
            Ok(SourceOutcome::Challenge(event)) => {
                breaker.record_failure(&manifest.id, event.kind);
                summary.attention.push(event);
                push_event(&mut summary, input, Some(&manifest.id), "attention", &seen);
            }
            Err((RetryOutcome::Cancelled, error)) => {
                summary.cancelled = true;
                summary.failures.push(error);
                break;
            }
            Err((_, error)) => {
                if input.policy.fail_open {
                    summary.failures.push(error);
                } else {
                    return Err(error);
                }
            }
        }
    }
    push_event(&mut summary, input, None, "completed", &seen);
    Ok((jobs, summary))
}

fn overrides(
    manifest: &Manifest,
    config: &Config,
    governor: &mut Governor,
) -> Result<RetryPolicy, String> {
    if governor
        .add_source(
            manifest.id.as_str(),
            SourceBudget::new(
                config
                    .rates
                    .as_ref()
                    .and_then(|rates| rates.get(&manifest.id))
                    .copied()
                    .unwrap_or(manifest.requests_per_minute),
                config.concurrency.unwrap_or(manifest.max_concurrency),
            )
            .map_err(|error| format!("{}: {error}", manifest.id))?,
        )
        .is_err()
    {
        return Err(format!("{}: source registered twice", manifest.id));
    }
    let mut policy = RetryPolicy::default();
    if let Some(seconds) = config.timeout {
        policy.max_delay = Duration::from_secs(seconds).min(policy.max_delay);
    }
    Ok(policy)
}

fn push_event(
    summary: &mut RunSummary,
    input: &RunInput,
    source_id: Option<&str>,
    status: &str,
    seen: &HashSet<String>,
) {
    let event = ProgressEvent {
        event: if status == "completed" {
            "run.completed".to_owned()
        } else {
            "source.progress".to_owned()
        },
        run_id: summary.run_id.clone(),
        source_id: source_id.map(str::to_owned),
        emitted: seen.len().min(summary.emitted),
        skipped: summary.skipped,
        status: status.to_owned(),
    };
    summary.events.push(event.clone());
    if let Some(stderr) = &input.progress {
        let _ = write_event(&mut stderr.lock(), &event);
    }
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

async fn run_source(
    manifest: &Manifest,
    input: &RunInput,
    fetcher: &impl Fetcher,
    seen: &mut HashSet<String>,
    breaker: &CircuitBreaker,
    retry_policy: &RetryPolicy,
) -> Result<SourceOutcome, (RetryOutcome, String)> {
    let values = input.values.get(&manifest.id).cloned().unwrap_or_default();
    let request = build_request(manifest, &values).map_err(|error| {
        (
            RetryOutcome::Failed,
            format!("{}: {}", manifest.id, error.message),
        )
    })?;
    let mut cancelled = input.cancel.receiver();
    let outcome = retry_async(
        *retry_policy,
        &mut cancelled,
        || fetcher.fetch(&request),
        |error: &String| {
            if error.contains("429") || error.contains("throttled") {
                Failure::Throttled(None)
            } else {
                Failure::Retryable
            }
        },
    )
    .await;
    if *cancelled.borrow() {
        return Err((
            RetryOutcome::Cancelled,
            format!("{}: cancelled", manifest.id),
        ));
    }
    let response =
        outcome.map_err(|(outcome, error)| (outcome, format!("{}: {error}", manifest.id)))?;
    if let Ok(text) = response.text()
        && let Some(challenge) = detect(response.status, &response.headers, text)
    {
        breaker.record_failure(&manifest.id, challenge.kind);
        return Ok(SourceOutcome::Challenge(attention_event(
            &manifest.id,
            &challenge,
            &input.config,
        )));
    }
    breaker.record_success(&manifest.id);
    if !(200..300).contains(&response.status) {
        return Err((
            RetryOutcome::Failed,
            format!(
                "{}: unexpected HTTP status {}",
                manifest.id, response.status
            ),
        ));
    }
    let records = parse_response(manifest, &response).map_err(|error| {
        (
            RetryOutcome::Failed,
            format!("{}: {}", manifest.id, error.message),
        )
    })?;
    let mut jobs = Vec::with_capacity(records.len());
    let mut skipped = 0;
    let discovery = input.config.email_discovery.unwrap_or(false);
    for record in records {
        if !input.policy.allows(&manifest.id, jobs.len()) {
            break;
        }
        let emails = observed_emails(record.description.as_deref())
            .into_iter()
            .filter(|email| discovery || email.provenance == Provenance::Observed)
            .collect();
        let job_input = JobInput {
            source_id: manifest.id.clone(),
            fetched_at: input.fetched_at.clone(),
            emails,
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
    Ok(SourceOutcome::Jobs(jobs, skipped))
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

    #[tokio::test]
    async fn emits_jobs_from_selected_source() {
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
        let (jobs, summary) = run(&registry, &input, &MockFetcher)
            .await
            .expect("pipeline completed");
        assert_eq!(jobs.len(), 1);
        assert_eq!(summary.emitted, 1);
        assert_eq!(jobs[0].source_id, "sample");
    }

    #[test]
    fn observed_emails_are_tagged_and_deduplicated() {
        let emails = observed_emails(Some(
            r"Apply to jobs@example.test or jobs@example.test; visit https://example.test",
        ));
        assert_eq!(emails.len(), 1);
        assert_eq!(emails[0].addr, "jobs@example.test");
        assert_eq!(emails[0].source, EmailSource::Description);
        assert_eq!(emails[0].provenance, Provenance::Observed);
        assert_eq!(observed_emails(None), [] as [Email; 0]);
        assert_eq!(observed_emails(Some("no address here")), [] as [Email; 0]);
    }

    struct EmailingFetcher;

    struct ChallengeFetcher;

    impl Fetcher for ChallengeFetcher {
        fn fetch(
            &self,
            _request: &Request,
        ) -> impl std::future::Future<Output = Result<Response, String>> + Send {
            let body = b"<title>cf-browser-verification</title>";
            std::future::ready(Response::new(
                "https://example.test/jobs",
                403,
                BTreeMap::new(),
                body.to_vec(),
            ))
        }
    }

    impl Fetcher for EmailingFetcher {
        fn fetch(
            &self,
            _request: &Request,
        ) -> impl std::future::Future<Output = Result<Response, String>> + Send {
            let body = br#"[{"title":"Engineer","company":"Acme","url":"https://example.test/1","description":"Mail jobs@example.test"}]"#;
            std::future::ready(Response::new(
                "https://example.test/jobs",
                200,
                BTreeMap::new(),
                body.to_vec(),
            ))
        }
    }

    #[tokio::test]
    async fn emitted_jobs_carry_observed_contact_provenance() {
        let manifest = crate::sources::manifest::parse(
            r#"version = 1
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
            fetched_at: "2026-11-27T00:00:00Z".to_owned(),
            ..RunInput::default()
        };
        let (jobs, summary) = run(&registry, &input, &EmailingFetcher)
            .await
            .expect("pipeline completed");
        assert_eq!(summary.emitted, 1);
        assert_eq!(jobs[0].emails.len(), 1);
        assert_eq!(jobs[0].emails[0].addr, "jobs@example.test");
        assert_eq!(jobs[0].emails[0].provenance, Provenance::Observed);
        assert_eq!(jobs[0].source_ids, vec!["sample".to_owned()]);
        assert_eq!(jobs[0].company_domain, None);
    }

    #[tokio::test]
    async fn challenge_response_is_classified_as_attention_not_failure() {
        let manifest = crate::sources::manifest::parse(
            r#"version = 1
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
            fetched_at: "2026-11-27T00:00:00Z".to_owned(),
            ..RunInput::default()
        };
        let (jobs, summary) = run(&registry, &input, &ChallengeFetcher)
            .await
            .expect("pipeline completed");
        assert_eq!(jobs, [] as [Job; 0]);
        assert_eq!(summary.failures, [] as [String; 0]);
        assert_eq!(summary.attention.len(), 1);
        assert_eq!(summary.attention[0].source_id, "sample");
        assert_eq!(summary.attention[0].status, 403);
    }
}
