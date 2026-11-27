// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Arin Balyan
// Last modified: 2026-11-27

//! Result caps and fail-open policy.
//!
//! See documentation/006-architecture.md.

// Rust guideline compliant 2026-10-09

use std::collections::BTreeMap;

/// Failure of one source during a run.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SourceFailure {
    /// Identifier of the source that failed.
    pub source_id: String,
    /// Sanitized diagnostic.
    pub message: String,
}

/// Terminal summary of one run.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct RunSummary {
    /// Total records emitted.
    pub emitted: usize,
    /// Records skipped because of caps or failures.
    pub skipped: usize,
    /// Effective cap resolved for each source.
    pub caps: BTreeMap<String, usize>,
    /// Failures recorded while fail-open was enabled.
    pub failures: Vec<SourceFailure>,
}

impl RunSummary {
    /// Record one emitted record.
    pub fn emit(&mut self) {
        self.emitted += 1;
    }

    /// Record skipped records.
    pub fn skip(&mut self, count: usize) {
        self.skipped = self.skipped.saturating_add(count);
    }

    /// Record a source failure when fail-open is enabled.
    pub fn fail_open(&mut self, source_id: impl Into<String>, message: impl Into<String>) {
        self.failures.push(SourceFailure {
            source_id: source_id.into(),
            message: message.into(),
        });
    }
}

/// Caps and failure handling for one run.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct ResultPolicy {
    /// Maximum records for the whole run.
    pub global_cap: Option<usize>,
    /// Maximum records for each source.
    pub source_caps: BTreeMap<String, usize>,
    /// Continue after source failure when true.
    pub fail_open: bool,
}

impl ResultPolicy {
    /// Resolve the effective cap for one source.
    #[must_use]
    pub fn cap_for(&self, source_id: &str) -> Option<usize> {
        let source = self.source_caps.get(source_id).copied();
        match (self.global_cap, source) {
            (Some(global), Some(source)) => Some(global.min(source)),
            (Some(global), None) => Some(global),
            (None, source) => source,
        }
    }

    /// Return true when another record can be emitted for a source.
    #[must_use]
    pub fn allows(&self, source_id: &str, emitted: usize) -> bool {
        self.cap_for(source_id).is_none_or(|cap| emitted < cap)
    }

    /// Return true when a source failure should stop the run.
    #[must_use]
    pub fn stops_on_failure(&self, unknown_source: bool) -> bool {
        !self.fail_open || unknown_source
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn policy() -> ResultPolicy {
        ResultPolicy {
            global_cap: Some(5),
            source_caps: BTreeMap::from([("one".to_owned(), 10), ("two".to_owned(), 2)]),
            fail_open: true,
        }
    }

    #[test]
    fn per_source_cap_cannot_exceed_global_cap() {
        assert_eq!(policy().cap_for("one"), Some(5));
        assert_eq!(policy().cap_for("two"), Some(2));
    }

    #[test]
    fn global_cap_applies_across_sources() {
        let policy = policy();
        assert!(policy.allows("one", 4));
        assert!(!policy.allows("one", 5));
        assert!(!policy.allows("two", 2));
    }

    #[test]
    fn fail_open_records_failure() {
        let mut summary = RunSummary::default();
        summary.emit();
        summary.skip(2);
        summary.fail_open("two", "challenge");
        assert_eq!(summary.emitted, 1);
        assert_eq!(summary.skipped, 2);
        assert_eq!(summary.failures.len(), 1);
        assert_eq!(summary.failures[0].source_id, "two");
    }

    #[test]
    fn unknown_source_always_stops_run() {
        assert!(policy().stops_on_failure(true));
        assert!(!policy().stops_on_failure(false));
    }
}
