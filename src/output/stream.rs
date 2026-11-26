//! Streaming output with atomic lines.
//!
//! See openspec/changes/oculus-foundation/specs/output-contract/spec.md.

// Rust guideline compliant 2026-10-09

use std::collections::BTreeSet;
use std::io::{self, Write};

use super::record::Job;

/// A stream that emits one complete line per job and flushes each line.
#[derive(Debug)]
pub struct Stream<W: Write> {
    out: W,
    emitted: usize,
    skipped: usize,
}

impl<W: Write> Stream<W> {
    /// Create a stream over the given writer.
    pub const fn new(out: W) -> Self {
        Self {
            out,
            emitted: 0,
            skipped: 0,
        }
    }

    /// Emit one job, flushing the complete line before returning.
    ///
    /// # Errors
    /// Returns an I/O error when writing or flushing fails. The caller may
    /// stop after an error; no partial line is written to the stream.
    pub fn emit(&mut self, job: &Job) -> io::Result<()> {
        super::writer::write_line(&mut self.out, job)?;
        self.emitted += 1;
        Ok(())
    }

    /// Record jobs skipped by caps or dedup.
    pub fn skip(&mut self, count: usize) {
        self.skipped = self.skipped.saturating_add(count);
    }

    /// Number of jobs emitted so far.
    #[must_use]
    pub const fn emitted(&self) -> usize {
        self.emitted
    }

    /// Number of jobs skipped so far.
    #[must_use]
    pub const fn skipped(&self) -> usize {
        self.skipped
    }

    /// Consume the stream and return the underlying writer.
    #[must_use]
    pub fn into_inner(self) -> W {
        self.out
    }
}

/// Deduplicate by title and company while preserving first-seen order.
///
/// This is opt-in and separate from URL dedup. A run that disables this
/// option keeps every record, including records sharing title and company.
#[must_use]
pub fn dedup_title_company(jobs: Vec<Job>) -> Vec<Job> {
    let mut seen = BTreeSet::new();
    jobs.into_iter()
        .filter(|job| seen.insert((job.title.clone(), job.company.clone())))
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::output::record::{
        Email, EmailSource, JobInput, Provenance, SCHEMA_VERSION, from_record,
    };
    use crate::parsing::Record;

    fn job(url: &str, title: &str, company: &str) -> Job {
        let record = Record::new(url, title, company).expect("valid record");
        let input = JobInput {
            source_id: "greenhouse".to_owned(),
            fetched_at: "2026-11-26T00:00:00Z".to_owned(),
            emails: vec![Email {
                addr: "jobs@example.test".to_owned(),
                verified: true,
                source: EmailSource::Description,
                role: None,
                provenance: Provenance::Observed,
            }],
            ..JobInput::default()
        };
        from_record(&record, &input, SCHEMA_VERSION).expect("valid job")
    }

    #[test]
    fn emitted_lines_survive_an_interrupted_run() {
        let mut stream = Stream::new(Vec::new());
        stream
            .emit(&job("https://example.test/1", "Engineer", "Example Co"))
            .expect("first job");
        stream
            .emit(&job("https://example.test/2", "Engineer", "Example Co"))
            .expect("second job");
        stream.skip(3);
        let emitted = stream.emitted();
        let skipped = stream.skipped();
        let bytes = stream.into_inner();
        let text = String::from_utf8(bytes).expect("utf8");
        for line in text.lines() {
            let _: serde_json::Value = serde_json::from_str(line).expect("complete json line");
        }
        assert_eq!(text.lines().count(), 2);
        assert_eq!(emitted, 2);
        assert_eq!(skipped, 3);
    }

    #[test]
    fn title_company_dedup_is_opt_in() {
        let jobs = vec![
            job("https://example.test/1", "Engineer", "Example Co"),
            job("https://example.test/2", "Engineer", "Example Co"),
        ];
        let deduped = dedup_title_company(jobs.clone());
        assert_eq!(deduped.len(), 1);
        assert_eq!(deduped[0].canonical_url, "https://example.test/1");
        assert_eq!(jobs.len(), 2);
    }
}
