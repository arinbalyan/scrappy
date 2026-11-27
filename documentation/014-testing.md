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
working, keyed, blocked, stale, and unregistered sections. Redirect stdout
to a local file to keep the report out of version control.

CI runs the baseline gate on Linux, macOS, and Windows. It also builds the
`emulation` and `browser` features, runs `cargo audit` and `cargo deny check`
from `deny.toml`, and builds the x86_64-unknown-linux-musl target. The static
job checks `file`, `ldd`, an executable smoke test, and a scratch-container
run. The audit check is verified against a controlled vulnerable fixture in
`documentation/audits/`; the fixture is never added to the main lockfile.

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
