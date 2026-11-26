# Design

## Context

See proposal.md for motivation and specs/ for the behavior contract. The branch preserves main's history with all inherited files removed; there is no existing code to preserve. The old Go implementation on main and the sibling JobHunter, job-ops, and JobRadar projects are read-only evidence of behavior. Every file written here must be authored fresh. Toolchain available locally: rustc and cargo 1.100.0.

## Goals / Non-Goals

**Goals:**

- Produce a small installable Rust binary with a clean CLI and typed configuration.
- Make fetching, parsing, and output independently testable with offline fixtures.
- Fix every CLI, config, security, and stealth gap identified in the audits.
- Keep the default build small and free of a browser runtime dependency.
- Establish a source port order grounded in observed source health.
- Preserve a clean-room provenance trail for licensing.

**Non-Goals:**

- Modifying JobHunter, job-ops, JobRadar, or the old scrappy main branch.
- Copying old source, fixtures, docs, prompts, config text, or branding.
- Solving CAPTCHAs, using credential-based access, or defeating access controls.
- Implementing email sending, application automation, or LLM features in this change.
- Running live network validation in normal CI.

## Decisions

### 1. Rust crate layout

Use a single Cargo package with a binary and library boundary.

- src/main.rs is a thin entry point that calls the library CLI runner.
- src/lib.rs exposes modules: cli, config, sources, fetching, parsing, engine, output, security, checkpoint, metrics.
- tests/ holds CLI contract tests and fixture replay tests.
- src/sources/manifests/ holds versioned TOML manifests, one per source.
- tests/fixtures/<source_id>/ holds sanitized request descriptors, response bodies, headers, and golden records.
- .github/workflows/ holds CI for tests, audits, static builds, and secret-leak checks.

Alternative considered: a workspace with many crates. Rejected for a single contributor because it adds build overhead and indirection without a team boundary to enforce.

### 2. CLI and configuration

Use clap 4 derive for subcommands and typed arguments. Use serde and toml for config documents and figment for layered extraction. Environment values are read as a layer, never as clap flag defaults. Every field is Option<T> until resolution, then converted to validated typed values.

Layer order is explicit CLI, environment, project config, user config, built-in default. The config flag may be repeated to add files; a no-default-config flag makes the run hermetic. Each field records its resolved source so config show can explain it. Booleans use Option<bool> with generated negative flags.

Alternative considered: keeping the old flat flag set and patching merge logic. Rejected because the audit found defects in nearly every merge path; a typed resolver is smaller and fully testable.

### 2a. Automation bridge

JobRadar consumes the binary as a process, so the Rust binary itself implements the wire protocol instead of exposing a Go-style library. A run --stdin-config mode reads one JSON object from stdin, treats it as the highest-precedence run-config layer, and starts fetching at EOF. stdout carries only enriched NDJSON records. stderr carries either human text or NDJSON progress events with a discriminated event field, run identifier, source identifier, counters, and a terminal event. Exit codes are published constants: 0 success, 1 internal failure, 2 usage or validation, 3 partial success, 4 challenge-paused, 5 cancelled.

Each record is enriched before emission: email extraction with provenance, MX verification, deterministic quality scoring, canonical company domain from source identity or company_url, deduplication, and source attribution. A challenge emits a separate event with an opaque resume state bound to the configuration fingerprint and pauses only the affected source. This keeps JobRadar's autopilot and attention routing policy outside the scraper while giving it the signals to decide.

Alternative considered: a thin Go bridge wrapping a scrappy package. Rejected because the clean-room Rust binary has no Go library and the process boundary must own the contract.

### 3. Source manifests and registry

Each source is a versioned TOML manifest with fields for source ID, display name, request template, parser kind, selector strategy, pagination, fetcher kind, emulation profile, requires_js, requests_per_minute, max_concurrency, robots policy, credential names, attribution, and fixture set. The registry validates unique lowercase IDs, required fields, credential declarations, and fixture presence. Status docs are generated from this registry.

Alternative considered: hard-coded Rust constructors per source. Rejected because it recreates the old drift where 141 declared sites had only 128 registered constructors.

### 4. Fetcher abstraction

Define a Fetcher trait that accepts a validated request and returns a bounded response. Three implementations:

- Plain HTTP: reqwest with default-features disabled and rustls-tls. Used for public APIs and feeds.
- Emulation HTTP: wreq with a pinned emulation profile from the manifest. Used for sources that need browser-like TLS and HTTP/2 behavior.
- Browser: chromiumoxide behind a browser feature. Used only for sources marked requires_js.

Parsing consumes a fetched response and emits normalized records. Parser modules never call the network directly, so fixture replay exercises the real parser.

Alternative considered: a single client for all sources. Rejected because wreq brings BoringSSL build cost and reqwest does not emulate browser fingerprints. Per-source selection keeps the default binary small.

### 5. BoringSSL and static builds

wreq and rquest use BoringSSL and require CMake, Perl, and Clang/libclang. Cross-compiling BoringSSL to musl needs a containerized toolchain and careful CA bundle handling. reqwest with rustls avoids native TLS and produces a simple static binary.

Decision: keep wreq behind an emulation feature as well as the browser feature. A minimal build uses only reqwest and rustls. A full build includes wreq and requires the documented toolchain. Never enable native-tls, native-tls-vendored, hyper-tls, or curl-sys. Never link OpenSSL and BoringSSL in the same binary. Pin exact wreq and wreq-util versions because emulation profile names change with browser releases.

Alternative considered: using rquest. Rejected for now because its stable line is older than wreq documented profile API and its 2.x line is pre-release. Revisit if profile maintenance matters more than build simplicity.

### 6. Config and secrets

Credentials live in environment variables or explicitly selected secret files, wrapped in redacting types from secrecy. Environment file parsing, if implemented, uses a real parser supporting quotes, inline comments, and export syntax, and is selected explicitly with a documented default. Saved config never contains credentials. A tracing field formatter masks fields named token, authorization, cookie, email, or phone as a safety net. CI injects synthetic secrets and fails if they appear in logs, errors, fixtures, or output.

Alternative considered: putting keys in config.toml. Rejected because the old setup wrote unquoted values, mis-parsed inline comments, and caused invalid proxy entries.

### 7. Politeness, retries, and challenges

Use governor token buckets per source and tokio semaphores for concurrency. Retries are bounded, jittered, cancellation-aware, respect Retry-After, and only apply to idempotent requests. Challenge detection covers DataDome, Cloudflare, generic JS-required markers, and repeated 403 or 429 responses. A challenged source stops, records a sanitized sample, backs off, and trips a circuit breaker after repeated failures. Robots policy in each manifest is honored. No CAPTCHA solving or credential-based access.

Alternative considered: the old semaphore-only approach. Rejected because the max-rps flags were actually concurrency caps and the real rate limiter was test-only.

### 8. Security and privacy

Validate every destination after DNS resolution, connect only to the validated address, revalidate redirects, and block private, loopback, link-local, and reserved ranges. Cap response bodies with a hard limit and enforce a real memory ceiling. Browser automation is opt-in, uses a pinned browser, passes configured proxies, caps HTML and cookie sizes, checks private addresses, and validates rendered output for challenges. Fixtures are scrubbed of personal data. Guessed emails and commit-email harvesting are disabled by default.

Alternative considered: literal host checks like the old email crawler. Rejected because they miss DNS rebinding and alternate IP encodings.

Fetchers share one thread-safe proxy selector with health feedback from real request outcomes, including the browser fetcher. Emulation profiles supply user agent and client hints together so they cannot contradict each other. Challenge inspection happens before response bodies close so a challenge is not misclassified as a generic retryable error. A single resolver locates browser dependencies for both doctor and the engine, and a browser-required source is skipped rather than silently downgraded.

### 9. Data integrity and resume

Normalized records carry a schema version, canonical URL, content hash, and fetched timestamp. Dedup uses canonical URL and content hash and can be disabled. Checkpoints are versioned and bound to a configuration fingerprint. Result files are written atomically with a hash manifest.

Alternative considered: the old global seen map that reset when results-wanted was zero. Rejected because it broke unlimited scrapes.

### 10. Port order

Port sources in tiers derived from the audit. Tier 1 is public JSON and RSS with contract evidence: arbeitnow, remoteok, remotive, workingnomads, himalayas, jobicy, hackernews, cryptocurrencyjobs, fossjobs, pythonjobs, railsjobs, golangjobs, devopsjobs, crunchboard, jobspresso, hasjob, landingjobs, realworkfromanywhere, weworkremotely, huggingfacejobs, androidjobs, aijobs, gunio, ycjobs, 4dayweek, functionalworks, getonboard, ecojobs. Tier 2 is HTML or embedded JSON: builtin, internshala, wuzzuf, freelancercom, jobindex, jobsdb, jobstreet, mycareersfuture, reed, hiringcafe, simplyhired, ukvisajobs. Tier 3 is keyed or slug-dependent: greenhouse, ats-ashby, ats-smartrecruiters, ats-recruitee, ats-workable, ats-rippling, ats-talentlyft, adzuna, careerjet, findwork. Hold linkedin, indeed, and the 13 declared-but-unregistered sites until the core is stable.

Alternative considered: porting all 141 sites to match old marketing. Rejected because 13 have no implementation and many have known keys, WAFs, or stale endpoints.

## Risks / Trade-offs

- [BoringSSL complicates static musl builds] → Keep emulation behind a feature, build it in a pinned container, verify with file and ldd, and smoke test in a minimal image.
- [Live source behavior changes faster than fixtures] → Fixtures are the test contract; live validation is a separate explicit workflow with its own report.
- [Strict validation breaks backward compatibility with old config] → This is an intentional clean-room break recorded in proposal.md; provide a migration doc during implementation.
- [Licensing depends on owner decision] → Write MIT files only during apply and keep a NOTICE that records clean-room provenance and third-party licenses.
- [Browser automation increases maintenance] → Keep it opt-in, pin the browser build, and limit it to sources that genuinely require JavaScript.
- [DNS validation may break sites using CDNs with dynamic IPs] → Revalidate at connect time and allow explicit, documented source exceptions only when their robots policy permits.
- [Rate limits reduce scrape throughput] → Correctness and politeness take priority; per-source limits are configurable.
- [Bridge contract changes with downstream needs] → Keep run-config keys and record fields versioned and validate them with golden contract tests before any release.
- [Published artifacts lag documentation] → Build and smoke-test every documented platform archive in the release workflow and verify downloads in installers.

## Migration Plan

No runtime migration exists because the working tree contains no inherited implementation. Implementation begins with scaffolding, then core contracts, then Tier 1 sources, then Tier 2 and Tier 3. Users of the old binary will need a documented config migration when this branch is released. Rollback is simply switching back to the old branch; no shared state is written by this change.
