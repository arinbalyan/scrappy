# Spec Delta

## Purpose

Define retry, identity, concurrency, and robots behavior that keeps public scraping reliable and polite.

## ADDED Requirements

### Requirement: Retry covers transport and retryable statuses
The engine SHALL retry transport failures and HTTP 429 or 5xx responses with bounded exponential backoff and randomized jitter.

#### Scenario: Rate limit is retried
- **WHEN** a source responds with HTTP 429
- **THEN** the engine retries after a jittered delay and honors Retry-After when present

#### Scenario: Client error is not retried
- **WHEN** a source responds with HTTP 404
- **THEN** the engine does not retry the request

### Requirement: Identity policy is explicit
Every request SHALL carry a documented user agent, and optional proxy and cookie configuration SHALL be applied without logging secret values.

#### Scenario: Default identity
- **WHEN** a source is fetched without explicit identity settings
- **THEN** the request includes the configured scrappy user agent

#### Scenario: Proxy configured
- **WHEN** a proxy is configured for a source
- **THEN** the request uses that proxy and diagnostics redact its credentials

### Requirement: Concurrency limits are observable
The engine SHALL enforce global and per-source concurrency limits and SHALL NOT hold a permit for an idle pagination loop.

#### Scenario: Per-source cap
- **WHEN** a source has max_concurrency 1
- **THEN** the engine never has more than one in-flight request for that source

#### Scenario: Global cap
- **WHEN** the global concurrency cap is reached
- **THEN** additional sources wait instead of exceeding the cap

### Requirement: Robots policy is interpreted
A manifest robots value beginning with `allow` SHALL permit fetching, and a value beginning with `disallow` SHALL skip the source with a policy outcome.

#### Scenario: Descriptive allow value
- **WHEN** a manifest declares `allow public job boards`
- **THEN** the engine fetches the source

#### Scenario: Disallow value
- **WHEN** a manifest declares `disallow automated access`
- **THEN** the engine skips the source and records a policy outcome

### Requirement: Cancellation stops waits
The engine SHALL abandon retry and pagination waits when the run is cancelled.

#### Scenario: Cancel during backoff
- **WHEN** cancellation arrives while a retry is waiting
- **THEN** the source stops and the run exits with the cancellation code
