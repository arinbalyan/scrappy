# Tasks

## 1. Preflight and contracts

- [x] 1.1 Confirm `.gitignore`, `LICENSE`, `NOTICE`, `Cargo.toml`, and `.codex/skills/ms-rust/SKILL.md` exist, and verify `git status` shows no generated artifacts
- [x] 1.2 Load the ms-rust skill and applicable guideline files before editing Rust, and record the guideline version in the change notes
- [x] 1.3 Add optional manifest fields for cursor and fanout plans plus per-source request budgets, and verify existing manifests parse without edits via `cargo test --test source_registry_contract`
- [x] 1.4 Extend manifest validation for malformed pagination, zero budgets, and unsupported plan forms, and verify each invalid fixture exits with a field-level diagnostic
- [x] 1.5 Narrow challenge detection to HTML, WAF headers, or challenge-shaped bodies, and verify JSON job payloads from getonboard and gunio parse instead of being classified as CAPTCHAs
- [x] 1.6 Document the manifest additions in `documentation/013-source-registry.md` and verify every documented key exists in the parser

## 2. Pagination execution

- [x] 2.1 Extend `src/engine/pagination.rs` with page, offset, cursor, and fanout plans, and verify unit tests cover ranges, steps, invalid forms, and bounded budgets
- [x] 2.2 Add repeated-page fingerprinting and empty-page stop reasons, and verify tests prove each stop reason is reported
- [x] 2.3 Execute pagination with per-request governor permits, and verify a two-page mock source records two permits and one source outcome
- [x] 2.4 Preserve collected records when a later page fails, and verify a mock first-success then failure returns partial records with a partial status
- [ ] 2.5 Add cursor-token and next-URL execution, and verify a mock source follows two cursor pages without looping
- [ ] 2.6 Add bounded id-array fanout, and verify a mock source caps requests and concurrency
- [x] 2.7 Wire the execute plan into `run` and verify `cargo run -- run --source arbeitnow` returns records from all three declared pages
- [ ] 2.8 Switch `tests/live/harvest.rs` to the shared executor and verify a filtered live run reports request count and stop reason
- [x] 2.9 Update `documentation/006-architecture.md` and `documentation/014-testing.md` with pagination behavior, and verify documented commands match the CLI

## 3. Fetch resilience

- [x] 3.1 Add transport retry for 429, 408, and 5xx with random jitter and a test seam, and verify tests assert delay bounds and attempt counts
- [x] 3.2 Parse Retry-After in seconds and HTTP-date form, and verify both forms clamp to the configured maximum
- [x] 3.3 Reuse one HTTP client per run and verify the fetcher construction test shows one client for multiple requests
- [x] 3.4 Add an explicit user-agent policy with per-source override, and verify a request without an override carries the package user agent
- [ ] 3.5 Add optional proxy and cookie configuration with redacted diagnostics, and verify secret-leak tests reject proxy credentials in logs
- [x] 3.6 Acquire concurrency permits per request and verify a mock source with two pages never exceeds its cap
- [x] 3.7 Add global and per-host concurrency caps, and verify a multi-source mock never exceeds the global cap
- [x] 3.8 Interpret robots values by prefix and verify descriptive allow and disallow strings produce fetch and policy-skip outcomes
- [x] 3.9 Verify cancellation interrupts backoff and pagination waits, and confirm exit code 5
- [x] 3.10 Update `documentation/006-architecture.md`, `documentation/015-security.md`, and `documentation/014-testing.md` with retry and identity rules

## 4. Credential flow

- [x] 4.1 Resolve declared credentials from environment and config into `RunInput.values`, and verify a mock keyed source receives its key
- [x] 4.2 Enforce CLI over config over environment precedence, and verify a table-driven test proves the order
- [x] 4.3 Emit a keyed outcome when credentials are missing and continue other sources, and verify a mixed run exits 3 with one keyed source
- [x] 4.4 Extend doctor to report credential names and presence counts, and verify JSON output contains no values
- [x] 4.5 Add secret-leak coverage for progress events, errors, fixtures, and output, and verify every check passes
- [x] 4.6 Point status, doctor, and live harvest at the same credential-presence check, and verify listing, doctor, and probe agree on keyed readiness
- [x] 4.7 Document credential names, environment variables, and precedence in `documentation/004-configuration.md` and `documentation/003-cli-reference.md`

## 5. Record completeness

- [x] 5.1 Add posted timestamp, compensation, apply method, direct apply URL, company URL, industry, and remote fields to parser records, and verify JSON, RSS, and Atom tests
- [x] 5.2 Pass parsed fields into `JobInput`, and verify a live SmartRecruiters run emits `date_posted` and `fetched_at` separately
- [x] 5.3 Derive `external_id` from source-native ids with hash fallback, and verify both cases
- [x] 5.4 Merge source ids for duplicate canonical URLs, and verify one record carries two source ids in stable order
- [x] 5.5 Add company-domain derivation and apply-route fields to output tests, and verify the schema remains version 1 compatible
- [x] 5.6 Bump output schema to version 2, keep `read_versioned` accepting version 1, and verify both versions deserialize
- [x] 5.7 Document the schema migration in `documentation/009-export-formats.md` and `documentation/016-config-migration.md`

## 6. Source coverage: public feeds and dead sources

- [x] 6.1 Apply the viability matrix to all 111 manifests and write a dated decision per source, and verify every source id appears once
- [x] 6.2 Re-probe and repair `androidjobs`, `eurojobs`, `djinni`, `drupaljobs`, `jobindex`, and `themusee`, and verify each either returns records or is marked stale with evidence
- [x] 6.3 Confirm Cloudflare and challenge outcomes for `crunchboard`, `remotefirstjobs`, `icrunchdata`, `snagajob`, `startupjobs`, `wuzzuf`, and `hiringcafe`, and verify no bypass attempt is made
- [x] 6.4 Repair `echojobs` throttling with backoff and per-source budget, and verify a live probe either returns records or records a dated blocked reason
- [ ] 6.5 Repair `headhunter` only if a public identity policy works, and verify the API response before changing health
- [x] 6.6 Remove `androidjobs`, `eurojobs`, and `drupaljobs` only after two dated probes, and verify manifest, fixtures, behavior docs, and generated status are removed together
- [ ] 6.7 Diagnose `canadajobbank` and `joinrise` DNS failures and either repair the routes or mark them environment-dead with dated evidence
- [ ] 6.8 Diagnose `careeronestop` connectivity and `functionalworks` body decoding, and verify both either succeed or carry a dated environment reason
- [x] 6.9 Make the response body ceiling configurable and verify `nofluffjobs` either parses under a documented limit or is marked oversized with evidence
- [ ] 6.10 Add DNS and body-size diagnostics to doctor and verify JSON output names the failing check without leaking URLs with credentials
- [x] 6.11 Update every affected `documentation/scrapers/<source_id>.md` with route, evidence, and break/fix history
- [x] 6.12 Run the public-feed replay suites and verify no fixture regression

## 7. Source coverage: ATS and new adapters

- [x] 7.1 Re-verify the 14 working ATS seeds and confirm each still returns records, and update behavior docs with counts
- [ ] 7.2 Add Workday support for a full tenant plus career-site tuple, and verify NVIDIA and Salesforce probes
- [x] 7.3 Add iCIMS HTML listing support or mark it new-adapter, and verify the parser against a synthetic page
- [x] 7.4 Add Jobvite XML feed support or mark it new-adapter, and verify Egnyte returns records
- [x] 7.5 Evaluate UKG tenant plus board UUID support, and verify one public board before committing a seed
- [x] 7.6 Classify `ats-talentlyft`, `ats-recruiterflow`, `loxo`, `manatal`, `ats-phenom`, `ats-successfactors`, `adp`, and `ats-rippling`, and verify each has a dated reason and correct health
- [x] 7.7 Update ATS fixtures, replay tests, behavior docs, and generated status together, and verify `cargo test --test tier9_ats_replay --test tier13_tenant_ats_replay`

## 8. Live verification and statistics

- [x] 8.1 Extend the live report with request count, stop reason, family, transport class, and latency percentiles, and verify the JSON schema test
- [x] 8.2 Run a full-registry probe with pagination and credentials where available, and verify every source has an outcome or a documented ineligible reason
- [x] 8.3 Run filtered probes for one fast source, one slow source, one paged source, one keyed source, and one blocked source, and verify the report distinguishes them
- [x] 8.4 Publish the updated counts-only inventory under `documentation/audits/` and verify it contains no addresses or descriptions
- [x] 8.5 Update `documentation/014-testing.md` with the live matrix and expected non-CI isolation, and verify normal `cargo test` stays offline

## 9. Integration and release

- [x] 9.1 Run `cargo fmt --all -- --check`, `cargo test --locked --tests --quiet`, and `cargo clippy --locked --all-targets --features emulation -- -D warnings`
- [x] 9.2 Run `openspec validate oculus-production-hardening --strict`, the secret-leak check, the fixture PII check, and generated-status parity
- [x] 9.3 Verify no downstream product names appear, no live report is committed, and no secret is present in git history staged for this change
- [x] 9.4 Review every changed source doc against its manifest and fixture, and verify the three agree

## Workflow follow-up

- Archive `oculus-foundation` before applying this change, because it is complete but unarchived.
- Archive this change only after the user reviews the live inventory and accepts the remaining blocked and credential-only sources.
- Re-run the full live probe on an unrestricted network before archive.
