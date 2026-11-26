//! Normalized records for downstream consumers.
//!
//! See openspec/changes/oculus-foundation/specs/output-contract/spec.md.

// Rust guideline compliant 2026-10-09

use serde::{Deserialize, Serialize};

use crate::engine::dedup::{ContentFields, canonical_url, content_hash};
use crate::parsing::Record;

/// Current output schema version.
pub const SCHEMA_VERSION: u32 = 1;

/// Where an address was obtained from.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum EmailSource {
    /// Address found in a job description.
    Description,
    /// Address found on a company page.
    CompanyPage,
    /// Address found in a mailto link.
    Mailto,
    /// Address supplied directly by a source.
    Direct,
    /// Address inferred from a known pattern.
    Guessed,
}

/// Whether an address was observed or inferred.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum Provenance {
    /// Address was present in source material.
    Observed,
    /// Address was inferred from a pattern.
    Inferred,
}

/// One email entry with attribution.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct Email {
    /// Email address.
    pub addr: String,
    /// Whether the address passed verification.
    pub verified: bool,
    /// Where the address came from.
    pub source: EmailSource,
    /// Role associated with the address.
    pub role: Option<String>,
    /// Whether the address was observed or inferred.
    pub provenance: Provenance,
}

/// An emitted job ready for automation.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct Job {
    /// Output schema version.
    pub schema_version: u32,
    /// Source identifier that produced the job.
    pub source_id: String,
    /// Stable external identifier.
    pub external_id: String,
    /// Canonical job URL.
    pub canonical_url: String,
    /// Job title.
    pub title: String,
    /// Hiring company.
    pub company: String,
    /// Job location.
    pub location: Option<String>,
    /// Job description.
    pub description: Option<String>,
    /// When the job was fetched, as an ISO 8601 timestamp.
    pub fetched_at: String,
    /// Content hash for deduplication.
    pub content_hash: u64,
    /// Whether the job is remote.
    pub is_remote: Option<bool>,
    /// ISO date the job was posted.
    pub date_posted: Option<String>,
    /// Compensation description.
    pub compensation: Option<String>,
    /// How to apply for the job.
    pub apply_method: Option<String>,
    /// Direct application URL.
    pub job_url_direct: Option<String>,
    /// Company website URL.
    pub company_url: Option<String>,
    /// Authoritative company domain.
    pub company_domain: Option<String>,
    /// Company description.
    pub company_description: Option<String>,
    /// Industry label.
    pub industry: Option<String>,
    /// Deterministic quality score from 0 to 100.
    pub quality_score: u8,
    /// Sources that contributed to the record.
    pub source_ids: Vec<String>,
    /// Contact addresses with attribution.
    pub emails: Vec<Email>,
}

/// Inputs used to build one emitted job.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct JobInput {
    /// Source identifier.
    pub source_id: String,
    /// Timestamp assigned by the caller.
    pub fetched_at: String,
    /// Whether the job is remote.
    pub is_remote: Option<bool>,
    /// ISO date the job was posted.
    pub date_posted: Option<String>,
    /// Compensation description.
    pub compensation: Option<String>,
    /// How to apply for the job.
    pub apply_method: Option<String>,
    /// Direct application URL.
    pub job_url_direct: Option<String>,
    /// Company website URL.
    pub company_url: Option<String>,
    /// Company description.
    pub company_description: Option<String>,
    /// Industry label.
    pub industry: Option<String>,
    /// Contact addresses.
    pub emails: Vec<Email>,
}

/// Build the authoritative company domain from a company URL.
///
/// Returns `None` for a missing or invalid URL so that a guessed address can
/// never set the company domain.
#[must_use]
pub fn company_domain(company_url: Option<&str>) -> Option<String> {
    let url = company_url?;
    let without_scheme = url.split_once("://").map_or(url, |(_, rest)| rest);
    let authority = without_scheme.split('/').next().unwrap_or_default();
    let host = authority.rsplit('@').next().unwrap_or_default();
    if host.is_empty() || !host.contains('.') {
        return None;
    }
    Some(host.to_ascii_lowercase())
}

fn score_job(record: &Record, input: &JobInput) -> u8 {
    let description = record.description.as_deref().unwrap_or_default();
    let mut score = 0_u8;
    if !description.is_empty() {
        score += 20;
    }
    if input.is_remote.is_some() {
        score += 10;
    }
    if input.date_posted.is_some() {
        score += 10;
    }
    if input.company_url.is_some() {
        score += 10;
    }
    if input.job_url_direct.is_some() {
        score += 10;
    }
    if input
        .emails
        .iter()
        .any(|email| email.provenance == Provenance::Observed)
    {
        score += 20;
    }
    let domain = company_domain(input.company_url.as_deref());
    if domain.is_some_and(|domain| {
        input
            .emails
            .iter()
            .any(|email| email.addr.rsplit('@').next() == Some(domain.as_str()))
    }) {
        score += 20;
    }
    score.min(100)
}

/// Build an emitted job, validating required fields before returning it.
///
/// # Errors
/// Returns a diagnostic for an empty required field, an invalid canonical
/// URL, or an unsupported schema version supplied by the caller.
pub fn from_record(record: &Record, input: &JobInput, schema_version: u32) -> Result<Job, String> {
    if schema_version != SCHEMA_VERSION {
        return Err(format!(
            "unsupported output schema version: {schema_version}"
        ));
    }
    if input.source_id.trim().is_empty() {
        return Err("job source id must not be empty".to_owned());
    }
    if input.fetched_at.trim().is_empty() {
        return Err("job fetched timestamp must not be empty".to_owned());
    }
    let url = canonical_url(&record.url)?;
    let hash = content_hash(&ContentFields {
        url: &url,
        title: &record.title,
        company: &record.company,
        description: record.description.as_deref().unwrap_or_default(),
    });
    let domain = company_domain(input.company_url.as_deref());
    let external_id = format!("{}-{:016x}", input.source_id, hash);
    let source_ids = vec![input.source_id.clone()];
    Ok(Job {
        schema_version,
        source_id: input.source_id.clone(),
        external_id,
        canonical_url: url,
        title: record.title.clone(),
        company: record.company.clone(),
        location: record.location.clone(),
        description: record.description.clone(),
        fetched_at: input.fetched_at.clone(),
        content_hash: hash,
        is_remote: input.is_remote,
        date_posted: input.date_posted.clone(),
        compensation: input.compensation.clone(),
        apply_method: input.apply_method.clone(),
        job_url_direct: input.job_url_direct.clone(),
        company_url: input.company_url.clone(),
        company_domain: domain,
        company_description: input.company_description.clone(),
        industry: input.industry.clone(),
        quality_score: score_job(record, input),
        source_ids,
        emails: input.emails.clone(),
    })
}

/// Read one serialized job, rejecting an unsupported schema version.
///
/// # Errors
/// Returns a diagnostic for malformed JSON, an unknown version, or a record
/// that does not match the current schema.
pub fn read_versioned(text: &str) -> Result<Job, String> {
    let value: serde_json::Value =
        serde_json::from_str(text).map_err(|error| format!("invalid job record: {error}"))?;
    let version = value
        .get("schema_version")
        .and_then(serde_json::Value::as_u64)
        .ok_or_else(|| "job record has no schema version".to_owned())?;
    if version != u64::from(SCHEMA_VERSION) {
        return Err(format!(
            "unsupported output schema version: {version}; expected {SCHEMA_VERSION}"
        ));
    }
    serde_json::from_value(value).map_err(|error| format!("invalid job record: {error}"))
}

#[cfg(test)]
mod tests {
    use super::*;

    fn record() -> Record {
        Record::new(
            "https://example.test/jobs/1?utm_source=x",
            "Rust Engineer",
            "Example Co",
        )
        .expect("valid record")
        .with_description(Some("Build reliable systems".to_owned()))
    }

    fn input() -> JobInput {
        JobInput {
            source_id: "greenhouse".to_owned(),
            fetched_at: "2026-11-26T00:00:00Z".to_owned(),
            company_url: Some("https://example.test/about".to_owned()),
            emails: vec![Email {
                addr: "jobs@example.test".to_owned(),
                verified: true,
                source: EmailSource::Description,
                role: None,
                provenance: Provenance::Observed,
            }],
            ..JobInput::default()
        }
    }

    #[test]
    fn serialization_includes_every_field() {
        let job = from_record(&record(), &input(), SCHEMA_VERSION).expect("valid job");
        let json = serde_json::to_string(&job).expect("serialized job");
        let value: serde_json::Value = serde_json::from_str(&json).expect("valid json");
        for field in [
            "schema_version",
            "source_id",
            "external_id",
            "canonical_url",
            "title",
            "company",
            "location",
            "description",
            "fetched_at",
            "content_hash",
            "is_remote",
            "date_posted",
            "compensation",
            "apply_method",
            "job_url_direct",
            "company_url",
            "company_domain",
            "company_description",
            "industry",
            "quality_score",
            "source_ids",
            "emails",
        ] {
            assert!(value.get(field).is_some(), "missing field: {field}");
        }
        assert_eq!(value["company_domain"], "example.test");
    }

    #[test]
    fn guessed_address_cannot_set_domain_or_inflate_score() {
        let mut input = input();
        input.company_url = Some("https://company.test".to_owned());
        input.emails = vec![Email {
            addr: "jobs@unrelated.test".to_owned(),
            verified: false,
            source: EmailSource::Guessed,
            role: None,
            provenance: Provenance::Inferred,
        }];
        let job = from_record(&record(), &input, SCHEMA_VERSION).expect("valid job");
        assert_eq!(job.company_domain.as_deref(), Some("company.test"));
        assert!(job.quality_score < 70);
        assert_eq!(job.emails[0].source, EmailSource::Guessed);
        assert_eq!(job.emails[0].provenance, Provenance::Inferred);
    }

    #[test]
    fn unknown_version_is_rejected() {
        let job = from_record(&record(), &input(), SCHEMA_VERSION).expect("valid job");
        let mut value = serde_json::to_value(&job).expect("json value");
        value["schema_version"] = serde_json::json!(999);
        let text = value.to_string();
        assert!(
            read_versioned(&text)
                .expect_err("unsupported version")
                .contains("unsupported output schema version")
        );
    }
}
