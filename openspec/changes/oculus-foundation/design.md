# Design

## Context

See proposal.md for motivation and specs/ for the behavior contract. The branch preserves main's history with all inherited files removed; there is no existing code to preserve. The old Go implementation on main and other parent-workspace repositories are read-only evidence of behavior. Every file written here must be authored fresh. Toolchain available locally: rustc and cargo 1.100.0.

## Goals / Non-Goals

The authoritative source tree is fixed by the layout below. Each subsystem is
a directory with a `mod.rs` root. `src/sources` contains `manifest.rs`,
`registry.rs`, and `manifests/` with one directory per adapter family. The
runtime bridge contract lives in `src/engine/contract.rs`. Integration tests
are directory-based targets declared in Cargo.toml.

```text
scrappy/
  Cargo.toml
  Cargo.lock
  README.md
  LICENSE
  NOTICE
  .gitignore
  openspec/
  .codex/skills/ms-rust/
  documentation/
    001-quickstart.md ... 016-config-migration.md
    scrapers/
  src/
    main.rs
    lib.rs
    cli/
      mod.rs
      doctor.rs
    config/
      mod.rs
      file.rs
    sources/
      mod.rs
      manifest.rs
      registry.rs
      manifests/
        mod.rs
        feeds_api/
        ats/
        html_static/
        html_rendered/
        keyed/
    fetching/
      mod.rs
      plain.rs
      emulation.rs
      browser.rs
      proxy.rs
    parsing/
      mod.rs
      json.rs
      rss.rs
      html.rs
      jsonld.rs
    engine/
      mod.rs
      contract.rs
    output/mod.rs
    security/mod.rs
    checkpoint/mod.rs
    metrics/mod.rs
  tests/
    fixtures/<source_id>/
      request.json
      request.template.json
      response.body
      headers.json
      frozen.json
      golden.ndjson
    cli_contract/
      mod.rs
      doctor.rs
    bridge_contract/
      mod.rs
    config_actions.rs
    docs_parity.rs
    live/
  .github/workflows/
```

Empty directories keep a short README so the layout is visible in version
control. Source manifests and fixtures use the paths shown above, and
documentation uses repository-relative paths only.

This tree is the required shape of the repository, not an illustration. A
parity check fails when any named path is missing or when a module is placed
outside its named directory. New files belong under an existing subsystem and
are recorded in design.md when they establish a new contract. Adding a source
means adding a manifest under its adapter-family directory, a fixture set under
tests/fixtures/<source_id>/, and a behavior document under documentation/scrapers/;
it does not create a new top-level directory.

**Goals:**

- Produce a small installable Rust binary with a clean CLI and typed configuration.
- Make fetching, parsing, and output independently testable with offline fixtures.
- Fix every CLI, config, security, and stealth gap identified in the audits.
- Keep the default build small and free of a browser runtime dependency.
- Establish a source port order grounded in observed source health.
- Preserve a clean-room provenance trail for licensing.

**Non-Goals:**

- Modifying the other parent-workspace projects or the old scrappy main branch.
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
- tests/fixtures/<source_id>/ holds a reusable request.template.json, a frozen request.json, response bodies, headers.json, frozen.json, and golden.ndjson records.
- .github/workflows/ holds CI for tests, audits, static builds, and secret-leak checks.

Alternative considered: a workspace with many crates. Rejected for a single contributor because it adds build overhead and indirection without a team boundary to enforce.
### 1b. Scraper location and classification
A scraper is not one file. It is the combination of a declarative manifest in src/sources/manifests/<family>/<source_id>.toml, a parser kind from src/parsing/, and a fetcher kind from src/fetching/. The manifest selects the request template, parser, fetcher, emulation profile, selector strategy, credential names, robots policy, and fixtures. Shared parsing code stays in src/parsing/; source-specific differences stay in manifests and fixtures. A custom source follows the same model from the documented user config directory.
Sources are classified on two axes. The adapter family controls code placement and runtime behavior: feeds_api, ats, html_static, html_rendered, and keyed. The health state controls eligibility: working, keyed, blocked, stale, requires_slug, and browser_only. Unregistered is a live-report and audit category for historical source IDs that have no manifest; it is not a health state. A registered source carries both axes, so a feed can be stale and an HTML page can be working. Numbered tiers in tasks.md are port order only and do not replace this classification.
The folder layout follows the family axis. Manifests live in src/sources/manifests/feeds_api/, ats/, html_static/, html_rendered/, and keyed/. Shared parsers live in src/parsing/ as json, rss, html, and jsonld modules. Fetchers live in src/fetching/ as plain, emulation, browser, and proxy modules. Fixtures live in tests/fixtures/<source_id>/. Behavior docs live in documentation/scrapers/<source_id>.md.
Every scraper gets a behavior document in documentation/scrapers/<source_id>.md. It records the source id, adapter family, health state, endpoint shape, parser, fetcher, emulation profile, credential and slug needs, rate and concurrency limits, robots policy, known quirks, and a dated break/fix log. When a scraper breaks and is repaired, the fix and the observed behavior are appended to that document in the same change. Documentation parity checks require one document per registered source and reject a document whose source id has no manifest.

### 1c. Downstream consumer compatibility
scrappy is a standalone scraper and is never named after its consumers. Its
contracts are shaped so a process consumer can use the binary without a
shared library or source changes. NDJSON on stdout, CSV and JSON writers,
versioned records, canonical URLs, deterministic hashes, provenance, and
structured progress events give a consumer stable fields to filter jobs and
route them into automated or human-attention lanes. An operator may invoke any
source, but the consumer owns its own selection, email, and application policy.

### 1a. Documentation and code style

Keep source comments short and factual. A comment or doc comment states what an item does or why a choice exists, then points to the relevant document in `documentation/` when more detail is needed. Long explanations, design notes, command references, config field tables, and operational runbooks live in `documentation/`. Document paths are written relative to the scrappy repository root.

The `documentation/` folder holds `001-quickstart.md`, `002-installation.md`, `003-cli-reference.md`, `004-configuration.md`, `005-status.md`, `006-architecture.md`, `007-email-extraction.md`, `008-quality-scoring.md`, `009-export-formats.md`, `010-proxy.md`, `011-dedup.md`, `012-browser-automation.md`, `013-source-registry.md`, `014-testing.md`, `015-security.md`, and `016-config-migration.md`. Each document names the source modules it explains. A documentation parity check in CI fails when a document references a missing path or a public module has no documentation entry.

The project-local ms-rust skill lives at `.codex/skills/ms-rust/SKILL.md`. It contains the Microsoft Pragmatic Rust Guidelines and the compliance marker rule. Before writing or reviewing any `.rs` file, load that skill and only the guideline files relevant to the task. Apply `cargo fmt`, `cargo clippy --all-targets -- -D warnings`, and `cargo doc --no-deps` as the baseline quality gate. Keep public items documented in canonical form and keep implementation detail out of public docs.

Alternative considered: long inline comments and a README-only documentation model. Rejected because the old project drifted from its own docs and the new binary needs a stable reference for CLI, config, sources, and operations.

### 2. CLI and configuration

Use clap 4 derive for subcommands and typed arguments. The command tree is `run`, `validate`, `config` (`init`, `show`, `set`, `unset`, `edit`, `explain`, `schema`), `list-sources`, `doctor` (with `fix`), `completion`, and `version`. Root and every subcommand support `--help` and `--version`; help includes one copyable example per subcommand and explains every flag. `completion` emits shell scripts and rejects unsupported shells. `version` exits without loading config.

Use serde and toml for config documents and figment for layered extraction. Environment values are read as a layer, never as clap flag defaults. Every field is Option<T> until resolution, then converted to validated typed values. List fields use a OneOrMany deserializer so a scalar and an array are both valid and an empty array stays empty. Booleans use Option<bool> with generated negative flags so explicit false survives lower layers.

Layer order is explicit CLI, environment, project config, user config, built-in default. The config flag may be repeated to add files; a no-default-config flag makes the run hermetic. Each resolved field records its winning layer and location so `config explain` can name it without printing secrets. `config init` generates a commented template with field types, defaults, and environment variable names. `config set`, `config unset`, and `config edit` operate on an explicit path and preserve unknown and per-source sections through a read-modify-atomic-write cycle. `config schema` prints the same schema used by the runtime parser.

Doctor derives its checks from the runtime schema and manifests. It reports named checks for config, schema, sources, credentials, proxies, browser, fixtures, output writability, and features. `doctor fix` applies only documented safe repairs and counts a failed fix as a failure. Doctor supports JSON output and never prints secret values.

Alternative considered: keeping the old flat flag set and patching merge logic. Rejected because the audit found defects in nearly every merge path; a typed resolver is smaller and fully testable.

### 2a. Automation bridge

A downstream automation consumer runs the binary as a process, so the Rust binary itself implements the wire protocol instead of exposing a Go-style library. A run --stdin-config mode reads one JSON object from stdin, treats it as the highest-precedence run-config layer, and starts fetching at EOF. stdout carries only enriched NDJSON records. stderr carries either human text or NDJSON progress events with a discriminated event field, run identifier, source identifier, counters, and a terminal event. Exit codes are published constants: 0 success, 1 internal failure, 2 usage or validation, 3 partial success, 4 challenge-paused, 5 cancelled.

Each record is enriched before emission: email extraction with provenance, MX verification, deterministic quality scoring, canonical company domain from source identity or company_url, deduplication, and source attribution. A challenge emits a separate event with an opaque resume state bound to the configuration fingerprint and pauses only the affected source. This keeps the consumer's routing policy outside the scraper while giving it the signals to decide.

Alternative considered: a thin Go bridge wrapping a scrappy package. Rejected because the clean-room Rust binary has no Go library and the process boundary must own the contract.

### 3. Source manifests and registry

Each source is a versioned TOML manifest with fields for source ID, display name, request template, parser kind, selector strategy, pagination, fetcher kind, emulation profile, requires_js, requests_per_minute, max_concurrency, robots policy, credential names, attribution, and fixture set. The registry validates unique lowercase IDs, required fields, credential declarations, and fixture presence. Status docs are generated from this registry.

Alternative considered: hard-coded Rust constructors per source. Rejected because it recreates the old drift where 141 declared sites had only 128 registered constructors.

Request templates apply to the URL, header values, and body. Substitutions are named slug, credential, or query values. A doubled brace renders as one literal brace, allowing declarative request bodies such as JSON payloads. Substitution values reject whitespace and braces before a request is built, and diagnostics name the placeholder without printing its value.

### 3a. Custom sources

Custom sources SHALL be added as local manifests under a documented user config directory, with a parser kind and fixture set drawn from the built-in parser library. Adding a source does not require rebuilding the binary or forking the repository. A custom manifest uses the same schema, validation, health states, rate limits, and secret handling as a built-in source, and it cannot override a built-in source identifier. External plugins compiled against Rust traits are out of scope for this change because they would break the small-binary and locked-build goals.

Alternative considered: a public Rust trait for runtime plugins. Rejected for this change because dynamic loading adds ABI and supply-chain risk; the declarative manifest path closes the old fork requirement without a plugin ABI.

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

Port sources in tiers derived from the audit. Tier 1 is public JSON and RSS with contract evidence: arbeitnow, remoteok, remotive, workingnomads, himalayas, jobicy, hackernews, cryptocurrencyjobs, fossjobs, pythonjobs, railsjobs, golangjobs, devopsjobs, crunchboard, jobspresso, hasjob, landingjobs, realworkfromanywhere, weworkremotely, huggingfacejobs, androidjobs, aijobs, gunio, ycjobs, 4dayweek, getonboard, ecojobs. Tier 2 is HTML or embedded JSON: builtin, functionalworks, internshala, wuzzuf, freelancercom, jobindex, jobsdb, jobstreet, mycareersfuture, reed, hiringcafe, simplyhired, ukvisajobs. Tier 3 is keyed or slug-dependent: greenhouse, ats-ashby, ats-smartrecruiters, ats-recruitee, ats-workable, ats-rippling, ats-talentlyft, adzuna, careerjet, findwork. Hold linkedin, indeed, and the 13 declared-but-unregistered sites until the core is stable.

Alternative considered: porting all 141 sites to match old marketing. Rejected because 20 historical IDs have no registered implementation and many have known keys, WAFs, or stale endpoints.

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
