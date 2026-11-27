# Spec Delta

## Purpose

Defines how sources are fetched, parsed, throttled, retried, and resumed so a long-running scrape is polite to sites, resilient to failures, and reproducible in tests.

## ADDED Requirements

### Requirement: Fetcher abstraction
Fetching SHALL be behind an abstraction with at least three implementations: plain HTTP, browser-emulation HTTP, and optional browser rendering. Parsing SHALL consume fetched content without depending on the fetcher implementation.

#### Scenario: Parser runs against a fixture
- **WHEN** a parser test supplies a saved HTTP response
- **THEN** parsing succeeds without a network connection

#### Scenario: Fetcher choice comes from the manifest
- **WHEN** a source declares a fetcher kind
- **THEN** the engine uses that fetcher for the source

### Requirement: Per-source rate limits and concurrency caps
Each source SHALL have an independent token-bucket rate limiter and a separate concurrency semaphore. A source SHALL NOT be starved by another source's traffic.

#### Scenario: One source does not affect another
- **WHEN** one source is throttled to a low rate and another is configured higher
- **THEN** each source follows its own configured rate

#### Scenario: Global kill switch stops all fetching
- **WHEN** a user activates the global kill switch
- **THEN** no further requests are sent to any source

### Requirement: Bounded retries with jitter
Retries SHALL apply only to idempotent requests and retryable conditions. Attempts SHALL be bounded, use jittered backoff, honor cancellation promptly, and respect Retry-After when present.

#### Scenario: Retry-After is honored
- **WHEN** a source responds with a Retry-After header
- **THEN** the next attempt waits at least that interval

#### Scenario: Cancellation interrupts backoff
- **WHEN** a run is cancelled during backoff
- **THEN** the process stops waiting without completing further attempts

#### Scenario: Permanent error is not retried
- **WHEN** a request fails with a non-retryable status or certificate error
- **THEN** no retry is attempted

### Requirement: Challenge detection and circuit breaking
The engine SHALL detect WAF and bot challenges, stop the affected source, record a sanitized sample, apply backoff, and surface an actionable error. Repeated failures SHALL trip a per-source circuit breaker. CAPTCHA solving and credential-based access SHALL NOT be used.

#### Scenario: Challenge stops the source
- **WHEN** a response contains a known challenge marker
- **THEN** the source is stopped and a challenge status is recorded

#### Scenario: Circuit breaker trips
- **WHEN** a source reaches its failure threshold
- **THEN** subsequent requests to that source are skipped until the cooldown expires

### Requirement: Canonicalization and content hashing
Each emitted record SHALL carry a canonical URL and a content hash computed from stable fields. Deduplication SHALL use those values and SHALL be idempotent across reruns.
A configured JSON parser SHALL derive a canonical URL from a named field path and URL prefix when the source omits a direct link. Derived URLs SHALL be deterministic, use https, and never use opaque tracking tokens.

#### Scenario: Same content produces the same hash
- **WHEN** the same job is fetched twice with unchanged fields
- **THEN** both records have the same content hash

#### Scenario: Deduplication can be disabled
- **WHEN** a user disables deduplication
- **THEN** duplicate records are emitted as separate results

#### Scenario: Derive a canonical URL from an id
    - **WHEN** a JSON manifest declares a `url_from` field path and a `url_prefix` starting with https
    - **THEN** each record that lacks a direct link uses the prefix plus the field value as its canonical URL

#### Scenario: Reject an incomplete URL derivation
    - **WHEN** a JSON manifest declares `url_from` without `url_prefix`
    - **THEN** parser construction fails with a diagnostic naming the missing prefix

### Requirement: Checkpoint and resume
An interrupted run SHALL persist progress so a resumed run skips completed sources and already-emitted records. The checkpoint SHALL be versioned and safe to reuse only with a matching configuration.

#### Scenario: Resume skips completed work
- **WHEN** a run is resumed from a matching checkpoint
- **THEN** completed sources and emitted records are not fetched again

#### Scenario: Mismatched checkpoint is rejected
- **WHEN** a checkpoint was written with a different configuration version
- **THEN** resume fails with a diagnostic instead of reusing it

### Requirement: Result limits and failure policy
Results limits SHALL apply uniformly across global and per-source settings. Unknown sources and invalid inputs SHALL fail the run. A configured fail-open policy MAY continue after source failure but SHALL record the failure in the run summary.

#### Scenario: Global limit applies across sources
- **WHEN** several sources each have high per-source caps and the global cap is lower
- **THEN** the run emits no more than the global cap

#### Scenario: Fail-open records the failure
- **WHEN** a source fails and fail-open is enabled
- **THEN** the run continues and the source failure appears in the summary

### Requirement: Enrichment before emission
Records SHALL be fully enriched, scored, deduplicated, and attributed before they are emitted. Enrichment SHALL preserve contact provenance from extraction, verification, and company-page discovery. No preliminary record SHALL be emitted before these steps complete.

#### Scenario: Streamed record is final
- **WHEN** a caller reads a record line from stdout
- **THEN** the record contains final email, quality, attribution, and dedup fields

#### Scenario: Provenance survives enrichment
- **WHEN** an address is discovered on a company page
- **THEN** its source and provenance in the emitted record name the company page

### Requirement: Challenge attention signal
A detected challenge SHALL emit a machine-readable challenge event containing source_id, challenge kind, opaque resume state bound to the configuration fingerprint, and sanitized evidence. Only the challenged source SHALL pause; other sources SHALL continue. Challenge solving and credential-based access SHALL NOT be used.

#### Scenario: One challenged source pauses
- **WHEN** one source returns a challenge marker
- **THEN** a challenge event is emitted and other sources continue fetching

#### Scenario: Resume state is opaque and bound
- **WHEN** a challenge event is emitted
- **THEN** its resume state cannot be reused with a different configuration fingerprint

### Requirement: Result cap precedence
The effective result cap SHALL be the minimum of the global cap and the applicable per-source cap. The resolved cap SHALL be recorded in the terminal summary. A per-source cap SHALL NOT raise the global cap.

#### Scenario: Per-source cap cannot exceed global cap
- **WHEN** a per-source cap is larger than the global cap
- **THEN** that source emits no more than the global cap

#### Scenario: Resolved cap is visible
- **WHEN** a run finishes
- **THEN** the terminal event records the effective cap for each source

### Requirement: Runtime configuration handoff
The run engine SHALL accept a validated run-config document from the CLI as its single source of runtime settings. Explicit values in that document SHALL take precedence over file and environment layers. Rate limits, concurrency caps, result caps, filters, dedup toggles, quality floor, verification settings, and resume state SHALL come from the resolved run-config.

#### Scenario: Run-config values drive the engine
- **WHEN** a stdin run-config sets a per-source result cap and dedup to false
- **THEN** the engine applies both values without consulting a lower-precedence layer

#### Scenario: Invalid run-config stops the engine
- **WHEN** run-config validation fails
- **THEN** no fetcher starts and no records are emitted
