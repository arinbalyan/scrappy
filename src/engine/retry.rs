//! Bounded, jittered retries with Retry-After support.
//!
//! See documentation/006-architecture.md.

// Rust guideline compliant 2026-10-09

use std::time::Duration;

use tokio::sync::watch;

/// Maximum number of attempts after the first request.
pub const DEFAULT_MAX_RETRIES: usize = 3;
/// Base delay used for the first backoff interval.
pub const DEFAULT_BASE_DELAY: Duration = Duration::from_millis(200);
/// Maximum backoff or Retry-After delay honored by the engine.
pub const MAX_DELAY: Duration = Duration::from_secs(30);

/// Classification of a request failure.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Failure {
    /// Retryable server error or transport failure.
    Retryable,
    /// Permanent error that must not be retried.
    Permanent,
    /// Rate limit response carrying an optional Retry-After value.
    Throttled(Option<Duration>),
}

/// Token used to cancel a retry sequence.
#[derive(Debug, Clone)]
pub struct CancelToken {
    sender: watch::Sender<bool>,
}

impl CancelToken {
    /// Create an inactive cancellation token.
    #[must_use]
    pub fn new() -> Self {
        let (sender, _) = watch::channel(false);
        Self { sender }
    }

    /// Cancel all waiters holding a receiver from this token.
    pub fn cancel(&self) {
        let _ = self.sender.send(true);
    }

    /// Subscribe to cancellation.
    #[must_use]
    pub fn receiver(&self) -> watch::Receiver<bool> {
        self.sender.subscribe()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::sync::Arc;
    use std::sync::atomic::{AtomicUsize, Ordering};
    use std::time::Instant;

    fn policy(max_retries: usize) -> RetryPolicy {
        RetryPolicy {
            max_retries,
            base_delay: Duration::from_millis(1),
            max_delay: MAX_DELAY,
        }
    }

    #[tokio::test]
    async fn retries_are_bounded() {
        let attempts = Arc::new(AtomicUsize::new(0));
        let counter = Arc::clone(&attempts);
        let token = CancelToken::new();
        let mut cancelled = token.receiver();
        let result: Result<(), (RetryOutcome, &str)> = retry(
            policy(2),
            &mut cancelled,
            || {
                counter.fetch_add(1, Ordering::Relaxed);
                Err("retryable")
            },
            |_| Failure::Retryable,
            || 0,
        )
        .await;
        assert_eq!(
            result.expect_err("all attempts fail").0,
            RetryOutcome::Failed
        );
        assert_eq!(attempts.load(Ordering::Relaxed), 3);
    }

    #[tokio::test]
    async fn permanent_failure_is_not_retried() {
        let attempts = Arc::new(AtomicUsize::new(0));
        let counter = Arc::clone(&attempts);
        let token = CancelToken::new();
        let mut cancelled = token.receiver();
        let result: Result<(), (RetryOutcome, &str)> = retry(
            policy(3),
            &mut cancelled,
            || {
                counter.fetch_add(1, Ordering::Relaxed);
                Err("permanent")
            },
            |_| Failure::Permanent,
            || 0,
        )
        .await;
        assert_eq!(
            result.expect_err("permanent failure").0,
            RetryOutcome::Failed
        );
        assert_eq!(attempts.load(Ordering::Relaxed), 1);
    }

    #[tokio::test]
    async fn retry_after_is_honored() {
        let token = CancelToken::new();
        let mut cancelled = token.receiver();
        let started = Instant::now();
        let attempts = Arc::new(AtomicUsize::new(0));
        let counter = Arc::clone(&attempts);
        let result: Result<u8, (RetryOutcome, &str)> = retry(
            policy(1),
            &mut cancelled,
            || {
                if counter.fetch_add(1, Ordering::Relaxed) == 0 {
                    Err("throttled")
                } else {
                    Ok(7)
                }
            },
            |_| Failure::Throttled(Some(Duration::from_millis(30))),
            || 0,
        )
        .await;
        assert_eq!(result.expect("retry succeeds"), 7);
        assert!(started.elapsed() >= Duration::from_millis(25));
    }

    #[tokio::test]
    async fn cancel_interrupts_oversized_retry_after() {
        let token = CancelToken::new();
        let sleeping = token.clone();
        let task = tokio::spawn(async move {
            let mut receiver = sleeping.receiver();
            retry(
                policy(1),
                &mut receiver,
                || Err::<(), &str>("throttled"),
                |_| Failure::Throttled(Some(Duration::from_secs(3600))),
                || 0,
            )
            .await
        });
        tokio::time::sleep(Duration::from_millis(10)).await;
        token.cancel();
        let started = Instant::now();
        let result = tokio::time::timeout(Duration::from_millis(500), task)
            .await
            .expect("cancellation completes promptly")
            .expect("task joins");
        assert_eq!(result.expect_err("cancelled").0, RetryOutcome::Cancelled);
        assert!(started.elapsed() < Duration::from_millis(500));
    }

    #[test]
    fn parses_retry_after_seconds() {
        assert_eq!(
            parse_retry_after(" 12 ").expect("seconds"),
            Duration::from_secs(12)
        );
        assert!(
            parse_retry_after("tomorrow")
                .expect_err("invalid delay")
                .contains("unsupported")
        );
    }
}

impl Default for CancelToken {
    fn default() -> Self {
        Self::new()
    }
}

/// Configuration for a bounded retry sequence.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct RetryPolicy {
    /// Number of attempts after the first request.
    pub max_retries: usize,
    /// Base delay for exponential backoff.
    pub base_delay: Duration,
    /// Upper bound for each wait.
    pub max_delay: Duration,
}

impl Default for RetryPolicy {
    fn default() -> Self {
        Self {
            max_retries: DEFAULT_MAX_RETRIES,
            base_delay: DEFAULT_BASE_DELAY,
            max_delay: MAX_DELAY,
        }
    }
}

/// Outcome of a retry sequence.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum RetryOutcome {
    /// The operation succeeded.
    Succeeded,
    /// All attempts were used or the failure was permanent.
    Failed,
    /// The token was cancelled while waiting.
    Cancelled,
}

/// Parse a Retry-After header as a delay in seconds.
///
/// # Errors
/// Returns a diagnostic when the value is malformed.
pub fn parse_retry_after(value: &str) -> Result<Duration, String> {
    let value = value.trim();
    if let Ok(seconds) = value.parse::<u64>() {
        return Ok(Duration::from_secs(seconds));
    }
    Err(format!("unsupported Retry-After value: {value}"))
}

fn bounded(delay: Duration, max: Duration) -> Duration {
    delay.min(max)
}

fn backoff(policy: RetryPolicy, attempt: usize, jitter: u64) -> Duration {
    let shift = u32::try_from(attempt).unwrap_or(u32::MAX).min(31);
    let scaled = policy.base_delay.saturating_mul(1_u32 << shift);
    let extra = Duration::from_millis(jitter % 100);
    bounded(scaled.saturating_add(extra), policy.max_delay)
}

async fn wait(delay: Duration, cancelled: &mut watch::Receiver<bool>) -> RetryOutcome {
    if *cancelled.borrow() {
        return RetryOutcome::Cancelled;
    }
    let mut changed = cancelled.clone();
    tokio::select! {
        result = changed.changed() => {
            if result.is_err() || *changed.borrow() {
                RetryOutcome::Cancelled
            } else {
                RetryOutcome::Succeeded
            }
        }
        () = tokio::time::sleep(delay) => {
            if *cancelled.borrow() {
                RetryOutcome::Cancelled
            } else {
                RetryOutcome::Succeeded
            }
        }
    }
}

/// Run an idempotent operation with bounded, cancellation-aware retries.
///
/// `operation` returns the failure classification for each attempt. A closure
/// supplies jitter to keep call sites testable.
///
/// # Errors
/// Returns the last operation error with `RetryOutcome::Failed` when attempts
/// are exhausted or the error is permanent, and with `RetryOutcome::Cancelled`
/// when the token is cancelled during backoff.
pub async fn retry<F, T, E, C, J>(
    policy: RetryPolicy,
    cancelled: &mut watch::Receiver<bool>,
    mut operation: F,
    classify: C,
    mut jitter: J,
) -> Result<T, (RetryOutcome, E)>
where
    F: FnMut() -> Result<T, E>,
    C: Fn(&E) -> Failure,
    J: FnMut() -> u64,
{
    let mut attempt = 0;
    loop {
        match operation() {
            Ok(value) => return Ok(value),
            Err(error) => {
                let failure = classify(&error);
                if failure == Failure::Permanent || attempt >= policy.max_retries {
                    return Err((RetryOutcome::Failed, error));
                }
                let delay = match failure {
                    Failure::Throttled(Some(retry_after)) => bounded(retry_after, policy.max_delay),
                    _ => backoff(policy, attempt, jitter()),
                };
                let outcome = wait(delay, cancelled).await;
                if outcome == RetryOutcome::Cancelled {
                    return Err((outcome, error));
                }
                attempt += 1;
            }
        }
    }
}

/// Run an asynchronous operation with bounded, cancellation-aware retries.
///
/// # Errors
/// Returns the last operation error with `RetryOutcome::Failed` when attempts
/// are exhausted, and with `RetryOutcome::Cancelled` when cancellation occurs.
pub async fn retry_async<F, Fut, T, E, C>(
    policy: RetryPolicy,
    cancelled: &mut watch::Receiver<bool>,
    mut operation: F,
    classify: C,
) -> Result<T, (RetryOutcome, E)>
where
    F: FnMut() -> Fut,
    Fut: std::future::Future<Output = Result<T, E>>,
    C: Fn(&E) -> Failure,
{
    let mut attempt = 0;
    loop {
        match operation().await {
            Ok(value) => return Ok(value),
            Err(error) => {
                let failure = classify(&error);
                if failure == Failure::Permanent || attempt >= policy.max_retries {
                    return Err((RetryOutcome::Failed, error));
                }
                let delay = match failure {
                    Failure::Throttled(Some(retry_after)) => bounded(retry_after, policy.max_delay),
                    _ => backoff(policy, attempt, attempt as u64),
                };
                if wait(delay, cancelled).await == RetryOutcome::Cancelled {
                    return Err((RetryOutcome::Cancelled, error));
                }
                attempt += 1;
            }
        }
    }
}
