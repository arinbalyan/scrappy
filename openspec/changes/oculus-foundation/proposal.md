# Proposal

## Why

The existing Go scraper grew from a small script into a 62,000-line program with no clear end state. Its CLI and config handling are the worst parts: documented precedence is not implemented, scalar config values silently erase defaults, boolean flags cannot be turned off, saved config destroys per-site settings, and stdout can contain warnings mixed into JSON. Now the desired end state is known, so a clean-room Rust rebuild can make the binary, config contract, and source adapters correct from the start.

This is a clean-room rebuild on the `oculus` branch, which preserves the history of `main` with all inherited files removed. The old implementation on `main` is read-only evidence of behavior. JobHunter, job-ops, and JobRadar are read-only sibling references. No source, fixtures, docs, prompts, config text, or branding may be copied. The new project will use an MIT-style license; only the repository owner can make that relicensing decision.

## What Changes

- Create a small Rust command-line binary with explicit subcommands and a minimal default feature set.
- Introduce a typed layered configuration contract with real precedence, tri-state booleans, and source-aware validation.
- Define source manifests with stable IDs, selectors, rate and concurrency limits, API-key requirements, robots policy, attribution, and fixture references.
- Separate fetching from parsing behind a fetcher abstraction with plain HTTP, browser-emulation HTTP, and opt-in browser paths.
- Emit machine-readable NDJSON on stdout and keep all diagnostics on stderr.
- Add content-hash deduplication, canonical URL normalization, checkpoint/resume, and deterministic fixture-driven tests.
- Add structural secret redaction, DNS-resolving SSRF protection, challenge detection, bounded retries, and per-source rate limits.
- Add source-health reporting that distinguishes working, keyed, blocked, stale, and unregistered sources.
- Support a non-interactive stdin run-config protocol with NDJSON records on stdout, structured progress events on stderr, and a published exit-code taxonomy so downstream automation such as JobRadar can consume the binary directly.
- Emit enriched records with contact provenance, source attribution, quality score, and authoritative company domain before streaming, so a downstream send policy can route work between autopilot and attention lanes.
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

The bridge and enrichment fields exist to serve JobRadar's automation-first workflow. JobRadar remains a read-only sibling; this change defines the scrappy side of the contract only. No email sending, application automation, or LLM behavior is added here.

### Modified Capabilities

- None. The branch starts with no existing specs.

## Impact

The work creates the first implementation files on `oculus`: Rust crate scaffolding, `.gitignore`, `LICENSE`, `NOTICE`, source manifests, fixtures, tests, and CI configuration. The sibling projects and old `main` remain untouched. External dependencies include clap, serde, toml, figment, tracing, secrecy, tokio, governor, scraper, reqwest with rustls, wreq with pinned emulation profiles, and an optional Chromium-based browser backend.
