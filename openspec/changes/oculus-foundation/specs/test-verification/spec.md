# Spec Delta

## Purpose

Defines the fixture policy, offline test suite, live validation workflow, and supply-chain checks that keep the scraper reproducible and verify source health without making normal CI depend on the network.

## ADDED Requirements

### Requirement: Network-free normal test suite
Normal tests SHALL pass with networking disabled. Parsers SHALL be tested by replaying sanitized fixtures through the real parsing pipeline. A separate ignored suite MAY perform live-network checks.

#### Scenario: Offline test run passes
- **WHEN** cargo test runs with networking disabled
- **THEN** every non-ignored test passes

#### Scenario: Live tests are opt-in
- **WHEN** a user runs the normal test command
- **THEN** live-network tests are not executed

### Requirement: Fixture contract
Each source fixture set SHALL include a request descriptor, its request template, response body, selected headers, a frozen timestamp, and a golden normalized record. Fixtures SHALL be sanitized before commit.

#### Scenario: Fixture replays into a parser
- **WHEN** a fixture is loaded for a source
- **THEN** the parser produces the golden record

#### Scenario: Fixture metadata is complete
- **WHEN** a fixture set is validated
- **THEN** request descriptor, request template, response body, headers, timestamp, and golden record are present

### Requirement: CLI contract tests
CLI tests SHALL cover precedence for every configurable field, scalar and array config forms, explicit false overrides, per-source values, output formats, config writes, doctor parity, and invalid input exit codes.

#### Scenario: Full CLI contract suite passes
- **WHEN** the CLI contract suite runs
- **THEN** every precedence and format case passes

#### Scenario: Invalid input exits nonzero
- **WHEN** an invalid date or unknown source is passed
- **THEN** the CLI exits nonzero and prints a diagnostic

### Requirement: Live source validation
A separate validation workflow SHALL check which sources currently return jobs, which require API keys, which return challenges, and which are stale. It SHALL use its own rate limits and SHALL NOT run in normal CI.

#### Scenario: Health report distinguishes failures
- **WHEN** live validation finishes
- **THEN** the report separates working, keyed, blocked, environment, stale, and skipped live outcomes and lists historical IDs without manifests under unregistered

#### Scenario: Live validation respects rate limits
- **WHEN** live validation runs
- **THEN** requests per source stay within configured limits

### Requirement: Dependency and artifact auditing
The project SHALL commit Cargo.lock, build with locked dependencies, run dependency vulnerability and license audits, and run a static-binary smoke test for release artifacts.

#### Scenario: Locked build succeeds
- **WHEN** the project is built with locked dependencies
- **THEN** compilation succeeds using only committed versions

#### Scenario: Audit fails on vulnerable dependency
- **WHEN** a dependency has a known vulnerability
- **THEN** the audit check fails

#### Scenario: Release artifact is static
- **WHEN** the musl release binary is inspected
- **THEN** it has no dynamic executable dependencies and runs in a minimal container

### Requirement: Bridge contract tests
CLI contract tests SHALL cover the stdin run-config protocol, structured stderr progress events, the published exit-code taxonomy, streaming flush behavior, and cancellation record preservation. Golden fixtures SHALL cover email provenance, source attribution, and an authoritative company_domain that cannot be self-inflated by an extracted address.

#### Scenario: Bridge protocol passes end to end
- **WHEN** a test writes a JSON run-config to stdin
- **THEN** stdout contains only NDJSON records and stderr contains parseable terminal events

#### Scenario: Domain scoring fixture is adversarial
- **WHEN** a fixture contains an extracted address from an unrelated domain
- **THEN** the golden record keeps company_domain from company_url and withholds the domain-match score

### Requirement: Release artifact publication
The release workflow SHALL build and upload every artifact named by the installation documentation and SHALL smoke-test each artifact. Installers SHALL verify a checksum or signature for the downloaded artifact. Promotion failures SHALL produce a nonzero exit with a diagnostic.

#### Scenario: Every documented artifact exists
- **WHEN** a release is published
- **THEN** every platform archive named in installation documentation is attached and verified by a smoke test

#### Scenario: Installer rejects a tampered download
- **WHEN** a downloaded archive fails checksum or signature verification
- **THEN** the installer exits nonzero and installs nothing

### Requirement: Registry parity checks in CI
Normal CI SHALL verify that default source selection contains only registered manifests, each registered source has a fixture set and exactly one behavior document in documentation/scrapers/, and generated status documentation matches registry states. These checks SHALL run offline. CI does not verify a per-source contract test for every registered source or compare generated ATS slug counts with loaded manifests.

#### Scenario: Drift fails CI
- **WHEN** a manifest is registered without a fixture set or a status document is edited by hand
- **THEN** the parity check fails and names the drifted source or file

### Requirement: Canonical source tree
The repository SHALL keep the module, documentation, manifest, and test layout
recorded in design.md as the canonical tree. A parity check SHALL fail when a
required path from that tree is missing or when a module is placed outside its
canonical directory. Empty directories SHALL keep a short README so the layout
is visible in version control.

#### Scenario: Canonical tree is present
- **WHEN** the parity check runs on a clean tree
- **THEN** every module, manifest family directory, documentation file, and test directory from design.md exists

#### Scenario: Layout drift fails CI
- **WHEN** a required directory or module file is removed
- **THEN** the parity check fails and names the missing path

#### Scenario: Fixture template is reusable
- **WHEN** a source fixture is replayed
- **THEN** request.json is derived from request.template.json and the frozen timestamp is recorded

### Requirement: Canonical fixture files
Every fixture set SHALL include `request.json`, `request.template.json`,
`response.body`, `headers.json`, `frozen.json`, and `golden.ndjson`.

#### Scenario: Fixture files are complete
- **WHEN** a fixture set is validated
- **THEN** all six named files are present

### Requirement: Documentation parity
The repository SHALL keep a `documentation/` folder with a document for the CLI, configuration, architecture, sources, testing, and security. Every documented source path SHALL be relative to the scrappy repository root and SHALL exist. Source comments SHALL remain short; long explanations and runbooks SHALL live in `documentation/`.

#### Scenario: Style gate passes
- **WHEN** normal CI runs on a clean tree with valid documentation paths
- **THEN** the documentation parity check succeeds

#### Scenario: Documentation drift fails CI
- **WHEN** a document names a source path that does not exist
- **THEN** the documentation parity check fails and names the document and missing path

### Requirement: Rust style gate
Normal CI SHALL run `cargo fmt --check`, `cargo clippy --all-targets -- -D warnings`, and `cargo doc --no-deps`. Rust work SHALL follow the project-local ms-rust skill at `.codex/skills/ms-rust/SKILL.md`.

#### Scenario: Style gate passes
- **WHEN** normal CI runs on a clean tree
- **THEN** formatting, clippy with warnings denied, and documentation generation all succeed

#### Scenario: Rust skill is present
- **WHEN** a contributor starts work on a Rust file
- **THEN** `.codex/skills/ms-rust/SKILL.md` and its referenced guideline files are present in the repository
