# Proposal

## Why

The existing Go scraper grew from a small script into a 62,000-line program with no clear end state. Its CLI and config handling are the worst parts: documented precedence is not implemented, scalar config values silently erase defaults, boolean flags cannot be turned off, saved config destroys per-site settings, and stdout can contain warnings mixed into JSON. Now the desired end state is known, so a clean-room Rust rebuild can make the binary, config contract, and source adapters correct from the start.

The audit findings that drive this change are recorded here so the new spec can be verified against them. The old CLI exposed `scrape`, `doctor`, and `setup`. Its documented precedence was not implemented, and the setup command wrote unquoted values, mis-parsed inline comments, and produced invalid proxy entries. Scalar TOML values could erase defaults, and boolean flags could not be explicitly turned off. Config writes destroyed per-site sections and credentials were written into config files. stdout mixed warnings into JSON output because plain output was an indented JSON array and JSONL went only to a file with `--out`. The old rate flags were concurrency caps while the real rate limiter existed only in tests. Doctor derived checks inconsistently, counted failed fixes as success, did not exercise every configured proxy, and could hide unrelated checks when an optional key was missing. There was no `--version`, no shell completion, no config inspection command, no schema documentation command, and no preflight validation. Only 128 of 141 declared sites were registered, status documents disagreed, and 13 declared sites had no implementation. Streaming on main emitted before enrichment, scoring, dedup, and source attribution; the dev branch fixed this but was never promoted. There was no public hook to register a custom scraper, no NDJSON-on-stdout mode, no toggle for expensive email enrichment, no way to distinguish a healthy zero-result site from a blocked one, no Go test CI, and a license that blocks commercial use.

This is a clean-room rebuild on the `oculus` branch, which preserves the history of `main` with all inherited files removed. The old implementation on `main` is read-only evidence of behavior. Other repositories in the parent workspace are read-only references. No source, fixtures, docs, prompts, config text, or branding may be copied. The new project will use an MIT-style license; only the repository owner can make that relicensing decision.

## What Changes

- Create a small Rust command-line binary with explicit subcommands, global `--help` and `--version`, shell completion, and a minimal default feature set.
- Introduce a typed layered configuration contract with real precedence, tri-state booleans, scalar-or-list values, source-aware validation, a generated config template, inspectable resolved values, and lossless writes.
- Provide `doctor` as a first-class diagnostic command with named checks, safe automatic fixes, proxy verification, JSON reporting, and an explicit fix mode.
- Define source manifests with stable IDs, selectors, rate and concurrency limits, API-key requirements, robots policy, attribution, and fixture references.
- Allow users to add custom sources through local manifests without rebuilding the binary or forking the repository.
- Keep a per-source behavior document for every scraper so quirks, breakages, and fixes are recorded beside the source identifier.
- Separate fetching from parsing behind a fetcher abstraction with plain HTTP, browser-emulation HTTP, and opt-in browser paths.
- Emit machine-readable NDJSON on stdout and keep all diagnostics on stderr.
- Add content-hash deduplication, canonical URL normalization, checkpoint/resume, and deterministic fixture-driven tests.
- Add structural secret redaction, DNS-resolving SSRF protection, challenge detection, bounded retries, and per-source rate limits.
- Add source-health reporting that distinguishes working, keyed, blocked, and stale sources, and report historical IDs without manifests under unregistered in the live validation audit.
- Support a non-interactive stdin run-config protocol with NDJSON records on stdout, structured progress events on stderr, and a published exit-code taxonomy so downstream automation can consume the binary directly.
- Emit enriched records with contact provenance, source attribution, quality score, and authoritative company domain before streaming, so a downstream send policy can route work between automated and attention lanes.
- Publish and verify installable release artifacts for every platform named in the installation documentation.
- Establish MIT licensing, NOTICE provenance, locked dependency builds, and security audits as implementation tasks.
- **BREAKING**: This branch replaces the Go binary and config format; it does not preserve the old CLI flags or TOML layout verbatim.

## Capabilities

### New Capabilities

- `cli-config`: CLI surface, config loading, precedence, validation, output streams, exit codes, and doctor diagnostics.
- `source-registry`: source manifest schema, stable IDs, key requirements, health states, and port-order metadata.
- `scrape-engine`: fetching abstraction, parsing pipeline, rate limits, retries, challenge handling, deduplication, and resume behavior.
- `output-contract`: normalized job records, NDJSON serialization, hashes, schema versioning, and safe artifact writing.
- `security-privacy`: secret handling, redaction, SSRF protection, browser isolation, PII minimization, and CI leak checks.
- `test-verification`: fixture policy, network-free tests, live validation workflow, and dependency auditing.

The bridge and enrichment fields exist to serve automation-first consumers. This change defines the scrappy side of the contract only. No email sending, application automation, or LLM behavior is added here.

### Modified Capabilities

- None. The branch starts with no existing specs.

## Impact

The work creates the first implementation files on `oculus`: Rust crate scaffolding, `.gitignore`, `LICENSE`, `NOTICE`, source manifests, fixtures, tests, and CI configuration. The other parent-workspace projects and old `main` remain untouched. External dependencies include clap, serde, toml, figment, tracing, secrecy, tokio, governor, scraper, reqwest with rustls, wreq with pinned emulation profiles, and an optional Chromium-based browser backend.
