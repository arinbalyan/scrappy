# Testing

Normal tests are offline and replay fixtures through real parsers. Live
validation is opt-in and ignored by default. Before writing or reviewing any
.rs file, load .codex/skills/ms-rust/SKILL.md and only the relevant guideline
files in .codex/skills/ms-rust/. Add the compliance marker at the top of every
compliant Rust file. Run cargo fmt, cargo clippy --all-targets -- -D warnings,
and cargo doc --no-deps as the baseline gate. The parity check in
tests/docs_parity.rs verifies that documented paths exist.

The ignored suite in `tests/live/validate.rs` runs with
`cargo test --test live_validate -- --ignored --nocapture`. It fetches each
registered source through the real plain fetcher, applies the source's
configured rate through `src/engine/governor.rs`, and prints a report with
working, keyed, blocked, environment, stale, skipped, and unregistered
sections. Redirect stdout to a local file to keep the report out of version
control.

The `environment` section holds DNS and connection failures that describe the
probe host rather than a source. A restricted network can put every reachable
source there, so do not change health labels from that section alone.

Set `SCRAPPY_LIVE_LIMIT` to probe a prefix of the registry during triage.
Set `SCRAPPY_LIVE_ONLY` to a comma-separated list of source ids to probe only
those sources. For a slug-dependent source, set `SCRAPPY_LIVE_SEED_<SOURCE_ID>`
to a verified active slug, such as `SCRAPPY_LIVE_SEED_GREENHOUSE=gitlab`.
Buckets follow the observed request outcome. Returned records are working,
missing credentials or query values are keyed, 401, 403, 429, and 5xx are
blocked, and 404, empty responses, and parse failures are stale. Slug
sources without an active seed are reported under skipped. Progress lines
are written to stdout before the report marker so stdout alone is a clean
log. Each source acquires one governor permit, so one request per source
stays within its manifest `requests_per_minute` and `max_concurrency`.

The current registry has 111 manifests: 59 working, 22 requires_slug, 19 keyed,
7 blocked, and 4 stale health states. These are manifest-declared health
states, as listed in documentation/005-status.md, rather than live probe
outcomes.

A pre-repair historical run on 2026-11-27 took 151.26 seconds and found 24
working sources, 16 keyed sources, 23 blocked sources, 46 stale sources, and
20 unregistered historical sources. Its accepted report is kept outside
version control in /tmp/scrappy-live-report-accepted.txt. Because that run
was made before the later repairs, its counts do not describe the current
registry. The run environment also restricted some outbound hosts, so a 403,
429, or fetch failure in it may reflect network policy rather than the source.
Re-run from an unrestricted network before changing health for a challenge-only
failure. `academiccareers` and `higheredjobs` were later restored to working,
with their manifests, replay contracts, and behavior records updated together.

CI runs the baseline gate on Linux, macOS, and Windows. It also builds the
`emulation` and `browser` features, runs `cargo audit` and `cargo deny check`
from `deny.toml`, and builds the x86_64-unknown-linux-musl target. The static
job checks `file`, `ldd`, an executable smoke test, and a scratch-container
run. The audit check is verified against a controlled vulnerable fixture in
`documentation/audits/`; the fixture is never added to the main lockfile.

## Latest live sweep

The latest full sweep on 2026-11-27 produced 43 working, 16 keyed, 16 blocked,
6 environment, 30 stale, and 20 unregistered entries in the report sections
defined above. Representative working yields were `functionalworks` with 259
records, `hnalgolia` with 50, and `jobdataapi` with 20. These are live probe
outcomes; documentation/005-status.md holds declared manifest health and
availability and is not a copy of this report. An environment entry describes
the probe host, so it cannot be read as a source failure.

## Fixture contract

As of 2026-11-26, every fixture set in `tests/fixtures/<source_id>/` contains
six files: `request.json`, `request.template.json`, `response.body`,
`headers.json`, `frozen.json`, and `golden.ndjson`.

`request.template.json` holds `method`, a `url` template, optional `headers`,
and an optional `body`. Each `{name}` token in the URL, headers, or body is a
placeholder; doubled braces carry literal JSON braces. `frozen.json` holds
`fetched_at` and a `substitutions` object with string values that replace
those tokens. `request.json` is the frozen request produced by that
replacement. `headers.json` records the response headers and status,
`response.body` the saved body, and `golden.ndjson` the expected normalized
output.

`tests/fixture_contract.rs` checks the six files in every fixture set and
verifies that the remoteok request equals its rendered template.

src/checkpoint/mod.rs stores a versioned checkpoint with a fingerprint of the
resolved config, completed source ids, and emitted content hashes. Resume
rejects a checkpoint when its version or fingerprint no longer matches.
Checkpoints are written to a temporary file and renamed into place so an
interrupted save leaves no partial state.

## Live harvest

The ignored suite in `tests/live/harvest.rs` complements the validation sweep
with counts of records and email mentions. Run it with
`SCRAPPY_LIVE_REPORT=/tmp/scrappy-live-report.json cargo test --test live_harvest -- --ignored --nocapture`.
The report contains counts and domains only. It never writes addresses,
descriptions, or other job content to disk. Use `SCRAPPY_LIVE_ONLY` to select
comma-separated source ids and `SCRAPPY_LIVE_SEED_<SOURCE_ID>` for a verified
slug. A source that declares credentials needs the matching `SCRAPPY_*`
variable in the environment; without it the source is reported as keyed.

The 2026-11-27 full harvest probed all 111 sources and produced 6,778 records
across 54 sources. It observed 254 email mentions in 162 unique addresses over 138
domains, with 14 sources contributing addresses. The largest yields were `ats-workable` with 2,157 records,
`devopsjobs` with 1,000, `breezyhr` with 757, `arbeitnow` with 300, and
`ats-gem` with 300. The largest email contributors were `devopsjobs`
with 141 mentions, `remoteok` with 28, `weworkremotely` with 22, and
`realworkfromanywhere` with 21. Repeated trailing dots in domains such as
`camunda.com.` indicate a normalization improvement for a later change.
Outcome buckets were 54 working, 11 blocked, 19 keyed, 9 stale, 10 skipped, 5
environment, and 3 challenge. These are live outcomes; manifest health in
documentation/005-status.md is declared separately.

ATS tenant slug candidates are recorded in
`documentation/audits/ats-slug-candidates.md`. Candidates come from vendor
documentation and dated public references. Do not add a candidate to a
manifest until the project's own fetcher has probed it and the fixtures, replay
tests, behavior doc, and generated status are updated together.
