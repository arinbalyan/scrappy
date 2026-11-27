// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Arin Balyan
// Last modified: 2026-11-27

//! Bounded, jittered retries with Retry-After support.
//!
//! See documentation/006-architecture.md.

// Rust guideline compliant 2026-10-09

use std::time::{Duration, SystemTime, UNIX_EPOCH};

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
    /// Throttled or retryable response carrying an optional Retry-After value.
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

    fn fixed_clock() -> SystemTime {
        UNIX_EPOCH + Duration::from_secs(1_700_000_000)
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

    #[test]
    fn equal_jitter_is_deterministic_and_bounded() {
        let delay = Duration::from_millis(100);
        assert_eq!(equal_jitter(delay, || 0), Duration::from_millis(50));
        assert_eq!(
            equal_jitter(delay, || 50_000_000),
            Duration::from_millis(100)
        );
        let middle = equal_jitter(delay, || 12_345_678);
        assert!(middle >= Duration::from_millis(50));
        assert!(middle <= delay);
        assert_eq!(equal_jitter(Duration::ZERO, || 7), Duration::ZERO);
    }

    #[test]
    fn backoff_uses_equal_jitter_with_fixed_entropy() {
        let backoff_policy = RetryPolicy {
            max_retries: 2,
            base_delay: Duration::from_millis(100),
            max_delay: MAX_DELAY,
        };
        assert_eq!(
            backoff(backoff_policy, 0, &mut || 0),
            Duration::from_millis(50)
        );
        assert_eq!(
            backoff(backoff_policy, 0, &mut || 50_000_000),
            Duration::from_millis(100)
        );
        assert_eq!(backoff(backoff_policy, 10, &mut || 0), MAX_DELAY);
    }

    #[test]
    fn statuses_classify_by_retry_policy() {
        for status in [408, 425, 429] {
            assert_eq!(classify_status(status), Some(Failure::Throttled(None)));
        }
        for status in [500, 503, 599] {
            assert_eq!(classify_status(status), Some(Failure::Retryable));
        }
        for status in [400, 403, 404, 418, 499] {
            assert_eq!(classify_status(status), Some(Failure::Permanent));
        }
        for status in [200, 204, 301, 304] {
            assert_eq!(classify_status(status), None);
        }
    }

    #[test]
    fn retry_methods_default_and_post_opt_in() {
        for method in ["GET", "head", " Options "] {
            assert!(method_retries_by_default(method));
            assert!(method_allows_retry(method, false));
        }
        assert!(!method_retries_by_default("POST"));
        assert!(!method_allows_retry("POST", false));
        assert!(method_allows_retry("post", true));
        assert!(!method_allows_retry("PUT", true));
    }

    #[test]
    fn parses_retry_after_imf_fixdate_with_fixed_clock() {
        let now = fixed_clock();
        assert_eq!(
            parse_retry_after_at("Tue, 14 Nov 2023 22:13:27 GMT", now).expect("future date"),
            Duration::from_secs(7)
        );
        assert_eq!(
            parse_retry_after_at("Tue, 14 Nov 2023 22:13:00 GMT", now).expect("past date"),
            Duration::ZERO
        );
    }

    #[test]
    fn rejects_unsupported_retry_after_values() {
        let now = fixed_clock();
        for value in [
            "tomorrow",
            "Sun, 31 Feb 2023 00:00:00 GMT",
            "Tue, 14 Nov 2023 22:13:27 UTC",
        ] {
            let _ = parse_retry_after_at(value, now).expect_err("malformed Retry-After");
        }
    }

    #[test]
    fn retry_after_forms_clamp_to_policy_maximum() {
        let now = fixed_clock();
        let seconds = parse_retry_after_at("120", now).expect("seconds");
        let date = parse_retry_after_at("Tue, 14 Nov 2023 22:15:00 GMT", now).expect("date");
        assert_eq!(bounded(seconds, MAX_DELAY), MAX_DELAY);
        assert_eq!(bounded(date, MAX_DELAY), MAX_DELAY);
        let short = parse_retry_after_at("Tue, 14 Nov 2023 22:13:27 GMT", now).expect("short");
        assert_eq!(bounded(short, MAX_DELAY), Duration::from_secs(7));
    }

    #[tokio::test]
    async fn async_retry_accepts_injected_jitter() {
        let attempts = Arc::new(AtomicUsize::new(0));
        let counter = Arc::clone(&attempts);
        let token = CancelToken::new();
        let mut cancelled = token.receiver();
        let result: Result<u8, (RetryOutcome, &str)> = retry_async_with_jitter(
            policy(1),
            &mut cancelled,
            || {
                let attempt = counter.fetch_add(1, Ordering::Relaxed);
                async move {
                    if attempt == 0 {
                        Err("retryable")
                    } else {
                        Ok(9)
                    }
                }
            },
            |_| Failure::Retryable,
            || 0,
        )
        .await;
        assert_eq!(result.expect("retry succeeds"), 9);
        assert_eq!(attempts.load(Ordering::Relaxed), 2);
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

/// Add equal jitter to a backoff delay.
///
/// The result lies between half the delay and the full delay. The closure
/// supplies entropy and is called once per delay.
#[must_use]
pub fn equal_jitter(delay: Duration, mut entropy: impl FnMut() -> u64) -> Duration {
    let half = delay / 2;
    let span = half.as_nanos().saturating_add(1);
    let offset = u128::from(entropy()) % span;
    let offset = u64::try_from(offset).unwrap_or(u64::MAX);
    half.saturating_add(Duration::from_nanos(offset))
}

/// Classify an HTTP status for retry handling.
///
/// Returns `None` for successful, redirect, and unrecognized statuses.
#[must_use]
pub fn classify_status(status: u16) -> Option<Failure> {
    match status {
        // These statuses may carry Retry-After instructions.
        408 | 425 | 429 => Some(Failure::Throttled(None)),
        500..=599 => Some(Failure::Retryable),
        400..=499 => Some(Failure::Permanent),
        _ => None,
    }
}

/// Return whether a method retries without explicit opt-in.
#[must_use]
pub fn method_retries_by_default(method: &str) -> bool {
    let method = method.trim();
    method.eq_ignore_ascii_case("GET")
        || method.eq_ignore_ascii_case("HEAD")
        || method.eq_ignore_ascii_case("OPTIONS")
}

/// Return whether a method may retry with POST opt-in.
///
/// GET, HEAD, and OPTIONS always retry. POST retries only when `retry_post`
/// is true.
#[must_use]
pub fn method_allows_retry(method: &str, retry_post: bool) -> bool {
    method_retries_by_default(method) || (retry_post && method.trim().eq_ignore_ascii_case("POST"))
}

/// Parse a Retry-After header as seconds or an IMF-fixdate.
///
/// # Errors
/// Returns a diagnostic when the value is malformed.
pub fn parse_retry_after(value: &str) -> Result<Duration, String> {
    parse_retry_after_at(value, SystemTime::now())
}

/// Parse Retry-After against a caller-supplied clock.
///
/// HTTP dates in the past produce a zero delay.
///
/// # Errors
/// Returns a diagnostic when the value is malformed.
pub fn parse_retry_after_at(value: &str, now: SystemTime) -> Result<Duration, String> {
    let value = value.trim();
    if let Ok(seconds) = value.parse::<u64>() {
        return Ok(Duration::from_secs(seconds));
    }
    let deadline = parse_imf_fixdate(value)
        .ok_or_else(|| format!("unsupported Retry-After value: {value}"))?;
    Ok(deadline.duration_since(now).unwrap_or(Duration::ZERO))
}

fn parse_imf_fixdate(value: &str) -> Option<SystemTime> {
    let fields: Vec<&str> = value.split_whitespace().collect();
    if fields.len() != 6 {
        return None;
    }
    let weekday = fields[0].strip_suffix(',')?;
    if weekday.len() != 3 || !weekday.bytes().all(|byte| byte.is_ascii_alphabetic()) {
        return None;
    }
    let day = parse_two_digits(fields[1])?;
    let month = month_number(fields[2])?;
    let year = fields[3];
    if year.len() != 4 || !year.bytes().all(|byte| byte.is_ascii_digit()) {
        return None;
    }
    let year = year.parse::<i64>().ok()?;
    let mut clock = fields[4].split(':');
    let hour = parse_two_digits(clock.next()?)?;
    let minute = parse_two_digits(clock.next()?)?;
    let second = parse_two_digits(clock.next()?)?;
    if clock.next().is_some() || !fields[5].eq_ignore_ascii_case("GMT") {
        return None;
    }
    if day == 0 || day > days_in_month(year, month) || hour > 23 || minute > 59 || second > 59 {
        return None;
    }
    let days = days_from_civil(year, month, day);
    let seconds =
        days * 86_400 + i64::from(hour) * 3_600 + i64::from(minute) * 60 + i64::from(second);
    if seconds >= 0 {
        UNIX_EPOCH.checked_add(Duration::from_secs(u64::try_from(seconds).ok()?))
    } else {
        UNIX_EPOCH.checked_sub(Duration::from_secs(u64::try_from(-seconds).ok()?))
    }
}

fn parse_two_digits(value: &str) -> Option<u32> {
    if value.len() != 2 || !value.bytes().all(|byte| byte.is_ascii_digit()) {
        return None;
    }
    value.parse().ok()
}

fn month_number(month: &str) -> Option<u32> {
    match month {
        "Jan" => Some(1),
        "Feb" => Some(2),
        "Mar" => Some(3),
        "Apr" => Some(4),
        "May" => Some(5),
        "Jun" => Some(6),
        "Jul" => Some(7),
        "Aug" => Some(8),
        "Sep" => Some(9),
        "Oct" => Some(10),
        "Nov" => Some(11),
        "Dec" => Some(12),
        _ => None,
    }
}

fn days_in_month(year: i64, month: u32) -> u32 {
    match month {
        1 | 3 | 5 | 7 | 8 | 10 | 12 => 31,
        4 | 6 | 9 | 11 => 30,
        2 if is_leap_year(year) => 29,
        _ => 28,
    }
}

fn is_leap_year(year: i64) -> bool {
    year.rem_euclid(4) == 0 && (year.rem_euclid(100) != 0 || year.rem_euclid(400) == 0)
}

fn days_from_civil(year: i64, month: u32, day: u32) -> i64 {
    let adjusted_year = if month <= 2 { year - 1 } else { year };
    let era = if adjusted_year >= 0 {
        adjusted_year
    } else {
        adjusted_year - 399
    } / 400;
    let year_of_era = adjusted_year - era * 400;
    let shifted_month = i64::from(month) + 9;
    let day_of_year = (153 * (shifted_month % 12) + 2) / 5 + i64::from(day) - 1;
    let day_of_era = year_of_era * 365 + year_of_era / 4 - year_of_era / 100 + day_of_year;
    era * 146_097 + day_of_era - 719_468
}

fn bounded(delay: Duration, max: Duration) -> Duration {
    delay.min(max)
}

fn backoff(policy: RetryPolicy, attempt: usize, entropy: &mut impl FnMut() -> u64) -> Duration {
    // Cap the shift so the duration multiplication cannot overflow.
    let shift = u32::try_from(attempt).unwrap_or(u32::MAX).min(31);
    let scaled = policy.base_delay.saturating_mul(1_u32 << shift);
    bounded(equal_jitter(scaled, entropy), policy.max_delay)
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
                    _ => backoff(policy, attempt, &mut jitter),
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

fn system_entropy() -> impl FnMut() -> u64 {
    use std::collections::hash_map::RandomState;
    use std::hash::{BuildHasher, Hasher};

    let state = RandomState::new();
    let mut counter = 0_u64;
    move || {
        counter = counter.wrapping_add(1);
        let mut hasher = state.build_hasher();
        hasher.write_u64(counter);
        hasher.finish()
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
    operation: F,
    classify: C,
) -> Result<T, (RetryOutcome, E)>
where
    F: FnMut() -> Fut,
    Fut: std::future::Future<Output = Result<T, E>>,
    C: Fn(&E) -> Failure,
{
    retry_async_with_jitter(policy, cancelled, operation, classify, system_entropy()).await
}

/// Run an asynchronous operation with injected jitter entropy.
///
/// # Errors
/// Returns the last operation error with `RetryOutcome::Failed` when attempts
/// are exhausted, and with `RetryOutcome::Cancelled` when cancellation occurs.
pub async fn retry_async_with_jitter<F, Fut, T, E, C, J>(
    policy: RetryPolicy,
    cancelled: &mut watch::Receiver<bool>,
    mut operation: F,
    classify: C,
    mut entropy: J,
) -> Result<T, (RetryOutcome, E)>
where
    F: FnMut() -> Fut,
    Fut: std::future::Future<Output = Result<T, E>>,
    C: Fn(&E) -> Failure,
    J: FnMut() -> u64,
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
                    _ => backoff(policy, attempt, &mut entropy),
                };
                if wait(delay, cancelled).await == RetryOutcome::Cancelled {
                    return Err((RetryOutcome::Cancelled, error));
                }
                attempt += 1;
            }
        }
    }
}
