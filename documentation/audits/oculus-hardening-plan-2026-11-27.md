# Oculus hardening plan 2026-11-27

Status: planning only. Baseline branch `oculus`, commit `aa2976d`. No Rust, manifest, fixture, spec, or test file was changed. Inputs read: the five 2026-11-27 audits, `src/engine/*`, `src/sources/adapter.rs`, `src/sources/manifest.rs`, `src/main.rs`, `tests/live/harvest.rs`, and `openspec/changes/oculus-foundation`.

## 0. Baseline and evidence

| Area | State at `aa2976d` |
| --- | --- |
| Registry | 111 manifests. feeds_api 58, ats 26, html_static 13, keyed 14. |
| Declared health | working 73, keyed 19, blocked 7, requires_slug 8, stale 4. |
| Latest live probe | 54 working, 6,778 records, 19 keyed, 11 blocked, 9 stale, 10 skipped, 5 environment, 3 challenge. Family yields: feeds_api 35/2,661, ats 14/3,993, html_static 5/124, keyed 0/0. |
| Pagination | 15 manifests declare a strategy. 5 execute today (arbeitnow, himalayas, internshala, adzuna, careerjet). 10 fall back to one request. |
| Query defaults | Declared in 24 manifests and merged in `run_source`. |
| Credentials | 19 manifests declare names. No CLI path puts values into `RunInput.values`. |
| Retry | 429 and 5xx enter the retry path. Jitter is `attempt` milliseconds. `Retry-After` is never read. POST is retried. `record_success` runs before the status check. |
| Transport | `PlainFetcher` builds a client per request. No default user agent, cookie jar, or proxy wiring. |
| Records | `external_id` is a content hash, `source_ids` is always one entry, salary is free text. |
| Live harvest | One page per source through its own probe loop. No percentiles or family totals. |

Three evidence conflicts drive the ordering below.

1. Declared working (73) exceeds verified working (54). The probe predates the verified ATS seed edits, so both numbers must be re-measured in one run before relabeling.
2. The challenge detector scans every body for `captcha`. Live rows for getonboard, gunio, and internshala show challenge evidence that begins with JSON or HTML job content, so these are likely false positives from page text.
3. `status.rs` reads the bare credential name from the environment while `secret::from_environment` reads `SCRAPPY_<NAME>`, so listing and live harvesting disagree about keyed readiness.

## 1. Scope boundaries and non-goals

In scope: credential resolution, pagination, retry and failure classification, client identity and concurrency, record completeness, source repair and adapter gaps, live verification reporting, migration rules, and OpenSpec packaging.

Out of scope, with reasons: CAPTCHA solving, WAF bypass, stealth plugins, and cookie replay from blocked sessions (policy and the foundation spec forbid them). Credential provisioning for vendor, affiliate, or account-issued keys (requires accounts this repository cannot create). Tenant discovery for Workday, UKG, ADP, and SuccessFactors at scale (requires a customer list or search service). Email verification, sending, and application automation (outside the proposal). Paid proxy pool procurement (only wiring is in scope).

Constraints: offline tests by default, live suites remain `#[ignore]`, no secret value in config, fixtures, reports, or diagnostics, and no new top-level directory.

## 2. Prioritized phases

The order is P0, P1, P2, P3, P4, P5, P6, P7. Each phase ends with its acceptance gate and a green `cargo test`. P2 and P3 both edit the pipeline loop, so run them sequentially. P6 route repairs and dead-source relabels may start after P0, but adapters wait for P2 and P3.

### P0 Evidence, classification, and instrumentation

- Goal: make failures trustworthy and give later phases counters to report.
- Changes: add `SourceReport` counters (requests, pages, retries, raw records, emitted, skipped, bytes, latency, stop reason, partial flag) to `src/engine/contract.rs` and populate from `src/engine/pipeline.rs`. Make challenge detection content-type aware in `src/engine/challenge.rs`, restricting body-marker scans to HTML or known WAF headers. Replace `robots_allows` in `src/engine/pipeline.rs` with a leading-token parser (`allow`, `permit`, `deny`, `disallow`) and reject unknown text during manifest validation.
- Depends: none. Conflicts: none, but P1 reuses the same report struct.
- Acceptance: fixture replay output is byte-identical. getonboard, gunio, and internshala classify as parsed records when the body is JSON or HTML job content without WAF headers. An unknown robots string fails validation.
- Tests: unit tables in `challenge.rs`, `pipeline.rs`, and `manifest.rs` using the three existing response shapes. Mock fetcher asserts counters.
- Rollback: counters are additive. Revert the detector guard if it hides a real challenge, and keep the evidence row in the live report.

### P1 Credential resolution

- Goal: make the 19 keyed sources runnable without putting values in config.
- Changes: add `SecretStore` to `src/config/secret.rs` (environment plus optional secrets file, `missing(names)`, redacted report). Add `secrets_file` to `src/config/mod.rs` and `CONFIG_SCHEMA`. Replace `config_values` in `src/main.rs` with `runtime_values(registry, &store)` used by both run paths. In `src/engine/pipeline.rs`, skip a manifest with missing declared credentials before building a request and return a `keyed` outcome. Point `src/sources/status.rs`, `src/cli/doctor.rs`, and `tests/live/harvest.rs` at the same store.
- Depends: P0. Conflicts: `status.rs` and `doctor.rs` are also touched by P6 relabels, so land P1 first.
- Acceptance: with a populated store each keyed manifest builds its request and sends exactly one request. With an empty store each reports `missing_credentials` with zero requests. No value appears in stdout, stderr, progress events, or the harvest report.
- Tests: store unit tests (env precedence, quoted and commented file entries, missing names), a child-process test for env isolation, and redaction assertions in `tests/config_secret.rs`. Fixture replays inject `TemplateValues` directly.
- Rollback: keep the injected `RunInput.values` path and disable the loader. Config keys never hold values, so rollback cannot leak secrets.

### P2 Pagination grammar and executors

- Goal: make all four modes executable with bounded work and honest failures.
- Changes: replace the string-only model in `src/engine/pagination.rs` with an untagged `PaginationSpec` (legacy string or `[pagination]` table) and a `PlanState` that yields the next request descriptor, its parameter values, and its stop reason. Extend `src/sources/manifest.rs` validation. Rewrite the loop in `src/engine/pipeline.rs` to acquire a permit per request, track seen page keys and cursors, and return a partial outcome. Migrate the 15 manifests. Add `page={query:page}` to builtin, `pn` to simplyhired, `offset` to ats-smartrecruiters and freelancercom, and the item endpoint to hackernews.
- Depends: P0 and P1 for adzuna, careerjet, and findwork. Conflicts: P3 and P4 both edit this loop, so expose `fetch_page` as a helper here.
- Acceptance: each mode's stop reason is reachable and recorded. A later-page failure returns earlier records with `partial = true` and exit code 3. A cap is checked before the next request. A repeated cursor stops with `RepeatedPage`. Cancellation stops within one backoff.
- Tests: unit tests for parsing and each stop reason, a mock fetcher serving three pages for page, offset, cursor, and fanout, a cap test that asserts the request count, a second-page 500 test, and a repeated-cursor test.
- Rollback: set `pagination = "none"` per manifest. The executor stays available and legacy strings keep parsing.

### P3 Retry and failure classification

- Goal: retry the right things with real delays and stop retrying the wrong things.
- Changes: add `fetch_with_retry` in `src/engine/retry.rs` that classifies a `Response` (status, headers, body) before returning, so `Retry-After` survives. Add equal jitter with an injectable entropy closure for async. Parse `Retry-After` seconds and IMF-fixdate relative to an injected clock. Classify 408, 425, and 429 as throttled, 5xx as retryable, other 4xx as permanent, and DNS or TLS errors as permanent. Retry GET, HEAD, and OPTIONS by default, with a manifest `retry_post` opt-in. Move `record_success` after the status check and open the breaker on repeated throttles in `src/engine/breaker.rs`.
- Depends: P2 helper. Conflicts: pipeline loop and breaker.
- Acceptance: a 429 with `Retry-After: 2` waits at least two seconds. A 500 then 200 succeeds once. A 404 makes one attempt. A POST makes one attempt without opt-in. Three consecutive throttles open the breaker.
- Tests: retry unit tests with fixed jitter and clock, a mock fetcher sequence table, a cancellation-during-delay test, and breaker tests.
- Rollback: keep the old classifier behind a constant and revert the pipeline call. Timeout caps remain enforced.

### P4 Client identity, concurrency, proxies, cookies

- Goal: reuse connections, present a consistent identity, and bound in-flight work.
- Changes: hold one `reqwest::Client` per run in `src/fetching/plain.rs`, add the `cookies` feature and a per-source jar, wire `ProxySelector` into plain and emulation paths through `FetchOptions` in `src/fetching/mod.rs`, and add `proxies` plus request budgets to `src/config/mod.rs`. In `src/engine/governor.rs`, add a global semaphore (default 4) and per-host buckets keyed by request host, with a fixed acquire order: host rate, source rate, global permit.
- Depends: P2 and P3. Conflicts: latency changes can look like source failures, so measure after retry stabilizes.
- Acceptance: one client per run with keep-alive observed by a local server. Two sources on one host share a host bucket. In-flight requests never exceed the global cap. A rejected proxy is not reused. HTML sources send a pinned Chrome identity, JSON APIs send `scrappy/<version>`, and a manifest header wins. A profile plus a manifest user agent fails validation.
- Tests: local HTTP server tests for reuse, cookies, proxy rejection, and concurrency, plus governor timing tests.
- Rollback: feature-gate client reuse and proxy wiring. Per-source jars are dropped with the run.

### P5 Record completeness and schema v2

- Goal: stable identity, merged attribution, and typed pay and apply data.
- Changes: extend `src/parsing/json.rs` for native ids, numeric and nested salary, and new aliases. Resolve relative record URLs against the response URL in `src/engine/pipeline.rs`. Merge duplicates in `src/engine/dedup.rs` (append `source_ids`, coalesce missing fields). Extend `src/output/record.rs` with `source_job_id`, structured salary, apply enum, and the v2 bump. Apply `Config.is_remote` as a tri-state filter. Add selector fields for location, description, and date only where a fixture proves the markup.
- Depends: P0 and P2. Conflicts: schema bump touches every golden contract test, so keep it one commit.
- Acceptance: `external_id` uses the native id when present and a url-title-company hash otherwise. Two sources with one URL yield one record with two `source_ids`. A relative href is resolved. Numeric salary becomes min, max, currency, and period strings with no inferred currency. A job-board `company_url` never sets `company_domain`.
- Tests: adversarial fixtures for domain scoring, relative URL, duplicate merge, numeric salary, and is_remote filtering. `read_versioned` accepts v1 and v2.
- Rollback: revert the version constant and the struct fields together. Readers remain compatible.

### P6 Source viability and adapters

- Goal: align health labels with evidence and close the highest-value source gaps.
- Changes: section 3 lists every source. Order: (a) relabel dead and challenge-only sources and regenerate `documentation/005-status.md`; (b) repair public routes and seeds (ats-trakstar, manatal, ats-rippling, jobindex, djinni, themuse, nofluffjobs, functionalworks); (c) add adapters for Workday, UKG, iCIMS, and Jobvite, which need composite tenant seeds, POST search, HTML or XML parsers, and pagination; (d) defer SuccessFactors, Phenom, ADP, TalentLyft, Recruiterflow, Loxo, and ycjobs with written reasons in behavior docs.
- Depends: P1, P2, P3, P4. Conflicts: each relabel must update the manifest, behavior doc, fixtures, and generated status in one commit or parity tests fail.
- Acceptance: every changed label has a dated behavior-doc entry and a live row in the verification report. New adapters pass fixture replay and a capped live probe. Removed sources have manifest, fixture set, and behavior doc removed together.
- Tests: existing tier replay plus one new replay per adapter. Parity tests in `tests/source_registry_contract.rs` and `tests/docs_parity.rs`.
- Rollback: restore the manifest and doc in one revert. Adapter code stays behind its parser kind.

### P7 Live verification, reporting, and harvest parity

- Goal: one report that measures the real pipeline and supports relabeling decisions.
- Changes: rewrite `tests/live/harvest.rs` to call `pipeline::run` with a report sink instead of its own single-page probe. Extend `tests/live/validate.rs` to emit the matrix. Add family totals and percentiles. Add a `tests/report_contract.rs` target with a mock fetcher asserting harvest and CLI counts are equal.
- Depends: P0 through P6. Conflicts: none, but the report schema freezes after P7.
- Acceptance: for one mock registry the CLI and harvest produce identical emitted, skipped, raw, request, and page counts. A keyed source with no credential records zero requests. No address appears in the report.
- Tests: report contract tests, percentile unit tests with known samples, and a dry run over the fixture registry.
- Rollback: keep the old probe behind a flag for one release and compare counts before deleting it.

## 3. Source viability matrix

The 54 verified working sources need no relabel. Pagination and credentials add yield to arbeitnow, himalayas, internshala, adzuna, careerjet, and the keyed group. The 57 non-working entries split as follows.

| Class | Sources | Recommendation |
| --- | --- | --- |
| Credential-only (19) | adzuna, authenticjobs, careerjet, comeet, crelate, deel, dice, exa, findwork, fountain, freshteam, hiringthing, infojobs, ismartrecruit, jazzhr, jobtechdev, talroo, usajobs, web3career | Wire credentials in P1, then verify. Remove exa (search API, not a job board). infojobs needs an OAuth adapter. web3career also needs a route repair. usajobs conflicts with the no-email-user-agent rule, so decide that policy before enabling. |
| Route repair (11) | themuse, djinni, jobindex, headhunter, echojobs, functionalworks, nofluffjobs, getonboard, gunio, internshala, simplyhired | Repair the manifest route or parser. simplyhired is already fixed by query defaults. getonboard, gunio, and internshala need the P0 false-positive check first. echojobs needs Retry-After and a reprobe. nofluffjobs needs the body cap or pagination. functionalworks needs charset or compression handling. headhunter needs a clean-egress reprobe before any challenge label. |
| New adapter (13) | adp, ats-phenom, ats-recruiterflow, ats-successfactors, ats-talentlyft, ats-ukg, ats-workday, icims, jobvite, loxo, manatal, ats-rippling, ycjobs | Build in P6 order: manatal and ats-rippling first (route plus parser), then icims (HTML), jobvite (XML), Workday and UKG (composite seeds, POST). Mark blocked until then. Defer adp, ats-successfactors, ats-phenom, ats-talentlyft, ats-recruiterflow, loxo, and ycjobs with reasons. |
| Challenge-only (7) | crunchboard, hiringcafe, icrunchdata, remotefirstjobs, snagajob, startupjobs, wuzzuf | Mark blocked and exclude from default selection. No bypass. Browser escalation only with explicit human approval and a terms check. |
| Engine gap (1) | hackernews | Implement fanout in P2. Add a literal default company so items without `company` still map. If real items still lack a usable company, mark stale. |
| Environment (3) | canadajobbank, careeronestop, joinrise | Reprobe from a stable egress. No manifest change. Record the environment reason in the report. |
| Dead (3) | androidjobs, eurojobs, drupaljobs | Remove from registry, with fixture set and behavior doc removed in the same commit. drupaljobs shut down in September 2026. |

Data-quality suspects that remain labeled working: jobsinjapan (blog feed in the taxonomy audit, records in the live probe), jobsch (422 in the taxonomy audit), and ats-trakstar (seed chargebee last posted in 2014). Verify semantic fields before the final relabel.

## 4. Pagination design

| Mode | Canonical form | Stop conditions |
| --- | --- | --- |
| page | `page:<param>:<start>:<stop>` | last page, empty page, short page, total reached, record cap, request budget |
| offset | `offset:<param>:<start>:<step>:<max_pages>` | empty or short page, total reached, cap, budget |
| cursor | `cursor:<param>:<path>` or `cursor:url:<path>` | absent or repeated cursor, cap, budget |
| fanout | `fanout:<batch>:<max_ids>:<id_path>:<item_url>` | id list exhausted, cap, budget |

Legacy strings map as follows: arbeitnow `page:page:1:3`, simplyhired `page:pn:1:3`, adzuna `page:page:1:20`, careerjet `page:page:1:10`, internshala `page:page:1:8`, builtin `page:page:0:6` after adding the page query, himalayas `offset:offset:0:20:10`, ats-smartrecruiters `offset:offset:0:100:until_short` after templating the offset, freelancercom `offset:offset:0:50` only if the API honors it, reed `page:pageno:1:until_empty`, jobsdb and jobstreet `page:page:1:until_empty`, findwork `cursor:url:next`, hackernews `fanout:10:45::<base>/item/{id}.json`.

Budgets and failures: default 25 requests per source, hard cap 1000 from the plan, and an optional run-wide budget from config. Check the cap against deduplicated jobs before the next request. A first-page transport, status, parse, or challenge failure fails the source. A later-page failure stops the plan, returns earlier records, and records `partial` with the page index. A challenge on any page produces attention. Keep the last URL or cursor and a small seen-set to stop `RepeatedPage`. Emit one progress event per page with page index, request count, new records, cumulative records, and stop reason. Acquire and release the governor permit per request.

Harvest parity: pagination state and counters live in the pipeline, so the live report becomes a projection of the run summary. The harvest test must not build requests itself.

## 5. Ban-avoidance design

Use equal jitter, `delay/2 + rand(0..=delay/2)`, seeded from `RandomState` and injected in tests. Honor `Retry-After` as seconds or IMF-fixdate, clamped to `max_delay` and the remaining budget, with at most 250 ms added jitter. Retry only idempotent methods, with `retry_post` as the explicit opt-in. Reuse one client per run for connection pooling and TLS sessions. Keep one cookie jar per source, enable cookies only for HTML and emulation sources, and clear the jar at source end. Send `scrappy/<version>` for JSON and RSS APIs and the pinned Chrome profile identity for HTML sources. A manifest header wins over the default, a profile and a manifest user agent cannot both be set, and email-style user agents stay forbidden. Wire `ProxySelector` into plain and emulation transports, select per request, reject a proxy after a connect or timeout failure, and redact userinfo. Bound in-flight work with the global semaphore, per-source semaphore, and per-host bucket, acquired in the order host rate, source rate, global permit. Parse robots as a leading token and validate unknown text at load. No CAPTCHA solving, no stealth plugin, no cookie replay from a blocked session, and no browser escalation without a documented human decision.

## 6. Credential plumbing design

One `SecretStore` resolves declared names from `SCRAPPY_<NAME>` first and an optional secrets file second. The file uses real quoting, comments, and `export` syntax and accepts bare names, normalized to the manifest name. Config stores only `secrets_file`, never a value, and `config set` rejects credential-shaped keys. `runtime_values` builds one `TemplateValues` per selected manifest and is shared by both run entry points. The pipeline checks all declared names before `build_request` and returns `keyed` with zero requests when any is missing. Listing, doctor, and harvest call the same `missing(names)` helper. Diagnostics name the variable, never the value. Rotation is not supported, one value per name.

## 7. Data completeness design

| Field | Rule |
| --- | --- |
| `source_job_id` | Native id from `id`, `jobId`, `ref`, `requisitionId`, or `jobReqId`, first match wins. |
| `external_id` | `<source_id>:<native id>` when present, else `<source_id>:<hash16>` over canonical URL, title, and company. Description is excluded so edits do not churn identity. |
| `source_ids` | Merge on duplicate canonical URL within a run. Keep first-seen order, keep the first record's values, and coalesce missing fields from later duplicates. |
| Salary | Keep `compensation` text. Add `salary_min`, `salary_max` as decimal strings, `salary_currency` ISO 4217 or null, and `salary_period` (`year`, `month`, `day`, `hour`, `unknown`). Parse numbers, numeric strings, and nested `{from,to,currency,type}`. Never infer currency. |
| Apply | `apply_method` becomes `url`, `email`, `ats`, or `unknown`. `job_url_direct` requires an absolute http or https URL. |
| Company domain | Derive from `company_url` or an explicit payload domain only. A denylist of ATS and job-board hosts (greenhouse.io, lever.co, ashbyhq.com, workable.com, myworkdayjobs.com, icims.com, jobvite.com, rippling.com, ultipro.com, successfactors.com, careers-page.com, and peers) forces null. |
| Posted time | Normalize RFC 2822, RFC 3339, and date-only forms to ISO 8601. Keep date-only values date-only. Invalid values become null. |
| URLs | Resolve relative HTML hrefs against the response URL before `from_record`. |
| Remote | Apply `Config.is_remote` as a filter that drops only known contradictions and keeps unknown values. |

Schema version 2 carries all new optional fields. `read_versioned` accepts 1 and 2. Goldens compare only url, title, company, location, and description, so they stay valid unless those change.

## 8. Live verification matrix and report fields

Source row: id, family, declared health, credential state, seed state, outcome (`working`, `partial`, `keyed`, `blocked`, `challenge`, `stale`, `skipped`, `environment`), http status, pages, requests, retries, raw records, emitted, skipped invalid, skipped duplicate, stop reason, bytes, elapsed ms, first-record ms, challenge kind, detail, verified_at.

Run block: generated_at, report version, run id, registry fingerprint, probe settings (timeout, concurrency, page budget, user agent, proxy enabled), and counts per outcome.

Family block: sources, eligible, working, partial, keyed, blocked, challenge, stale, skipped, environment, requests, records, records per working source, emails observed, unique domains.

Percentiles over per-source elapsed ms and records per working source: p50, p90, p95, p99, max, with sample size. Use nearest-rank, `ceil(p*n)-1`, and mark percentiles indicative when n is below 20. Define eligible as registered minus keyed, skipped, and environment. Keep addresses out, cap domains at 20, and sort rows by family then id.

## 9. Migration and compatibility rules

Manifest version 2 is required for the `[pagination]` table, composite tenant seeds, and `retry_post`. Version 1 keeps the legacy string and stays valid. Mixed forms fail validation with a message naming the manifest. Fixtures keep the six-file contract. A request change updates `request.json` and `request.template.json` together. Goldens change only when url, title, company, location, or description change. Each relabel updates the manifest, behavior doc, and generated status in one commit. Removed sources remove all three artifacts. Output schema bumps to 2 once, with a migration note in `documentation/009-export-formats.md` and `documentation/016-config-migration.md`. `external_id` stays opaque to consumers and is not parsed by field position.

## 10. Gap review

| Gap | Why it is uncovered |
| --- | --- |
| WAF and CAPTCHA work | Forbidden by policy and the foundation spec. Challenge-only sources stay blocked. |
| robots.txt fetching | The manifest policy is the contract. Fetching robots.txt adds a network dependency and a second parser, deferred. |
| Credential provisioning | Vendor, affiliate, and account keys need external accounts. Only resolution and wiring are covered. |
| Tenant discovery at scale | Workday, UKG, ADP, and SuccessFactors need customer lists or search. Adapters accept explicit tuples only. |
| Browser escalation for challenge sites | Needs a human approval and terms check per host. The plan covers the decision gate, not execution. |
| Cross-source parallelism | Source-level sequencing stays. Only fanout workers and per-request limits are covered. Deterministic output ordering would need buffering. |
| Page-level checkpoint resume | Existing checkpoints track sources and hashes. Page cursors are not persisted, so a resumed run refetches the plan. |
| Proxy pool operation | Wiring and rejection are covered. Buying, rotating, and monitoring a pool is operational. |
| JSON-LD parser | `src/parsing/jsonld.rs` is unreachable. Wiring it is cheap but no current manifest needs it, so it is deferred. |
| HTML selector breadth | 52 sources lack fixture evidence for date, salary, and description. Adding selectors without fixtures would be unverifiable. |
| Salary conversion | Only normalization to decimal strings and ISO codes. FX conversion and annualization are out of scope. |
| USAJobs user-agent exception | The vendor expects a contact email in the agent while the spec forbids it. Needs a policy decision. |
| Live probe environment bias | DNS, TLS, 403, and 429 failures can be local. The report separates `environment`, and relabels require a second clean-egress run. |
| Percentile reliability | Small samples make p95 and p99 unstable. The report includes n and flags low-n results. |

## 11. Recommended OpenSpec change set

Archive `oculus-foundation` first. It is complete but unarchived, and new deltas would otherwise merge against unarchived capability text.

| Order | Change | Capabilities | Tasks |
| --- | --- | --- | --- |
| 1 | `oculus-credential-resolution` | new `credential-resolution`, MODIFIED `cli-config` | store, config key, runtime wiring, pipeline skip, status and doctor parity, redaction tests |
| 2 | `oculus-pagination-and-fetch-policy` | MODIFIED `scrape-engine`, MODIFIED `security-privacy` | grammar and validation, page and offset, cursor and fanout, budgets and partials, retry classification, jitter and Retry-After, concurrency and host buckets, client reuse and identity, proxy and cookie wiring, robots tokens |
| 3 | `oculus-record-completeness` | MODIFIED `output-contract` | native id and external_id, source_ids merge, salary, apply, company domain denylist, posted normalization, relative URLs, is_remote filter, schema v2 |
| 4 | `oculus-source-viability-and-verification` | MODIFIED `source-registry`, MODIFIED `test-verification` | relabels and removals, route repairs, adapter order, live matrix and percentiles, harvest parity, status regeneration, doc parity |

Run these changes sequentially. Changes 2 and 3 both touch `scrape-engine` and `output-contract` semantics, and changes 1 and 4 both touch `source-registry`, so parallel application invites delta conflicts. If parallelism is required, assign one owner per capability file and merge in the order above.
