# Design

## Context

See `proposal.md` for motivation. The implementation already has a manifest registry, a `Fetcher` trait separated from parsing, a typed config layer, NDJSON output, a governor, a circuit breaker, and fixture replay suites. The live audit at `documentation/audits/live-inventory-2026-11-27.md` is the baseline. This design builds on those contracts rather than replacing them.

## Goals / Non-Goals

**Goals:**

- Remove silent one-page scraping and make every declared strategy either executed or explicitly unsupported.
- Make retry and identity behavior good enough for long public-board runs without bypassing access controls.
- Make keyed sources runnable through credential resolution and make their absence a clear skip reason.
- Preserve every job field the parser can see.
- Make health labels and registry membership reflect observed behavior.
- Make live statistics complete and comparable across families.

**Non-Goals:**

- CAPTCHA solving, WAF bypass, or access-control circumvention.
- Turning challenge-only boards into working sources.
- Adding a database, queue, or service. The binary stays a CLI.
- Copying old Go code. Old behavior is an observed contract only.
- Making live network access part of normal CI.

## Decisions

### Pagination is a manifest contract with explicit plans

The manifest keeps `pagination` as text and adds no opaque script. The executor parses a small grammar into a typed plan.

| Form | Meaning | Request parameter |
| --- | --- | --- |
| `none` | one request | none |
| `page:N` | pages 1 through N | `page` |
| `page:start:end` | inclusive page range | `page` |
| `offset:step` | one offset request | `offset` |
| `offset:step:count` | offsets 0, step, ... | `offset` |
| `cursor:<param>` | follow a token or next URL | named cursor |
| `fanout:count:size` | bounded fanout of item requests | implementation-defined |

Rules:

- A strategy is executed only when its request template consumes the parameter. A `page:3` manifest without `{query:page}` stays one request and is reported as unsupported.
- A page returning zero new canonical URLs is a stop reason.
- The executor tracks a per-source request budget and stops on cancellation, result cap, repeated page fingerprint, or exhausted budget.
- First-page failure fails the source. Later-page failure returns collected records and marks the source partial.
- The same executor is used by `run` and `tests/live/harvest.rs` so live totals are comparable.

Cursor and fanout are separate adapters because they need response-driven state. Cursor support reads a configured path and substitutes either a token or a full next URL. Fanout support reads an id array, applies a bounded item template, and caps both requests and concurrency.

Alternative considered: one universal JSONPath loop. Rejected because most sources need no loop and the manifest contract should stay inspectable.

### Fetching is a pipeline of transport, policy, and parsing
The `Fetcher` trait stays responsible only for transport. Parsing stays in `sources::adapter` and `parsing::*`. Retry, pagination, and policy wrap the fetcher in the engine.

Transport decisions:

- Reuse one HTTP client per run to keep connection pools and cookies coherent.
- Send an explicit user agent derived from package name, version, and repository URL. Per-source overrides come from manifest headers.
- Randomize backoff jitter with a small deterministic test seam so tests can assert bounds while production jitter remains unpredictable.
- Parse Retry-After in seconds and HTTP-date form and clamp it to the configured maximum.
- Treat 429 and 5xx as retryable. Treat 403, 404, and parse failures as non-retryable unless the manifest opts in.
- Acquire permits per request, not once per source. Keep per-source and global semaphores independent.
- Support optional proxy and cookie inputs from config, never log their values, and resolve proxy hostnames through the existing SSRF guard.
- Keep the browser feature opt-in. The default build uses `rustls` for a small static binary. The `emulation` feature may use `wreq`/BoringSSL for TLS fingerprints. The `browser` feature may add Chromium. Neither feature is required for the plain path.

Alternative considered: retry inside each manifest adapter. Rejected because policy belongs in one place and adapters must stay network-free.

### Credentials are resolved at the run boundary
Credentials flow from, in increasing precedence, discovered file values, environment values, explicit config, and CLI overrides. Resolution produces `TemplateValues` keyed by source id. Values are wrapped in the existing redacting credential type. Doctor reports names and presence, never values. Config writing continues to refuse secrets.

Missing credentials produce a keyed outcome for that source. They do not abort unrelated sources, and they do not prevent doctor from running other checks.

### Records carry all parser-visible fields
`Record` gains optional posted timestamp, compensation, apply method, direct apply URL, company URL, industry, and remote flag. JSON aliases use manifest aliases first, then a documented default alias set. RSS reads `pubDate`, Atom reads `published` and `updated`, and HTML reads selectors where available.

The engine passes these fields into `JobInput`. `fetched_at` remains the run ingest time. `date_posted` remains the source posting time.

`external_id` becomes `source_id:source_native_id` when a stable id exists, otherwise the existing content hash. Multi-source merging keys by canonical URL and appends source ids in first-seen order.


### Challenge evidence is narrowed before health changes
A body containing the word `captcha` is not enough to classify a challenge. Detection requires HTML content, a WAF header, or a challenge-shaped document. JSON job payloads and ordinary HTML listings must parse first. This removes the suspected false positives for getonboard, gunio, and internshala before any source is relabelled.

### Versioning is explicit
Manifest version 1 keeps the legacy pagination string and remains valid. Manifest version 2 adds the structured pagination table, composite tenant seeds, and an explicit POST-retry opt-in. Output schema version 2 adds optional record fields and stable source identifiers. `read_versioned` accepts versions 1 and 2, and the migration notes live in `documentation/009-export-formats.md` and `documentation/016-config-migration.md`.

### Credential storage has one resolution path
A single secret store reads `SCRAPPY_<NAME>` first and an optional secrets file second. The config schema stores only the secrets-file path. Status, doctor, live harvest, and both run entry points call the same presence check, removing the current mismatch between bare names and the `SCRAPPY_` prefix. The detailed phase and gap review is recorded in `documentation/audits/oculus-hardening-plan-2026-11-27.md`.

### Source coverage uses a viability matrix
Each source receives one class and one action.

| Class | Action |
| --- | --- |
| public-working | keep, schedule re-probe |
| route-repair | fix manifest route and fixtures |
| new-adapter | add cursor, fanout, browser, or envelope support |
| credential-only | wire credential, skip without it |
| challenge-only | keep blocked, no bypass |
| dead | remove or mark blocked with a dated reason |

No source is labelled working from structural inspection alone. A working label requires a live probe that returns records through the manifest adapter. Repairs update manifest, fixtures, replay tests, behavior doc, and generated status together.

### Live verification is a separate opt-in suite
The live suite probes every eligible source, uses the real pagination executor, and writes a counts-only report. Report fields include source id, family, transport class, outcome, HTTP status, request count, record count, skipped count, email mention count, unique address count, bytes, latency, and stop reason. Aggregate fields include per-family totals, outcome totals, latency minimum, median, p95, maximum, and fastest and slowest working sources.

Alternative considered: reuse the normal run output. Rejected because run output intentionally excludes per-source failures and timing.

### Command, config, and documentation contracts stay stable
The command tree remains `run`, `validate`, `config` with its actions, `list-sources`, `doctor`, `completion`, and `version`. `--help` and `--version` work globally and per subcommand. stdout carries only result data. stderr carries diagnostics and progress events. Exit codes remain 0, 1, 2, 3, 4, and 5 with the published meanings.

The documentation layout remains `documentation/001-quickstart.md` through `015-security.md` plus `documentation/scrapers/<source_id>.md`. Pagination, retry, identity, and credential behavior go in `006-architecture.md`, `014-testing.md`, and a dedicated operations document. Each source behavior record keeps observed quirks, seed, route, break/fix history, and latest live evidence.

### Rust quality follows ms-rust
Every Rust change loads the ms-rust skill and only the applicable guideline files before editing. Public items use canonical documentation sections. Comments stay short and link to documentation. `cargo fmt`, `cargo clippy --all-targets -- -D warnings`, fixture replay, strict OpenSpec validation, secret checks, and PII checks are the baseline gate. Tests are integration tests under `tests/`, fixtures stay synthetic, and live tests stay ignored by default.

## Risks / Trade-offs

- [Pagination increases request volume and ban risk] -> Per-source budgets, governor permits, jitter, and empty-page stops. Default page counts remain manifest-declared.
- [Health labels become volatile as boards change] -> Date every live claim, keep generated status separate from live reports, and require re-probe before relabelling.
- [Credential plumbing can leak values] -> Central redacting type, doctor name-only output, no secret writes, and secret-leak tests.
- [Record enrichment can break golden fixtures] -> Migrate fixtures and replay expectations in the same change, and keep a focused parser test for every new field.
- [Cursor and fanout expand the manifest surface] -> Introduce them behind explicit strategies and validate unsupported forms before any request.
- [Browser escalation grows the binary] -> Keep browser and emulation behind opt-in features and verify default builds remain small.
- [Dead-source removal can hide regressions] -> Retire only after two dated probes and keep the reason in the behavior doc and inventory.

## Migration Plan

1. Add manifest fields and parser support behind defaults so existing manifests load unchanged.
2. Run fixture replay before any live probe. Fix regressions before changing health labels.
3. Enable pagination and credential resolution for one family at a time, starting with `feeds_api` and `ats`.
4. Re-probe every source and write the counts-only report.
5. Relabel health, update behavior docs, regenerate status, and remove only sources with two dated dead observations.
6. Roll back by reverting the change. Manifest additions are optional and existing fixtures remain valid.

## Open Questions

- Cursor and fanout grammar details can be finalized during the new-adapter phase because they do not change the first four phases.
- Browser escalation policy for challenge-only boards can be decided after the plain-path probe, because the specs keep bypass out of scope.
