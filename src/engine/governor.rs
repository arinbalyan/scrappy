//! Per-source rate limits, concurrency caps, and the global kill switch.
//!
//! See documentation/006-architecture.md.

// Rust guideline compliant 2026-10-09

use governor::{Quota, RateLimiter};
use std::collections::BTreeMap;
use std::num::NonZeroU32;
use tokio::sync::{Semaphore, SemaphorePermit, watch};

/// Default requests per minute when a source does not declare a rate.
pub const DEFAULT_RATE_PER_MINUTE: usize = 60;
/// Default simultaneous requests when a source does not declare a cap.
pub const DEFAULT_CONCURRENCY: usize = 4;

/// A source-specific rate and concurrency budget.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct SourceBudget {
    /// Maximum requests per minute for one source.
    pub requests_per_minute: usize,
    /// Maximum simultaneous requests for one source.
    pub max_concurrency: usize,
}

impl SourceBudget {
    /// Create a budget after rejecting a zero rate or concurrency cap.
    ///
    /// # Errors
    /// Returns a diagnostic when either limit is zero.
    pub fn new(requests_per_minute: usize, max_concurrency: usize) -> Result<Self, String> {
        if requests_per_minute == 0 {
            return Err("requests per minute must be greater than zero".to_owned());
        }
        if max_concurrency == 0 {
            return Err("concurrency must be greater than zero".to_owned());
        }
        Ok(Self {
            requests_per_minute,
            max_concurrency,
        })
    }
}

/// Handle used to activate the global kill switch.
#[derive(Debug, Clone)]
pub struct KillSwitch {
    sender: watch::Sender<bool>,
}

impl KillSwitch {
    /// Stop all future requests immediately.
    pub fn activate(&self) {
        let _ = self.sender.send(true);
    }
}

#[derive(Debug)]
pub struct RequestPermit<'a> {
    _concurrency: SemaphorePermit<'a>,
}

/// Independent rate and concurrency control for every source.
#[derive(Debug)]
pub struct Governor {
    sources: BTreeMap<String, SourceLimits>,
    kill_switch: watch::Receiver<bool>,
    switch: KillSwitch,
}

#[derive(Debug)]
struct SourceLimits {
    rate: RateLimiter<
        governor::state::direct::NotKeyed,
        governor::state::InMemoryState,
        governor::clock::DefaultClock,
    >,
    concurrency: Semaphore,
}

impl Default for Governor {
    fn default() -> Self {
        Self::new()
    }
}

impl Governor {
    /// Build an empty governor with an inactive kill switch.
    #[must_use]
    pub fn new() -> Self {
        let (sender, kill_switch) = watch::channel(false);
        Self {
            sources: BTreeMap::new(),
            kill_switch,
            switch: KillSwitch { sender },
        }
    }

    /// Register a source with its own rate limiter and semaphore.
    ///
    /// # Errors
    /// Returns a diagnostic when the source budget is invalid.
    pub fn add_source(
        &mut self,
        source_id: impl Into<String>,
        budget: SourceBudget,
    ) -> Result<(), String> {
        let rate = u32::try_from(budget.requests_per_minute)
            .ok()
            .and_then(NonZeroU32::new)
            .ok_or_else(|| "rate exceeds limiter capacity".to_owned())?;
        let Some(burst) = NonZeroU32::new(1) else {
            return Err("burst size must be greater than zero".to_owned());
        };
        let quota = Quota::per_minute(rate).allow_burst(burst);
        self.sources.insert(
            source_id.into(),
            SourceLimits {
                rate: RateLimiter::direct(quota),
                concurrency: Semaphore::new(budget.max_concurrency),
            },
        );
        Ok(())
    }

    /// Return a handle that activates the global kill switch.
    #[must_use]
    pub fn kill_switch(&self) -> KillSwitch {
        self.switch.clone()
    }

    /// Reserve one request slot for a source.
    ///
    /// # Errors
    /// Returns a diagnostic when the source is unregistered or the run is stopped.
    pub async fn acquire(&self, source_id: &str) -> Result<RequestPermit<'_>, String> {
        let limits = self
            .sources
            .get(source_id)
            .ok_or_else(|| format!("unknown source: {source_id}"))?;
        loop {
            if *self.kill_switch.borrow() {
                return Err("run stopped by kill switch".to_owned());
            }
            let mut stopped = self.kill_switch.clone();
            tokio::select! {
                result = stopped.changed() => {
                    if result.is_err() || *stopped.borrow() {
                        return Err("run stopped by kill switch".to_owned());
                    }
                }
                permit = limits.concurrency.acquire() => {
                    let permit = permit.map_err(|error| format!("concurrency slot closed: {error}"))?;
                    if *self.kill_switch.borrow() {
                        return Err("run stopped by kill switch".to_owned());
                    }
                    let mut stopped = self.kill_switch.clone();
                    tokio::select! {
                        result = stopped.changed() => {
                            if result.is_err() || *stopped.borrow() {
                                return Err("run stopped by kill switch".to_owned());
                            }
                        }
                        () = limits.rate.until_ready() => {}
                    }
                    if *self.kill_switch.borrow() {
                        return Err("run stopped by kill switch".to_owned());
                    }
                    return Ok(RequestPermit { _concurrency: permit });
                }
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::time::Duration;
    use std::time::Instant;

    #[tokio::test]
    async fn one_source_does_not_affect_another() {
        let mut governor = Governor::new();
        governor
            .add_source("slow", SourceBudget::new(60, 1).expect("valid slow budget"))
            .expect("slow source registered");
        governor
            .add_source(
                "fast",
                SourceBudget::new(60_000, 2).expect("valid fast budget"),
            )
            .expect("fast source registered");

        let slow_start = Instant::now();
        let _slow = governor.acquire("slow").await.expect("slow permit");
        assert!(
            tokio::time::timeout(Duration::from_millis(50), governor.acquire("slow"))
                .await
                .is_err(),
            "second slow request must wait for its rate slot"
        );

        let fast_start = Instant::now();
        let _fast = governor.acquire("fast").await.expect("fast permit");
        let _fast_second = governor.acquire("fast").await.expect("second fast permit");
        assert!(fast_start.elapsed() < Duration::from_millis(900));
        assert!(slow_start.elapsed() < Duration::from_millis(900));
    }

    #[tokio::test]
    async fn kill_switch_stops_all_sources() {
        let mut governor = Governor::new();
        governor
            .add_source("one", SourceBudget::new(60, 1).expect("valid budget"))
            .expect("source registered");
        let switch = governor.kill_switch();
        switch.activate();
        assert!(
            governor
                .acquire("one")
                .await
                .expect_err("kill switch stops requests")
                .contains("kill switch")
        );
    }

    #[tokio::test]
    async fn kill_switch_interrupts_rate_wait() {
        let mut governor = Governor::new();
        governor
            .add_source("one", SourceBudget::new(60, 1).expect("valid budget"))
            .expect("source registered");
        let _first = governor.acquire("one").await.expect("first permit");
        let switch = governor.kill_switch();
        switch.activate();
        let started = Instant::now();
        let result = governor.acquire("one").await;
        assert!(
            result
                .expect_err("kill switch interrupts waiting")
                .contains("kill switch")
        );
        assert!(started.elapsed() < Duration::from_millis(900));
    }

    #[test]
    fn rejects_zero_budget() {
        assert!(
            SourceBudget::new(0, 1)
                .expect_err("zero rate")
                .contains("greater than zero")
        );
        assert!(
            SourceBudget::new(1, 0)
                .expect_err("zero concurrency")
                .contains("greater than zero")
        );
    }
}
