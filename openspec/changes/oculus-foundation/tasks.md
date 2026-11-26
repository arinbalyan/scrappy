# Tasks

## 1. Repository scaffolding and licensing

- [ ] 1.1 Create a minimal Cargo package with src/main.rs and src/lib.rs and verify that cargo build --locked succeeds
- [ ] 1.2 Create .gitignore covering target, local config, and secret files and verify no generated artifacts appear in git status
- [ ] 1.3 Write LICENSE in MIT form and NOTICE documenting clean-room provenance, the read-only references, and third-party license obligations, and verify both files are present
- [ ] 1.4 Add Cargo.toml with explicit feature flags for emulation and browser and verify that cargo tree shows no native-tls or openssl in the default build

## 2. CLI and configuration foundation

- [ ] 2.1 Implement clap subcommands for run, validate, config show, config set, list-sources, and doctor and verify that help lists every subcommand and flag
- [ ] 2.2 Implement typed Config with Option fields and layered resolution in CLI, environment, project config, user config, default order, and verify precedence with a table-driven test
- [ ] 2.3 Add OneOrMany deserialization for scalar and array config values and verify both forms load in tests
- [ ] 2.4 Add tri-state boolean flags with negative counterparts and verify that explicit false overrides config true
- [ ] 2.5 Add strict validation for unknown keys, unknown sources, dates, memory limits, and rate maps, and verify invalid input exits nonzero with a stderr diagnostic
- [ ] 2.6 Implement machine-separated stdout and stderr and verify that warnings do not corrupt JSON output
- [ ] 2.7 Implement atomic config set that preserves unknown and per-source sections with 0700 directories and 0600 files, and verify with a tempdir test
- [ ] 2.8 Implement doctor using the runtime config schema with redacted key reporting, and verify it lists the same files the runtime loads
- [ ] 2.9 Implement run --stdin-config with one JSON document, EOF start, and full run-config key validation, and verify malformed input exits 2 with no fetch
- [ ] 2.10 Implement structured NDJSON progress events on stderr with run id, source id, counters, terminal event, and published exit codes 0 through 5, and verify a consumer can parse every event line
- [ ] 2.11 Add actionable validation for enums, ranges, unsupported filters, and preflight diagnostics, and verify each invalid case exits 2 before any request

## 3. Source registry and manifests

- [ ] 3.1 Define the versioned source manifest schema and registry loader and verify duplicate IDs and unknown fields fail validation
- [ ] 3.2 Add credential requirement declarations and verify that a missing key produces a missing-credentials status with no request
- [ ] 3.3 Add robots policy, attribution, and fixture set fields and verify validation rejects missing or empty values
- [ ] 3.4 Implement list-sources health states and verify counts match registered manifests
- [ ] 3.5 Generate status documentation from the registry and verify generated counts match list-sources output
- [ ] 3.6 Add list-sources --format json with health, failure, credential, fetcher, slug, and fixture fields, and verify empty and blocked states are distinguishable
- [ ] 3.7 Add registry parity checks for default selection, fixtures, contract tests, status uniqueness, and slug counts, and verify drift fails CI offline

## 4. Fetching and parsing foundation

- [ ] 4.1 Define the Fetcher trait with validated request and bounded response types and verify a mock fetcher can drive a parser test
- [ ] 4.2 Implement plain HTTP fetcher with reqwest rustls and verify it works against a local test server
- [ ] 4.3 Implement emulation HTTP fetcher behind the emulation feature with pinned wreq profiles and verify a build with the feature and a build without it
- [ ] 4.4 Implement browser fetcher behind the browser feature with pinned Chromium, proxy passing, response caps, and private-address checks, and verify doctor reports browser availability
- [ ] 4.5 Implement normalized record parser pipeline and verify required-field rejection with a unit test
- [ ] 4.6 Add shared emulation-profile headers and one thread-safe proxy selector with health feedback for all fetchers, and verify hints match the profile and rejected proxies are never reused

## 5. Engine behavior

- [ ] 5.1 Implement per-source governor rate limits and tokio concurrency semaphores and verify separate source limits with a timing test
- [ ] 5.2 Implement bounded jittered retries with Retry-After and cancellation and verify an oversized delay is interrupted on cancel
- [ ] 5.3 Implement challenge detection and per-source circuit breaker and verify a challenge response stops the source
- [ ] 5.4 Implement canonical URL normalization and content-hash dedup with a disable switch and verify idempotent output across two runs
- [ ] 5.5 Implement versioned checkpoint and resume bound to a config fingerprint and verify a matching checkpoint skips completed work
- [ ] 5.6 Implement global and per-source result caps and fail-open policy and verify the run summary records skipped failures
- [ ] 5.7 Implement enrichment before emission with contact provenance, deterministic scoring, authoritative company_domain, dedup, and source attribution, and verify every stdout record is final
- [ ] 5.8 Implement challenge attention events with opaque resume state bound to the config fingerprint and verify only the challenged source pauses

## 6. Output contract

- [ ] 6.1 Implement normalized record with schema version, provenance, timestamp, and content hash and verify serialization includes every field
- [ ] 6.2 Implement NDJSON, JSON, and CSV writers with correct escaping and verify each format with a golden test
- [ ] 6.3 Implement atomic file writes and hash manifests and verify no partial file remains after a simulated failure
- [ ] 6.4 Implement schema version reader rejection and verify an unknown version produces a clear error
- [ ] 6.5 Add enriched record fields and email provenance to the normalized schema, and verify an adversarial company-domain fixture cannot inflate quality_score
- [ ] 6.6 Implement immediate stdout line flush with no partial lines on cancellation and verify emitted records survive an interrupted run
- [ ] 6.7 Separate URL dedup from title-company dedup and verify disabling dedup preserves records sharing title and company

## 7. Security and privacy

- [ ] 7.1 Wrap credentials in redacting types and verify debug output contains only a redaction marker
- [ ] 7.2 Add tracing field masking and URL query redaction and verify a synthetic secret never appears in logs or errors
- [ ] 7.3 Implement DNS-resolving SSRF protection with redirect revalidation and validated-address connection and verify literal and resolved private addresses are blocked
- [ ] 7.4 Implement hard body size and memory ceilings and verify oversized responses are rejected
- [ ] 7.5 Add fixture scrubbing tooling and verify committed fixtures contain no synthetic personal data
- [ ] 7.6 Keep guessed email and commit-email harvesting disabled by default and verify no such records appear in a default run
- [ ] 7.7 Add wire-protocol secret checks across stdout, stderr, progress events, and resume state, and verify a synthetic secret never appears in any surface
- [ ] 7.8 Inspect challenge bodies before closing responses and classify challenges separately from generic 403 failures, and verify with a challenge fixture

## 8. Fixtures and offline tests

- [ ] 8.1 Define fixture directory contract with request descriptor, response body, headers, frozen timestamp, and golden record, and verify a validator test passes
- [ ] 8.2 Build fixture replay harness that injects saved responses into real parsers and verify a parser test passes with networking disabled
- [ ] 8.3 Add CLI contract tests for precedence, scalar forms, explicit false, formats, config writes, doctor parity, and invalid exits, and verify the suite passes
- [ ] 8.4 Add secret-leak CI check with synthetic secrets and verify the check fails on an injected leak
- [ ] 8.5 Add bridge contract tests for stdin config, progress events, exit codes, flush behavior, and cancellation, and verify the suite passes offline

## 9. Source porting Tier 1

- [ ] 9.1 Port arbeitnow, remoteok, remotive, workingnomads, himalayas, and jobicy with fixtures and parser tests, and verify cargo test passes offline
- [ ] 9.2 Port hackernews, cryptocurrencyjobs, fossjobs, pythonjobs, railsjobs, golangjobs, and devopsjobs with fixtures and parser tests, and verify cargo test passes offline
- [ ] 9.3 Port crunchboard, jobspresso, hasjob, landingjobs, realworkfromanywhere, and weworkremotely with fixtures and parser tests, and verify cargo test passes offline
- [ ] 9.4 Port huggingfacejobs, androidjobs, aijobs, gunio, ycjobs, 4dayweek, functionalworks, getonboard, and ecojobs with fixtures and parser tests, and verify cargo test passes offline

## 10. Source porting Tier 2

- [ ] 10.1 Port builtin, internshala, wuzzuf, freelancercom, and jobindex with fixtures and parser tests, and verify cargo test passes offline
- [ ] 10.2 Port jobsdb, jobstreet, mycareersfuture, reed, and hiringcafe with fixtures and parser tests, and verify cargo test passes offline
- [ ] 10.3 Port simplyhired and ukvisajobs with fixtures and parser tests, and verify cargo test passes offline

## 11. Source porting Tier 3

- [ ] 11.1 Port greenhouse, ats-ashby, ats-smartrecruiters, ats-recruitee, ats-workable, ats-rippling, and ats-talentlyft with slug manifests and fixtures, and verify cargo test passes offline
- [ ] 11.2 Port adzuna, careerjet, and findwork with credential declarations and fixtures, and verify missing keys skip without requests

## 12. CI and release

- [ ] 12.1 Add CI matrix for stable Rust, offline cargo test --locked, and feature builds, and verify a dry CI run passes
- [ ] 12.2 Add cargo audit and cargo deny checks and verify a known vulnerable fixture dependency fails the check
- [ ] 12.3 Add musl static build with file and ldd verification and verify the binary runs in a minimal container
- [ ] 12.4 Document config migration from the old binary and verify every documented command works against the new CLI
- [ ] 12.5 Build and upload every installation-documented artifact in the release workflow, verify checksums in installers, and smoke-test each artifact

## 13. Live validation, opt-in

- [ ] 13.1 Add a separate ignored live validation suite that reports working, keyed, blocked, stale, and unregistered sources, and verify normal cargo test does not execute it
- [ ] 13.2 Run live validation once with configured rate limits and record the result in a non-committed report, and verify no source exceeds its configured request rate

## Workflow follow-up

- Archive the change after review and after the opt-in live validation report is accepted.
- Verify the archived specs match the implemented behavior.
