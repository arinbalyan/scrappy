// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Arin Balyan
// Last modified: 2026-11-27

//! Per-source circuit breaker for repeated challenge failures.
//!
//! See documentation/006-architecture.md.

// Rust guideline compliant 2026-10-09

use std::collections::BTreeMap;
use std::sync::Mutex;
use std::time::{Duration, Instant};

use super::challenge::ChallengeKind;

/// Number of failures before a source is stopped.
pub const DEFAULT_FAILURE_THRESHOLD: usize = 3;
/// Cooldown before a stopped source can be tried again.
pub const DEFAULT_COOLDOWN: Duration = Duration::from_secs(300);

/// State of one source in the breaker.
#[derive(Debug)]
struct SourceState {
    failures: usize,
    open_until: Option<Instant>,
    kind: Option<ChallengeKind>,
}

/// Tracks failures independently for every source.
#[derive(Debug)]
pub struct CircuitBreaker {
    threshold: usize,
    cooldown: Duration,
    states: Mutex<BTreeMap<String, SourceState>>,
}

impl Default for CircuitBreaker {
    fn default() -> Self {
        Self::new(DEFAULT_FAILURE_THRESHOLD, DEFAULT_COOLDOWN)
    }
}

impl CircuitBreaker {
    /// Create a breaker with a failure threshold and cooldown.
    #[must_use]
    pub fn new(threshold: usize, cooldown: Duration) -> Self {
        Self {
            threshold: threshold.max(1),
            cooldown,
            states: Mutex::new(BTreeMap::new()),
        }
    }

    /// Return true when a source is currently stopped.
    #[must_use]
    pub fn is_open(&self, source_id: &str) -> bool {
        let Ok(mut states) = self.states.lock() else {
            return true;
        };
        match states.get_mut(source_id) {
            Some(state) => match state.open_until {
                Some(until) if until <= Instant::now() => {
                    state.open_until = None;
                    state.failures = 0;
                    false
                }
                Some(_) => true,
                None => false,
            },
            None => false,
        }
    }

    /// Record a challenge or repeated failure for a source.
    pub fn record_failure(&self, source_id: &str, kind: ChallengeKind) {
        let Ok(mut states) = self.states.lock() else {
            return;
        };
        let state = states.entry(source_id.to_owned()).or_insert(SourceState {
            failures: 0,
            open_until: None,
            kind: None,
        });
        state.failures += 1;
        state.kind = Some(kind);
        if state.failures >= self.threshold {
            state.open_until = Some(Instant::now() + self.cooldown);
        }
    }

    /// Clear failures after a successful request.
    pub fn record_success(&self, source_id: &str) {
        if let Ok(mut states) = self.states.lock() {
            states.remove(source_id);
        }
    }

    /// Return the challenge kind that stopped a source.
    #[must_use]
    pub fn kind(&self, source_id: &str) -> Option<ChallengeKind> {
        let states = self.states.lock().ok()?;
        states.get(source_id).and_then(|state| state.kind)
    }

    /// Return the number of failures recorded for a source.
    #[must_use]
    pub fn failures(&self, source_id: &str) -> usize {
        self.states
            .lock()
            .ok()
            .and_then(|states| states.get(source_id).map(|state| state.failures))
            .unwrap_or_default()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn challenge_stops_only_affected_source() {
        let breaker = CircuitBreaker::new(1, Duration::from_secs(60));
        breaker.record_failure("one", ChallengeKind::Captcha);
        assert!(breaker.is_open("one"));
        assert!(!breaker.is_open("two"));
        assert_eq!(breaker.kind("one"), Some(ChallengeKind::Captcha));
    }

    #[test]
    fn breaker_opens_at_threshold() {
        let breaker = CircuitBreaker::new(2, Duration::from_secs(60));
        breaker.record_failure("one", ChallengeKind::RepeatedForbidden);
        assert!(!breaker.is_open("one"));
        breaker.record_failure("one", ChallengeKind::RepeatedForbidden);
        assert!(breaker.is_open("one"));
        assert_eq!(breaker.failures("one"), 2);
    }

    #[test]
    fn success_clears_failures() {
        let breaker = CircuitBreaker::new(2, Duration::from_secs(60));
        breaker.record_failure("one", ChallengeKind::RepeatedThrottled);
        breaker.record_success("one");
        assert_eq!(breaker.failures("one"), 0);
        assert!(!breaker.is_open("one"));
    }

    #[test]
    fn cooldown_reopens_source() {
        let breaker = CircuitBreaker::new(1, Duration::from_millis(1));
        breaker.record_failure("one", ChallengeKind::Captcha);
        std::thread::sleep(Duration::from_millis(5));
        assert!(!breaker.is_open("one"));
    }
}
