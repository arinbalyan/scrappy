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

For feeds_api sources, src/sources/adapter.rs joins manifest.rs with fetching
and parsing. A caller supplies runtime substitutions (slug, named credential,
or query value), and build_request expands placeholders in the manifest
`request` URL, header values, and body into a validated fetching::Request. A
literal brace is written twice in a template, so `{{` becomes `{` and `}}`
becomes `}` after substitution. This keeps JSON request bodies declarative.
The adapter defaults `accept` to JSON for JSON sources, RSS media types for
RSS sources, and HTML media types for HTML sources. A manifest header with
the same name overrides the default. The caller fetches that
request, then parse_response turns the bounded fetching::Response into
parsing::Record values. The adapter performs no network I/O and never places
credential values in error messages. Manifest fields request_template,
field_aliases, selectors, and parser_options are optional; older manifests
without them continue to load. tests/source_adapters.rs covers this path
offline.

Two parser options cover common feed shapes. default_company_path reads a
company name from the envelope when job objects omit it, as in public Workable
widget JSON. creator reads dc:creator from each RSS item as a company fallback;
a company prefix in the title wins. CDATA values and names ending at an HTML
entity, such as `AT&T`, are supported. Bare JSON arrays use the same
default_company_path aliases through field aliases. Configurable selectors may
match the item node itself, which keeps a linked card and its URL on the same
element.

JSON records no longer need a company field to be kept. When no configured
alias or default_company_path value resolves, the parser uses "unknown" as the
company instead of rejecting the record. Embedded JSON records follow the
same rule.

tests/docs_parity.rs checks the canonical tree from design.md: crate metadata,
the skill file, all 16 numbered documents, subsystem modules, five manifest
families, directory test targets, and each registered source's fixture set and
behavior document. A fixture set includes request.json, request.template.json,
response.body, headers.json, frozen.json, and golden.ndjson. A missing
manifest family directory fails the parity check.

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
