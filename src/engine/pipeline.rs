// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Arin Balyan
// Last modified: 2026-11-27

//! Scrape execution from selected manifests to emitted jobs.
//! See documentation/006-architecture.md.

// Rust guideline compliant 2026-10-09

use std::collections::{BTreeMap, HashSet};
use std::io;
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

use crate::config::Config;
use crate::engine::attention::{AttentionEvent, attention_event};
use crate::engine::breaker::CircuitBreaker;
use crate::engine::challenge::detect;
use crate::engine::contract::{ProgressEvent, SourceReport, write_event};
use crate::engine::governor::{Governor, SourceBudget};
use crate::engine::limits::ResultPolicy;
use crate::engine::memory::MemoryCeiling;
use crate::engine::pagination::{self, Strategy};
use crate::engine::retry::{
    CancelToken, Failure, RetryOutcome, RetryPolicy, classify_status, method_allows_retry,
    parse_retry_after, retry_async,
};
use crate::fetching::{Fetcher, Request};
use crate::output::record::{
    Email, EmailSource, Job, JobInput, Provenance, SCHEMA_VERSION, from_record,
};
use crate::sources::adapter::{TemplateValues, build_request, parse_response};
use crate::sources::manifest::Manifest;
use crate::sources::registry::Registry;

/// Outcome of one source run before jobs are merged.
enum SourceOutcome {
    /// Parsed jobs, skipped record count, and counters.
    Jobs(Vec<Job>, usize, SourceReport),
    /// Challenge classified for human attention with counters.
    Challenge(AttentionEvent, SourceReport),
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
    /// Per-source counters for live reporting.
    pub reports: Vec<SourceReport>,
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

#[expect(clippy::ptr_arg, reason = "retry_async supplies &String")]
fn classify_fetch_error(error: &String) -> Failure {
    if let Some(rest) = error.strip_prefix("throttled:") {
        let seconds = rest
            .rsplit(':')
            .next()
            .and_then(|value| value.parse::<u64>().ok())
            .unwrap_or(0);
        return Failure::Throttled(Some(Duration::from_secs(seconds)));
    }
    if error.starts_with("server:")
        || error.contains("request failed")
        || error.contains("error sending request")
    {
        return Failure::Retryable;
    }
    Failure::Permanent
}

fn robots_allows(policy: &str) -> bool {
    let value = policy.trim().to_ascii_lowercase();
    value == "permit" || value.starts_with("allow")
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
    let mut summary = RunSummary {
        emitted: 0,
        skipped: 0,
        failures: Vec::new(),
        attention: Vec::new(),
        events: Vec::new(),
        run_id: run_identifier(&input.fetched_at),
        cancelled: false,
        reports: Vec::new(),
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
            push_event(&mut summary, input, Some(&manifest.id), "policy_skip");
            continue;
        }
        let budget = overrides(manifest, &input.config, &mut governor)?;
        if breaker.is_open(&manifest.id) {
            summary.failures.push(format!(
                "{}: circuit open after repeated challenges",
                manifest.id
            ));
            continue;
        }
        match run_source(manifest, input, fetcher, &breaker, &budget, &governor).await {
            Ok(SourceOutcome::Jobs(source_jobs, skipped, report)) => {
                summary.reports.push(report);
                summary.skipped += skipped;
                summary.emitted += source_jobs.len();
                jobs.extend(source_jobs);
                push_event(&mut summary, input, Some(&manifest.id), "success");
            }
            Ok(SourceOutcome::Challenge(event, report)) => {
                summary.reports.push(report);
                breaker.record_failure(&manifest.id, event.kind);
                summary.attention.push(event);
                push_event(&mut summary, input, Some(&manifest.id), "attention");
            }
            Err((RetryOutcome::Cancelled, error)) => {
                summary.cancelled = true;
                summary.failures.push(error);
                break;
            }
            Err((_, error)) => {
                let mut report = SourceReport {
                    source_id: manifest.id.clone(),
                    family: format!("{:?}", manifest.family),
                    outcome: "failure".to_owned(),
                    stop_reason: "error".to_owned(),
                    ..SourceReport::default()
                };
                report.elapsed_ms = 0;
                summary.reports.push(report);
                if input.policy.fail_open {
                    summary.failures.push(error);
                } else {
                    return Err(error);
                }
            }
        }
    }
    let jobs = merge_jobs(jobs);
    summary.emitted = jobs.len();
    push_event(&mut summary, input, None, "completed");
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

fn push_event(summary: &mut RunSummary, input: &RunInput, source_id: Option<&str>, status: &str) {
    let event = ProgressEvent {
        event: if status == "completed" {
            "run.completed".to_owned()
        } else {
            "source.progress".to_owned()
        },
        run_id: summary.run_id.clone(),
        source_id: source_id.map(str::to_owned),
        emitted: summary.emitted,
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

#[expect(
    clippy::too_many_lines,
    reason = "source execution keeps pagination, retry, and challenge handling together"
)]
async fn run_source(
    manifest: &Manifest,
    input: &RunInput,
    fetcher: &impl Fetcher,
    breaker: &CircuitBreaker,
    retry_policy: &RetryPolicy,
    governor: &Governor,
) -> Result<SourceOutcome, (RetryOutcome, String)> {
    let started = Instant::now();
    let mut report = SourceReport {
        source_id: manifest.id.clone(),
        family: format!("{:?}", manifest.family),
        outcome: "working".to_owned(),
        ..SourceReport::default()
    };
    let mut values = input.values.get(&manifest.id).cloned().unwrap_or_default();
    if values.slug.is_none() {
        values.slug = manifest.slugs.first().cloned();
    }
    if values.tenant.is_none() {
        values.tenant = manifest.tenant.clone();
    }
    if values.site.is_none() {
        values.site = manifest.site.clone();
    }
    for (name, value) in &manifest.query_defaults {
        values
            .query
            .entry(name.clone())
            .or_insert_with(|| value.clone());
    }
    if manifest
        .credentials
        .iter()
        .any(|name| !values.credentials.contains_key(name))
    {
        "keyed".clone_into(&mut report.outcome);
        "missing_credentials".clone_into(&mut report.stop_reason);
        report.elapsed_ms = u64::try_from(started.elapsed().as_millis()).unwrap_or(u64::MAX);
        return Ok(SourceOutcome::Jobs(Vec::new(), 0, report));
    }
    let strategy = pagination::parse(&manifest.pagination);
    let strategy = if pagination::template_supports(&manifest.request, strategy)
        || matches!(strategy, Strategy::Cursor | Strategy::Fanout { .. })
    {
        strategy
    } else {
        Strategy::None
    };
    if strategy == Strategy::Cursor {
        return run_cursor(
            manifest,
            input,
            fetcher,
            breaker,
            retry_policy,
            governor,
            report,
        )
        .await;
    }
    if let Strategy::Fanout { batch, max_ids } = strategy {
        return run_fanout(
            manifest,
            input,
            fetcher,
            governor,
            retry_policy,
            report,
            batch,
            max_ids,
        )
        .await;
    }
    let mut records = Vec::new();
    let mut seen_urls = HashSet::new();
    let budget = manifest.request_budget.unwrap_or(25).max(1);
    let mut cancelled = input.cancel.receiver();
    for (index, page) in strategy.values().into_iter().enumerate() {
        if report.requests >= budget {
            "budget_exhausted".clone_into(&mut report.stop_reason);
            break;
        }
        if !input.policy.allows(&manifest.id, records.len()) {
            "cap_reached".clone_into(&mut report.stop_reason);
            break;
        }
        if let Some(key) = strategy.query_key() {
            values.query.insert(key.to_owned(), page.to_string());
        }
        report.requests += 1;
        let _permit = match governor.acquire(&manifest.id).await {
            Ok(permit) => permit,
            Err(error) if index == 0 => {
                return Err((RetryOutcome::Failed, format!("{}: {error}", manifest.id)));
            }
            Err(_) => break,
        };
        let request = build_request(manifest, &values).map_err(|error| {
            (
                RetryOutcome::Failed,
                format!("{}: {}", manifest.id, error.message),
            )
        })?;
        if !seen_urls.insert(request.url.clone()) {
            "repeated_page".clone_into(&mut report.stop_reason);
            break;
        }
        let mut retry_policy = *retry_policy;
        retry_policy.max_retries = if method_allows_retry(&request.method, manifest.retry_post) {
            retry_policy.max_retries
        } else {
            0
        };
        let outcome = retry_async(
            retry_policy,
            &mut cancelled,
            || async {
                let response = fetcher.fetch(&request).await?;
                match classify_status(response.status) {
                    Some(Failure::Throttled(_)) => {
                        let retry_after = response
                            .headers
                            .get("retry-after")
                            .and_then(|value| parse_retry_after(value).ok())
                            .map_or(0, |delay| delay.as_secs());
                        Err(format!("throttled:{}:{retry_after}", response.status))
                    }
                    Some(Failure::Retryable) => Err(format!("server:{}", response.status)),
                    _ => Ok(response),
                }
            },
            classify_fetch_error,
        )
        .await;
        if *cancelled.borrow() {
            return Err((
                RetryOutcome::Cancelled,
                format!("{}: cancelled", manifest.id),
            ));
        }
        let response = match outcome {
            Ok(response) => response,
            Err((outcome, error)) if index == 0 => {
                return Err((outcome, format!("{}: {error}", manifest.id)));
            }
            Err(_) => {
                report.partial = true;
                "partial_failure".clone_into(&mut report.stop_reason);
                break;
            }
        };
        report.http_status = Some(response.status);
        let body = response.text().unwrap_or_default();
        report.bytes += body.len();
        if let Some(challenge) = detect(response.status, &response.headers, body) {
            if index == 0 {
                breaker.record_failure(&manifest.id, challenge.kind);
                "challenge".clone_into(&mut report.outcome);
                "challenge".clone_into(&mut report.stop_reason);
                report.elapsed_ms =
                    u64::try_from(started.elapsed().as_millis()).unwrap_or(u64::MAX);
                return Ok(SourceOutcome::Challenge(
                    attention_event(&manifest.id, &challenge, &input.config),
                    report,
                ));
            }
            report.partial = true;
            "partial_challenge".clone_into(&mut report.stop_reason);
            break;
        }
        if !(200..300).contains(&response.status) {
            if index == 0 {
                return Err((
                    RetryOutcome::Failed,
                    format!(
                        "{}: unexpected HTTP status {}",
                        manifest.id, response.status
                    ),
                ));
            }
            report.partial = true;
            "partial_status".clone_into(&mut report.stop_reason);
            break;
        }
        breaker.record_success(&manifest.id);
        let page_records = match parse_response(manifest, &response) {
            Ok(records) => records,
            Err(error) if index == 0 => {
                return Err((
                    RetryOutcome::Failed,
                    format!("{}: {}", manifest.id, error.message),
                ));
            }
            Err(_) => break,
        };
        report.pages += 1;
        report.raw_records += page_records.len();
        if page_records.is_empty() {
            "empty_page".clone_into(&mut report.stop_reason);
            break;
        }
        records.extend(page_records);
    }
    let (jobs, skipped, invalid, duplicate) = convert_records(records, manifest, input);
    report.skipped_invalid = invalid;
    report.skipped_duplicate = duplicate;
    report.emitted = jobs.len();
    report.elapsed_ms = u64::try_from(started.elapsed().as_millis()).unwrap_or(u64::MAX);
    if report.stop_reason.is_empty() {
        "completed".clone_into(&mut report.stop_reason);
    }
    Ok(SourceOutcome::Jobs(jobs, skipped, report))
}

fn convert_records(
    records: Vec<crate::parsing::Record>,
    manifest: &Manifest,
    input: &RunInput,
) -> (Vec<Job>, usize, usize, usize) {
    let mut jobs = Vec::with_capacity(records.len());
    let mut seen = HashSet::new();
    let mut skipped = 0;
    let mut invalid = 0;
    let mut duplicate = 0;
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
            source_job_id: record.source_job_id.clone(),
            fetched_at: input.fetched_at.clone(),
            is_remote: record.is_remote,
            date_posted: record.date_posted.clone(),
            compensation: record.compensation.clone(),
            salary_min: record.salary_min.map(|value| value.to_string()),
            salary_max: record.salary_max.map(|value| value.to_string()),
            salary_currency: record.salary_currency.clone(),
            salary_period: record.salary_period.clone(),
            apply_method: record.apply_method.clone(),
            apply_route: record.apply_route,
            job_url_direct: record.job_url_direct.clone(),
            company_url: record.company_url.clone(),
            industry: record.industry.clone(),
            emails,
            ..JobInput::default()
        };
        if let Ok(job) = from_record(&record, &job_input, SCHEMA_VERSION) {
            if seen.insert(job.canonical_url.clone()) {
                jobs.push(job);
            } else {
                skipped += 1;
                duplicate += 1;
            }
        } else {
            skipped += 1;
            invalid += 1;
        }
    }
    (jobs, skipped, invalid, duplicate)
}

async fn fetch_checked(
    manifest: &Manifest,
    values: &TemplateValues,
    fetcher: &impl Fetcher,
    governor: &Governor,
    retry_policy: &RetryPolicy,
    cancelled: &mut tokio::sync::watch::Receiver<bool>,
    report: &mut SourceReport,
) -> Result<crate::fetching::Response, (RetryOutcome, String)> {
    let _permit = governor
        .acquire(&manifest.id)
        .await
        .map_err(|error| (RetryOutcome::Failed, format!("{}: {error}", manifest.id)))?;
    report.requests += 1;
    let request = build_request(manifest, values).map_err(|error| {
        (
            RetryOutcome::Failed,
            format!("{}: {}", manifest.id, error.message),
        )
    })?;
    let mut policy = *retry_policy;
    if !method_allows_retry(&request.method, manifest.retry_post) {
        policy.max_retries = 0;
    }
    let outcome = retry_async(
        policy,
        cancelled,
        || async {
            let response = fetcher.fetch(&request).await?;
            match classify_status(response.status) {
                Some(Failure::Throttled(_)) => {
                    let retry_after = response
                        .headers
                        .get("retry-after")
                        .and_then(|value| parse_retry_after(value).ok())
                        .map_or(0, |delay| delay.as_secs());
                    Err(format!("throttled:{}:{retry_after}", response.status))
                }
                Some(Failure::Retryable) => Err(format!("server:{}", response.status)),
                _ => Ok(response),
            }
        },
        classify_fetch_error,
    )
    .await;
    outcome.map_err(|(outcome, error)| (outcome, format!("{}: {error}", manifest.id)))
}

fn json_path<'a>(value: &'a serde_json::Value, path: &str) -> Option<&'a serde_json::Value> {
    let mut current = value;
    for segment in path.split('.') {
        current = current.get(segment)?;
    }
    Some(current)
}

async fn run_cursor(
    manifest: &Manifest,
    input: &RunInput,
    fetcher: &impl Fetcher,
    breaker: &CircuitBreaker,
    retry_policy: &RetryPolicy,
    governor: &Governor,
    mut report: SourceReport,
) -> Result<SourceOutcome, (RetryOutcome, String)> {
    let started = Instant::now();
    let mut values = input.values.get(&manifest.id).cloned().unwrap_or_default();
    for (name, value) in &manifest.query_defaults {
        values
            .query
            .entry(name.clone())
            .or_insert_with(|| value.clone());
    }
    let mut records = Vec::new();
    let mut cursor: Option<String> = None;
    let mut seen = HashSet::new();
    let budget = manifest.request_budget.unwrap_or(25).max(1);
    let mut cancelled = input.cancel.receiver();
    for _ in 0..budget {
        if let Some(token) = &cursor {
            values.query.insert("cursor".to_owned(), token.clone());
        }
        let response = match fetch_checked(
            manifest,
            &values,
            fetcher,
            governor,
            retry_policy,
            &mut cancelled,
            &mut report,
        )
        .await
        {
            Ok(response) => response,
            Err(_) if report.requests > 1 => break,
            Err(error) => return Err(error),
        };
        report.http_status = Some(response.status);
        let body = response.text().unwrap_or_default();
        report.bytes += body.len();
        if let Some(challenge) = detect(response.status, &response.headers, body) {
            "challenge".clone_into(&mut report.outcome);
            "challenge".clone_into(&mut report.stop_reason);
            report.elapsed_ms = u64::try_from(started.elapsed().as_millis()).unwrap_or(u64::MAX);
            return Ok(SourceOutcome::Challenge(
                attention_event(&manifest.id, &challenge, &input.config),
                report,
            ));
        }
        if !(200..300).contains(&response.status) {
            report.partial = true;
            "status".clone_into(&mut report.stop_reason);
            break;
        }
        breaker.record_success(&manifest.id);
        let page = match parse_response(manifest, &response) {
            Ok(records) => records,
            Err(_) if report.pages > 0 => break,
            Err(error) => {
                return Err((
                    RetryOutcome::Failed,
                    format!("{}: {}", manifest.id, error.message),
                ));
            }
        };
        report.pages += 1;
        report.raw_records += page.len();
        records.extend(page);
        let path = manifest.cursor_path.as_deref().unwrap_or("next");
        let next = serde_json::from_str::<serde_json::Value>(body)
            .ok()
            .and_then(|value| json_path(&value, path).cloned())
            .and_then(|value| value.as_str().map(str::to_owned))
            .filter(|value| !value.trim().is_empty());
        match next {
            Some(token) if seen.insert(token.clone()) => cursor = Some(token),
            Some(_) => {
                "repeated_page".clone_into(&mut report.stop_reason);
                break;
            }
            None => {
                "completed".clone_into(&mut report.stop_reason);
                break;
            }
        }
    }
    let (jobs, skipped, invalid, duplicate) = convert_records(records, manifest, input);
    report.skipped_invalid = invalid;
    report.skipped_duplicate = duplicate;
    report.emitted = jobs.len();
    report.elapsed_ms = u64::try_from(started.elapsed().as_millis()).unwrap_or(u64::MAX);
    if report.stop_reason.is_empty() {
        "completed".clone_into(&mut report.stop_reason);
    }
    Ok(SourceOutcome::Jobs(jobs, skipped, report))
}

#[expect(
    clippy::too_many_arguments,
    reason = "fanout needs manifest, input, fetcher, governor, policy, report, batch, and limit"
)]
async fn run_fanout(
    manifest: &Manifest,
    input: &RunInput,
    fetcher: &impl Fetcher,
    governor: &Governor,
    retry_policy: &RetryPolicy,
    mut report: SourceReport,
    batch: u32,
    max_ids: u32,
) -> Result<SourceOutcome, (RetryOutcome, String)> {
    let started = Instant::now();
    let mut values = input.values.get(&manifest.id).cloned().unwrap_or_default();
    for (name, value) in &manifest.query_defaults {
        values
            .query
            .entry(name.clone())
            .or_insert_with(|| value.clone());
    }
    let mut cancelled = input.cancel.receiver();
    let response = fetch_checked(
        manifest,
        &values,
        fetcher,
        governor,
        retry_policy,
        &mut cancelled,
        &mut report,
    )
    .await?;
    report.http_status = Some(response.status);
    let body = response.text().unwrap_or_default();
    report.bytes += body.len();
    let ids = serde_json::from_str::<serde_json::Value>(body)
        .ok()
        .and_then(|value| {
            value.as_array().cloned().or_else(|| {
                json_path(&value, "items")
                    .or_else(|| json_path(&value, "jobs"))
                    .and_then(serde_json::Value::as_array)
                    .cloned()
            })
        })
        .unwrap_or_default();
    let item_template = manifest.fanout_item.clone().unwrap_or_default();
    let mut records = Vec::new();
    let limit = usize::try_from(max_ids).unwrap_or(usize::MAX).min(
        usize::try_from(batch)
            .unwrap_or(usize::MAX)
            .saturating_mul(10),
    );
    for id in ids.iter().take(limit) {
        let Some(id) = id
            .as_str()
            .map(str::to_owned)
            .or_else(|| id.as_u64().map(|v| v.to_string()))
        else {
            continue;
        };
        if item_template.is_empty() {
            break;
        }
        let url = item_template.replace("{id}", &id);
        let Ok(request) = Request::get(url) else {
            continue;
        };
        let Ok(_permit) = governor.acquire(&manifest.id).await else {
            break;
        };
        report.requests += 1;
        let Ok(item) = fetcher.fetch(&request).await else {
            continue;
        };
        report.http_status = Some(item.status);
        let text = item.text().unwrap_or_default();
        report.bytes += text.len();
        let wrapped = format!("[{text}]");
        let Ok(response) = crate::fetching::Response::new(
            request.url.clone(),
            item.status,
            item.headers.clone(),
            wrapped.into_bytes(),
        ) else {
            continue;
        };
        if let Ok(page) = parse_response(manifest, &response) {
            report.pages += 1;
            report.raw_records += page.len();
            records.extend(page);
        }
    }
    let (jobs, skipped, invalid, duplicate) = convert_records(records, manifest, input);
    report.skipped_invalid = invalid;
    report.skipped_duplicate = duplicate;
    report.emitted = jobs.len();
    "completed".clone_into(&mut report.stop_reason);
    report.elapsed_ms = u64::try_from(started.elapsed().as_millis()).unwrap_or(u64::MAX);
    Ok(SourceOutcome::Jobs(jobs, skipped, report))
}

fn merge_jobs(jobs: Vec<Job>) -> Vec<Job> {
    let mut output: Vec<Job> = Vec::new();
    let mut index_by_url: BTreeMap<String, usize> = BTreeMap::new();
    for job in jobs {
        if let Some(index) = index_by_url.get(&job.canonical_url).copied() {
            let existing = &mut output[index];
            for source in job.source_ids {
                if !existing.source_ids.contains(&source) {
                    existing.source_ids.push(source);
                }
            }
            existing.location = existing.location.take().or(job.location);
            existing.description = existing.description.take().or(job.description);
            existing.date_posted = existing.date_posted.take().or(job.date_posted);
            existing.compensation = existing.compensation.take().or(job.compensation);
            existing.job_url_direct = existing.job_url_direct.take().or(job.job_url_direct);
            existing.company_url = existing.company_url.take().or(job.company_url);
            existing.industry = existing.industry.take().or(job.industry);
            existing.emails.extend(job.emails);
        } else {
            index_by_url.insert(job.canonical_url.clone(), output.len());
            output.push(job);
        }
    }
    output
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::fetching::Response;
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

    #[test]
    fn merge_jobs_keeps_all_source_ids() {
        let record = crate::parsing::Record::new("https://example.test/jobs/1", "Engineer", "Acme")
            .expect("valid record");
        let first = from_record(
            &record,
            &JobInput {
                source_id: "one".to_owned(),
                fetched_at: "2026-11-27T00:00:00Z".to_owned(),
                ..JobInput::default()
            },
            SCHEMA_VERSION,
        )
        .expect("first job");
        let second = from_record(
            &record,
            &JobInput {
                source_id: "two".to_owned(),
                fetched_at: "2026-11-27T00:00:00Z".to_owned(),
                ..JobInput::default()
            },
            SCHEMA_VERSION,
        )
        .expect("second job");
        let merged = merge_jobs(vec![first, second]);
        assert_eq!(merged.len(), 1);
        assert_eq!(
            merged[0].source_ids,
            vec!["one".to_owned(), "two".to_owned()]
        );
    }

    struct CursorFetcher;

    impl Fetcher for CursorFetcher {
        fn fetch(
            &self,
            request: &Request,
        ) -> impl std::future::Future<Output = Result<Response, String>> + Send {
            let body = if request.url.contains("cursor=abc") {
                br#"{"next":null,"jobs":[{"title":"Two","company":"Acme","url":"https://example.test/2"}]}"#.to_vec()
            } else {
                br#"{"next":"abc","jobs":[{"title":"One","company":"Acme","url":"https://example.test/1"}]}"#.to_vec()
            };
            std::future::ready(Response::new(
                request.url.clone(),
                200,
                BTreeMap::new(),
                body,
            ))
        }
    }

    #[tokio::test]
    async fn cursor_pages_stop_on_null_next() {
        let manifest = crate::sources::manifest::parse(
            r#"
version = 1
id = "cursor"
name = "Cursor"
family = "feeds_api"
health = "working"
request = "https://example.test/jobs?cursor={query:cursor}"
parser = "json"
fetcher = "plain"
pagination = "cursor:next"
cursor_path = "next"
profile = "none"
requires_js = false
requests_per_minute = 30
max_concurrency = 1
robots = "allow"
attribution = "Example"
[query_defaults]
cursor = ""
[parser_options]
envelope = ["jobs"]
"#,
        )
        .expect("manifest parsed");
        let mut registry = Registry::new();
        registry.insert(manifest);
        let input = RunInput {
            sources: vec!["cursor".to_owned()],
            fetched_at: "2026-11-27T00:00:00Z".to_owned(),
            ..RunInput::default()
        };
        let (jobs, summary) = run(&registry, &input, &CursorFetcher)
            .await
            .expect("pipeline completed");
        assert_eq!(jobs.len(), 2);
        assert_eq!(summary.reports[0].requests, 2);
        assert_eq!(summary.reports[0].stop_reason, "completed");
    }

    struct FanoutFetcher;

    impl Fetcher for FanoutFetcher {
        fn fetch(
            &self,
            request: &Request,
        ) -> impl std::future::Future<Output = Result<Response, String>> + Send {
            let body = if request.url.ends_with("/ids") {
                br"[1,2]".to_vec()
            } else if request.url.ends_with("/item/1.json") {
                br#"{"title":"One","company":"Acme","url":"https://example.test/1"}"#.to_vec()
            } else {
                br#"{"title":"Two","company":"Acme","url":"https://example.test/2"}"#.to_vec()
            };
            std::future::ready(Response::new(
                request.url.clone(),
                200,
                BTreeMap::new(),
                body,
            ))
        }
    }

    #[tokio::test]
    async fn fanout_requests_items_and_stops_at_limit() {
        let manifest = crate::sources::manifest::parse(
            r#"
version = 1
id = "fanout"
name = "Fanout"
family = "feeds_api"
health = "working"
request = "https://example.test/ids"
parser = "json"
fetcher = "plain"
pagination = "fanout:10:2"
fanout_item = "https://example.test/item/{id}.json"
profile = "none"
requires_js = false
requests_per_minute = 30
max_concurrency = 1
robots = "allow"
attribution = "Example"
"#,
        )
        .expect("manifest parsed");
        let mut registry = Registry::new();
        registry.insert(manifest);
        let input = RunInput {
            sources: vec!["fanout".to_owned()],
            fetched_at: "2026-11-27T00:00:00Z".to_owned(),
            ..RunInput::default()
        };
        let (jobs, summary) = run(&registry, &input, &FanoutFetcher)
            .await
            .expect("pipeline completed");
        assert_eq!(jobs.len(), 2);
        assert_eq!(summary.reports[0].requests, 3);
        assert_eq!(summary.reports[0].stop_reason, "completed");
    }
}
