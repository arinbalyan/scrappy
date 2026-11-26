# Architecture

src/main.rs is a thin entry point. src/lib.rs exposes cli, config, sources,
fetching, parsing, engine, output, security, checkpoint, and metrics modules.
Each subsystem is a directory with a mod.rs root. src/config holds schema and
resolution in mod.rs and file editing in file.rs. src/sources holds the
manifest schema in manifest.rs, the loader in registry.rs, and manifests in
src/sources/manifests/<family>/. src/engine/contract.rs owns the runtime
bridge contract for streams, exit codes, and stdin run config. Integration
tests use directory targets under tests/cli_contract and tests/bridge_contract,
declared in Cargo.toml. Fixtures live in tests/fixtures/<source_id>/ and
behavior docs in documentation/scrapers/<source_id>.md. Empty directories keep
a short README. The canonical tree is recorded in
openspec/changes/oculus-foundation/design.md.

The engine reserves each request through src/engine/governor.rs. Every source
gets its own governor token bucket and tokio semaphore, so one source never
consumes another source's rate or concurrency budget. Rates come from manifest
limits and config overrides in requests per minute. Registering a source with
a zero rate or zero concurrency is rejected. The global kill switch stops all
sources, including requests already waiting on a rate or concurrency slot.
Kill-switch cancellation is checked before and after each wait, so no request
starts after the switch is activated.

Retries live in src/engine/retry.rs and apply only to idempotent requests.
Each attempt is classified as retryable, permanent, or throttled. Permanent
errors are never retried. Retryable errors wait with capped exponential
backoff plus jitter from a caller-supplied source. Throttled responses use a
parsed Retry-After delay when present, capped at the same maximum. A cancel
token interrupts any wait promptly, including an oversized Retry-After, and
no further attempt is started after cancellation.

Challenge detection is shared by the plain, emulated, and browser fetchers
through src/engine/challenge.rs. It classifies Cloudflare, `DataDome`,
captcha, and JavaScript-required pages, and records only a scrubbed evidence
sample with no query strings or long tokens. src/engine/breaker.rs keeps one
circuit breaker per source. Repeated challenge or failure responses open that
source only, and a successful response clears its failures. A cooldown allows
a stopped source to be tried again later.

Result caps live in src/engine/limits.rs. A source's effective cap is the
smaller of its per-source cap and the global cap, so a larger per-source cap
cannot raise the global cap. When fail-open is enabled, a failed source does
not stop the run; its sanitized diagnostic appears in the terminal summary
along with emitted, skipped, and resolved cap counts. Unknown sources still
fail the run.
