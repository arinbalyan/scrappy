# security-privacy Specification

## Purpose
Defines how credentials, sensitive fields, network targets, browser automation, and personal data are protected so the scraper can run without leaking secrets or reaching unintended hosts.

## Requirements

### Requirement: Structural secret handling
Credentials SHALL be held in redacting types whose debug output never reveals the value. Secrets SHALL be loaded from environment variables or explicitly selected secret files, never from committed config.

#### Scenario: Debug output masks a secret
- **WHEN** a credential value is formatted for debug output
- **THEN** the output contains only a redaction marker

#### Scenario: Secret is absent from config
- **WHEN** a config file is saved after a run that used credentials
- **THEN** the config file contains no credential values

### Requirement: No secrets in logs, errors, or artifacts
Logs, error messages, fixtures, output files, and test artifacts SHALL NOT contain credential values, authorization headers, cookies, or query-string tokens. A CI check SHALL feed synthetic secrets through a run and fail if they appear anywhere in artifacts.

#### Scenario: Synthetic secret leak fails CI
- **WHEN** a synthetic secret appears in a log or artifact during CI
- **THEN** the leak check fails and identifies the artifact

#### Scenario: URL logging is redacted
- **WHEN** a request URL contains a token in its query string
- **THEN** logged URLs omit the query string or mask the token

### Requirement: SSRF protection with DNS resolution
Every outbound HTTP destination SHALL be validated after DNS resolution. Private, loopback, link-local, and reserved addresses SHALL be blocked, redirects SHALL be revalidated, and rebinding SHALL be mitigated by connecting only to the validated address.

#### Scenario: Literal private address is blocked
- **WHEN** a URL targets 127.0.0.1 or ::1
- **THEN** the request is refused before connection

#### Scenario: Domain resolving to private address is blocked
- **WHEN** a hostname resolves to a private IP
- **THEN** the request is refused before connection

#### Scenario: Redirect to private address is blocked
- **WHEN** a redirect target resolves to a private IP
- **THEN** the redirect is not followed

### Requirement: Browser isolation
Browser automation SHALL be opt-in, use a pinned browser build, pass configured proxy settings, enforce private-address checks, cap response and cookie sizes, validate the rendered result for challenges, and avoid the configured script lookup by path.

#### Scenario: Browser feature disabled
- **WHEN** the default binary is built without the browser feature
- **THEN** no browser runtime dependency is required

#### Scenario: Browser navigation to private address is refused
- **WHEN** a browser source target resolves to a private address
- **THEN** navigation is blocked

#### Scenario: Rendered challenge is detected
- **WHEN** browser rendering produces a challenge page
- **THEN** the source is stopped and a challenge status is recorded

### Requirement: Bounded resource use
Response bodies SHALL have a hard size limit. Memory use SHALL have a hard ceiling rather than a cooperative GC-and-sleep approach. Request and result channels SHALL be bounded.

#### Scenario: Oversized response is rejected
- **WHEN** a response exceeds the configured body limit
- **THEN** reading stops and an oversized-response error is recorded

#### Scenario: Memory ceiling is enforced
- **WHEN** allocation approaches the configured ceiling
- **THEN** the engine stops starting new work rather than continuing optimistically

### Requirement: PII minimization and retention
Fixtures SHALL be scrubbed of personal data before commit. Output containing job-seeker data SHALL have a documented retention policy. Guessed email addresses and historical commit-email harvesting SHALL NOT be enabled by default.

#### Scenario: Fixture contains no personal data
- **WHEN** a fixture is committed
- **THEN** personal names, emails, and phone numbers have been replaced

#### Scenario: Email guessing is disabled by default
- **WHEN** a user runs scraping without enabling experimental email discovery
- **THEN** no guessed email addresses are emitted

### Requirement: Wire protocol secret safety
Credentials, authorization headers, cookies, query-string tokens, and unsanitized challenge samples SHALL NOT appear in stdout records, stderr progress events, resume state, or diagnostic output. The synthetic-secret CI check SHALL assert against all four surfaces.

#### Scenario: Secret absent from bridge surfaces
- **WHEN** a synthetic secret is injected into a stdin run-config and a challenge response
- **THEN** stdout, stderr, progress events, and resume state contain no copy of it

#### Scenario: Challenge evidence is sanitized
- **WHEN** a challenge event is emitted
- **THEN** its evidence excludes cookies, tokens, and query strings

### Requirement: Coordinated emulation profile
User agents and client hints SHALL come from one validated emulation profile selected by the manifest. Every fetcher, including the browser fetcher, SHALL use a single thread-safe proxy selector with health feedback from real request outcomes. Health-check denial SHALL be final for the duration of a run.

#### Scenario: Profile hints are consistent
- **WHEN** a manifest selects an emulation profile
- **THEN** all emitted user-agent and client-hint headers match that profile

#### Scenario: Browser-required source does not downgrade
- **WHEN** the browser feature is unavailable for a browser-required source
- **THEN** the source is skipped with a clear status instead of trying a plain HTTP fallback

### Requirement: Browser dependency resolver
Browser automation SHALL share one dependency resolver between doctor and the engine, enforce a browser concurrency cap, and SHALL NOT silently downgrade a browser-required source when the browser is unavailable.

#### Scenario: Doctor and engine agree on browser availability
- **WHEN** doctor reports the browser backend available
- **THEN** the engine can start a browser-required source up to the configured browser concurrency cap

### Requirement: Challenge inspection before response close
Challenge detection SHALL inspect the response body before retry logic closes it. Challenge evidence SHALL be sanitized and retained for the challenge event. A challenge SHALL NOT be reduced to a generic retryable or permanent failure.

#### Scenario: Challenge body is inspected
- **WHEN** a response contains a known challenge marker
- **THEN** the challenge is classified before the body is closed

#### Scenario: Challenge is not a generic error
- **WHEN** a source is challenged
- **THEN** the run reports a challenge status rather than a plain 403 failure

### Requirement: Explicit opt-in for email discovery
Any email discovery beyond addresses observed in job descriptions SHALL be an explicitly enabled feature with its own source manifest, rate limit, credential declaration, and output policy. Guessed addresses and historical commit-email harvesting SHALL remain disabled by default.

#### Scenario: Discovery is off by default
- **WHEN** a default run processes a job description with no address
- **THEN** no guessed or harvested address is emitted

#### Scenario: Enabled discovery is declared
- **WHEN** a caller enables a discovery feature
- **THEN** that feature has a manifest entry and its output is tagged with provenance
