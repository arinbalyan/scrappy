# Spec Delta

## Purpose

Define how each source is classified, repaired, seeded, or retired so health labels describe observed behavior.

## ADDED Requirements

### Requirement: Every source has a viability class
Each manifest SHALL be classified as public-working, credential-only, route-repair, new-adapter, challenge-only, or dead.

#### Scenario: Dead source
- **WHEN** a source has no reachable public route after documented probes
- **THEN** its manifest is marked blocked or removed from the registry with a recorded reason

### Requirement: Working means observed working
A source SHALL be labelled working only after a live probe returns records through the manifest adapter.

#### Scenario: Seed swaps break a source
- **WHEN** a working source later returns zero records after a seed or route change
- **THEN** its health label is downgraded before release

### Requirement: Seeds are verified before commit
A slug or tenant seed SHALL be live-probed before it is written to a manifest or fixture.

#### Scenario: Candidate seed
- **WHEN** a candidate seed returns non-job HTML or a redirect to a support page
- **THEN** the seed is not added to the manifest

### Requirement: Repairs update all contracts
A repaired source SHALL update its manifest, fixtures, replay tests, behavior document, and generated status together.

#### Scenario: Route repair
- **WHEN** a source route changes from JSON to RSS
- **THEN** the manifest parser, fixture body, golden records, and behavior doc change in the same commit

### Requirement: Unsupported adapters are explicit
A source that needs cursor, fanout, browser, or credential behavior outside the current adapter SHALL be recorded as new-adapter rather than silently failing.

#### Scenario: Browser-only board
- **WHEN** a board loads jobs only after client-side rendering
- **THEN** the source is marked browser-required and excluded from plain runs
