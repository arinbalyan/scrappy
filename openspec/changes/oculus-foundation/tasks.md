# Tasks

## 1. Repository scaffolding and licensing

- [x] 1.1 Create a minimal Cargo package with src/main.rs and src/lib.rs and verify that cargo build --locked succeeds
- [x] 1.2 Create .gitignore covering target, local config, and secret files and verify no generated artifacts appear in git status
- [x] 1.3 Write LICENSE in MIT form and NOTICE documenting clean-room provenance, the read-only references, and third-party license obligations, and verify both files are present
- [x] 1.4 Add Cargo.toml with explicit feature flags for emulation and browser and verify that cargo tree shows no native-tls or openssl in the default build
- [x] 1.5 Verify that .codex/skills/ms-rust/SKILL.md and its generated guideline files are present, and record the ms-rust workflow in documentation/014-testing.md
- [x] 1.6 Create documentation/ with the file layout from design.md and verify that every document uses paths relative to the scrappy repository root
- [x] 1.7 Add a documentation parity check that fails when a document references a missing source path or a public module lacks a documentation entry, and verify it runs offline
- [x] 1.8 Add cargo fmt, cargo clippy --all-targets -- -D warnings, and cargo doc --no-deps to CI and verify a clean run on the scaffolding

## 2. CLI and configuration foundation

- [x] 2.1 Implement clap subcommands for run, validate, config init/show/set/unset/edit/explain/schema, list-sources, doctor with fix, completion, and version with global --help and --version, and verify that help lists every subcommand, flag, and one example per subcommand
- [x] 2.2 Implement typed Config with Option fields and layered resolution in CLI, environment, project config, user config, default order, and verify precedence with a table-driven test
- [x] 2.3 Add OneOrMany deserialization for scalar and array config values and verify both forms load in tests
- [x] 2.4 Add tri-state boolean flags with negative counterparts and verify that explicit false overrides config true
- [x] 2.5 Add strict validation for unknown keys, unknown sources, dates, memory limits, and rate maps, and verify invalid input exits nonzero with a stderr diagnostic
- [x] 2.6 Implement machine-separated stdout and stderr and verify that warnings do not corrupt JSON output
- [x] 2.7 Implement atomic config set that preserves unknown and per-source sections with 0700 directories and 0600 files, and verify with a tempdir test
- [x] 2.8 Implement doctor with named checks for config, schema, sources, credentials, proxies, browser, fixtures, output writability, and features, with JSON output and a fix action that counts failed repairs as failures, and verify key values are redacted
- [x] 2.9 Implement run --stdin-config with one JSON document, EOF start, and full run-config key validation, and verify malformed input exits 2 with no fetch
- [x] 2.10 Implement structured NDJSON progress events on stderr with run id, source id, counters, terminal event, and published exit codes 0 through 5, and verify a consumer can parse every event line
- [x] 2.11 Add actionable validation for enums, ranges, unsupported filters, and preflight diagnostics, and verify each invalid case exits 2 before any request
- [x] 2.12 Implement config init with a commented template documenting field types, defaults, and environment variables, and verify no secret values appear in the template
- [x] 2.13 Implement config schema and enum-value printing from the runtime schema, and verify every runtime field is documented
- [x] 2.14 Implement shell completion generation and verify supported shells produce scripts while an unsupported shell exits 2
- [x] 2.15 Implement config init as a non-destructive write that refuses to overwrite an existing file unless explicitly forced, and verify the original file is unchanged on a refused write
- [x] 2.16 Restrict doctor fix to documented safe repairs of config files and generated artifacts, and verify it never modifies source, fixture, or manifest files
- [x] 2.17 Add scraper behavior docs under documentation/scrapers/ and define the record format used when a scraper is fixed

## 3. Source registry and manifests

- [x] 3.1 Define the versioned source manifest schema and registry loader and verify duplicate IDs and unknown fields fail validation
- [x] 3.2 Add credential requirement declarations and verify that a missing key produces a missing-credentials status with no request
- [x] 3.3 Add robots policy, attribution, and fixture set fields and verify validation rejects missing or empty values
- [x] 3.4 Implement list-sources health states and verify counts match registered manifests
- [x] 3.5 Generate status documentation from the registry and verify generated counts match list-sources output
- [x] 3.6 Add list-sources --format json with health, failure, credential, fetcher, slug, and fixture fields, and verify empty and blocked states are distinguishable
- [x] 3.7 Add registry parity checks for default selection, fixtures, contract tests, status uniqueness, and slug counts, and verify drift fails CI offline
- [x] 3.8 Implement custom source manifests from a documented user config directory using built-in parser kinds, and verify a custom source runs without rebuilding or overriding a built-in ID
- [x] 3.9 Record one behavior document in documentation/scrapers/<source_id>.md for every ported source, covering endpoint family, parser, fetcher, quirks, and break/fix history

## 4. Fetching and parsing foundation

- [x] 4.1 Define the Fetcher trait with validated request and bounded response types and verify a mock fetcher can drive a parser test
- [x] 4.2 Implement plain HTTP fetcher with reqwest rustls and verify it works against a local test server
- [x] 4.3 Implement emulation HTTP fetcher behind the emulation feature with pinned wreq profiles and verify a build with the feature and a build without it
- [x] 4.4 Implement browser fetcher behind the browser feature with pinned Chromium, proxy passing, response caps, and private-address checks, and verify doctor reports browser availability
- [x] 4.5 Implement normalized record parser pipeline and verify required-field rejection with a unit test
- [x] 4.6 Add shared emulation-profile headers and one thread-safe proxy selector with health feedback for all fetchers, and verify hints match the profile and rejected proxies are never reused

## 5. Engine behavior

- [x] 5.1 Implement per-source governor rate limits and tokio concurrency semaphores and verify separate source limits with a timing test
- [x] 5.2 Implement bounded jittered retries with Retry-After and cancellation and verify an oversized delay is interrupted on cancel
- [x] 5.3 Implement challenge detection and per-source circuit breaker and verify a challenge response stops the source
- [x] 5.4 Implement canonical URL normalization and content-hash dedup with a disable switch and verify idempotent output across two runs
- [x] 5.5 Implement versioned checkpoint and resume bound to a config fingerprint and verify a matching checkpoint skips completed work
- [x] 5.6 Implement global and per-source result caps and fail-open policy and verify the run summary records skipped failures
- [ ] 5.7 Implement enrichment before emission with contact provenance, deterministic scoring, authoritative company_domain, dedup, and source attribution, and verify every stdout record is final
- [x] 5.8 Implement challenge attention events with opaque resume state bound to the config fingerprint and verify only the challenged source pauses

## 6. Output contract

- [x] 6.1 Implement normalized record with schema version, provenance, timestamp, and content hash and verify serialization includes every field
- [x] 6.2 Implement NDJSON, JSON, and CSV writers with correct escaping and verify each format with a golden test
- [x] 6.3 Implement atomic file writes and hash manifests and verify no partial file remains after a simulated failure
- [x] 6.4 Implement schema version reader rejection and verify an unknown version produces a clear error
- [x] 6.5 Add enriched record fields and email provenance to the normalized schema, and verify an adversarial company-domain fixture cannot inflate quality_score
- [x] 6.6 Implement immediate stdout line flush with no partial lines on cancellation and verify emitted records survive an interrupted run
- [x] 6.7 Separate URL dedup from title-company dedup and verify disabling dedup preserves records sharing title and company

## 6a. Canonical tree and consumer-ready output

- [x] 6a.1 Add a canonical-tree parity check for every path in design.md, including manifest family directories, numbered documentation files, scraper documents, and fixture files, and verify it fails on a removed path
- [x] 6a.2 Add request.template.json and frozen.json to the fixture contract and verify replay derives request.json from the template and timestamp
- [x] 6a.3 Verify CSV output has a stable header and standard escaping, and verify NDJSON stdout records use the same field names

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

- [x] 8.1 Define fixture directory contract with request descriptor, response body, headers, frozen timestamp, and golden record, and verify a validator test passes
- [x] 8.2 Build fixture replay harness that injects saved responses into real parsers and verify a parser test passes with networking disabled
- [ ] 8.3 Add CLI contract tests for precedence, scalar forms, explicit false, formats, config writes, doctor parity, doctor fix failure counting, init template, schema output, completion, version, and invalid exits, and verify the suite passes
- [ ] 8.4 Add secret-leak CI check with synthetic secrets and verify the check fails on an injected leak
- [ ] 8.5 Add bridge contract tests for stdin config, progress events, exit codes, flush behavior, and cancellation, and verify the suite passes offline

## 9. Source porting Tier 1

- [x] 9.0 Add manifest fields for a request template name, selector map, and parser options, then add source adapters that turn each manifest into a validated request and a parsed record set
- [x] 9.1 Port arbeitnow, remoteok, remotive, workingnomads, himalayas, and jobicy with fixtures and parser tests, and verify cargo test passes offline
- [x] 9.5 Write scraper behavior docs for every Tier 1 source and verify each source id has a matching document
- [x] 9.2 Port hackernews, cryptocurrencyjobs, fossjobs, pythonjobs, railsjobs, golangjobs, and devopsjobs with fixtures and parser tests, and verify cargo test passes offline
- [x] 9.3 Port crunchboard, jobspresso, hasjob, landingjobs, realworkfromanywhere, and weworkremotely with fixtures and parser tests, and verify cargo test passes offline
- [x] 9.4 Port huggingfacejobs, androidjobs, aijobs, gunio, ycjobs, 4dayweek, functionalworks, getonboard, and ecojobs with fixtures and parser tests, and verify cargo test passes offline

## 10. Source porting Tier 2

- [x] 10.0 Extend the source adapter path to static HTML and rendered HTML families with selector-based extraction, POST method, header and body templates, and embedded JSON parsing
- [x] 10.1 Port builtin, internshala, wuzzuf, freelancercom, and jobindex with fixtures and parser tests, and verify cargo test passes offline
- [x] 10.2 Port jobsdb, jobstreet, mycareersfuture, reed, and hiringcafe with fixtures and parser tests, and verify cargo test passes offline
- [x] 10.3 Port simplyhired and ukvisajobs with fixtures and parser tests, and verify cargo test passes offline
- [x] 10.4 Write scraper behavior docs for every Tier 2 source and verify each source id has a matching document

## 11. Source porting Tier 3

- [x] 11.0 Extend the source adapter path to keyed and slug-dependent ATS families with credential and slug substitution
- [x] 11.1 Port greenhouse, ats-ashby, ats-smartrecruiters, ats-recruitee, ats-workable, ats-rippling, and ats-talentlyft with slug manifests and fixtures, and verify cargo test passes offline
- [x] 11.2 Port adzuna, careerjet, and findwork with credential declarations and fixtures, and verify missing keys skip without requests
- [x] 11.3 Write scraper behavior docs for every Tier 3 source and verify each source id has a matching document

## 11a. Niche feed and API source ports

- [x] 11a.1 Port berlinstartupjobs, conservationjobs, djinni, drupaljobs, echojobs, eurojobs, nofluffjobs, remotefirstjobs, themuse, and wordpressjobs with manifests, behavior docs, and six-file fixtures
- [x] 11a.2 Add offline golden replay coverage for each niche source and verify the suite passes without networking
- [x] 11a.3 Align the nofluffjobs manifest and fixtures with the historical posting API and verify replay passes

## 11b. Search feed and API source ports

- [x] 11b.1 Port jobsinjapan, academiccareers, icrunchdata, higheredjobs, cryptojobslist, and jobsch with manifests, behavior docs, and six-file fixtures
- [x] 11b.2 Add offline golden replay coverage in tests/tier5_search_replay.rs and verify the suite passes without networking
- [x] 11b.3 Record live probe results, mark challenge-blocked sources, and regenerate documentation/005-status.md
- [x] 11b.4 Verify the offline suite with cargo test and strict OpenSpec validation

## 11c. Public API source ports

- [x] 11c.1 Port canadajobbank, coroflot, joinrise, startupjobs, and web3career with manifests, behavior docs, and six-file fixtures
- [x] 11c.2 Add offline golden replay coverage in tests/tier6_public_replay.rs and verify the suite passes without networking
- [x] 11c.3 Record live probe results, mark challenge-blocked sources, and regenerate documentation/005-status.md

## 11d. Keyed API source ports

- [x] 11d.1 Port deel, fountain, dice, snagajob, and talroo with manifests, behavior docs, and six-file fixtures
- [x] 11d.2 Add offline golden replay coverage in tests/tier7_keyed_replay.rs with synthetic credentials and verify the suite passes without networking
- [x] 11d.3 Record live probe results, mark challenge-blocked snagajob, and regenerate documentation/005-status.md
- [x] 11d.4 Verify the offline suite with cargo fmt, clippy, cargo test, emulation check, and strict OpenSpec validation

## 11e. Credentialed API source ports

- [x] 11e.1 Port authenticjobs, jobdataapi, usajobs, infojobs, and exa with manifests, behavior docs, and six-file fixtures
- [x] 11e.2 Add offline golden replay coverage in tests/tier8_credentialed_replay.rs with rendered credentialed requests and golden records
- [x] 11e.3 Record live probe results, mark challenge-blocked authenticjobs and usajobs, and regenerate documentation/005-status.md
- [x] 11e.4 Verify the offline suite with cargo fmt, clippy, cargo test, emulation check, and strict OpenSpec validation

## 11f. Seed-based ATS API source ports

- [x] 11f.1 Port bamboohr, breezyhr, comeet, jobscore, and manatal with slug-based manifests, behavior docs, and six-file fixtures
- [x] 11f.2 Add offline golden replay coverage in tests/tier9_ats_replay.rs with rendered slug requests and golden records
- [x] 11f.3 Record live probe results, update health labels where needed, and regenerate documentation/005-status.md
- [x] 11f.4 Verify the offline suite with cargo fmt, clippy, cargo test, emulation check, and strict OpenSpec validation

## 12. CI and release

- [ ] 12.1 Add CI matrix for stable Rust, offline cargo test --locked, and feature builds, and verify a dry CI run passes
- [ ] 12.2 Add cargo audit and cargo deny checks and verify a known vulnerable fixture dependency fails the check
- [ ] 12.3 Add musl static build with file and ldd verification and verify the binary runs in a minimal container
- [ ] 12.4 Document config migration from the old binary and verify every documented command works against the new CLI
- [ ] 12.5 Build and upload every installation-documented artifact in the release workflow, verify checksums in installers, and smoke-test each artifact

## 13. Live validation, opt-in

- [ ] 13.1 Add a separate ignored live validation suite that reports working, keyed, blocked, stale, and unregistered sources, and verify normal cargo test does not execute it
- [ ] 13.2 Run live validation once with configured rate limits and record the result in a non-committed report, and verify no source exceeds its configured request rate
- [ ] 13.3 Update affected documentation/scrapers/<source_id>.md entries when live validation finds a broken source, and verify the break and fix are recorded

## Workflow follow-up

- Archive the change after review and after the opt-in live validation report is accepted.
- Verify the archived specs match the implemented behavior.
