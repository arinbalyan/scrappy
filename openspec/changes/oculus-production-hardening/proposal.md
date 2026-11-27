# Proposal

## Why

The registry now has 111 manifests, but the live probe leaves 57 sources outside the working set. Nineteen need credentials, eleven are blocked, nine are stale, ten lack seeds, five fail from the environment, and three hit challenges. The engine also fetches one page per source, retries only transport errors, cannot rotate identity, and drops useful job fields. The next step is to close those gaps in a planned order so coverage and reliability improve without hiding dead sources or weakening the fixture contract.

This is planning only. The branch remains a clean-room rebuild with inherited files removed, and no implementation or `.gitignore` change happens during this propose workflow.

## What Changes

- Execute declared manifest pagination for page, offset, cursor, and bounded fanout strategies, with stop reasons, request budgets, partial-failure handling, and parity between the engine and the live harvest. **BREAKING** Manifest version 2 is required only for structured pagination tables and composite tenant seeds. Legacy version 1 manifests remain valid.
- Harden fetching with random jitter, Retry-After handling, retryable status classification, client reuse, explicit user-agent policy, optional proxy and cookie support, and honest concurrency limits.
- Wire credentials from environment and config into runtime substitutions so keyed sources can run without writing secrets to config or output.
- Complete normalized job data with posted timestamps, salary, apply URLs, company domains, source-native identifiers, and merged source attribution. **BREAKING** Output schema moves to version 2, and readers continue to accept version 1.
- Repair or retire sources against a documented viability matrix. Working health labels must match observed behavior, and dead sources must stop being reported as working. **BREAKING** Sources with two dated dead observations are removed from the registry with their fixtures and behavior docs.
- Expand live verification to cover every source, follow pagination, record latency percentiles and per-family totals, and write counts-only reports.
- Correct challenge false positives by restricting body-marker scans to HTML or WAF evidence, so JSON job content is not misclassified as a CAPTCHA.
- Keep all external access polite. No CAPTCHA solving, WAF bypass, or credential-based access to protected boards.

## Capabilities

### New Capabilities

- `scrape-pagination`: manifest-driven page, offset, cursor, and fanout execution with stop conditions, request budgets, partial failures, and live-harvest parity.
- `fetch-resilience`: bounded retries, jitter, Retry-After, retryable statuses, client reuse, identity policy, proxy and cookie support, concurrency, and robots handling.
- `credential-flow`: credential resolution from environment and config into runtime template values, with redaction and doctor reporting.
- `record-completeness`: normalized records retain posted dates, compensation, apply routes, company metadata, source-native identifiers, and source attribution.
- `source-coverage`: per-source viability classification, seed and route repair, health-label accuracy, and explicit retirement of dead sources.
- `live-verification`: full-registry live probes with pagination, counts-only reports, latency statistics, and per-family and per-outcome summaries.

### Modified Capabilities

None. The project has no synced main specs yet, so this change introduces capability specs and leaves `oculus-foundation` history intact.

## Impact

- `src/engine/` for pagination, retry, concurrency, and run summaries.
- `src/sources/` for manifest schema additions, validation, and adapter options.
- `src/main.rs` and `src/config/` for credential plumbing and run wiring.
- `src/output/record.rs` and `src/parsing/` for record completion.
- All manifests, fixtures, and per-source documentation affected by seed or route changes.
- `tests/live/` for full pagination, timing, and statistics reporting.
- `documentation/` for runbooks, source behavior records, and generated status.

Observed old behavior is limited to pagination loops, status retries, concurrency semaphores, and identity rotation seen in the read-only `main` branch. The new design is independently written and uses the manifest schema as its contract. The new branch remains MIT-licensed. No third-party implementation is copied or relicensed.
