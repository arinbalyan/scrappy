# Spec Delta

## Purpose

Define the opt-in live measurement that proves coverage, yield, and latency across the full registry.

## ADDED Requirements

### Requirement: Verification covers the full registry
An opt-in live run SHALL probe every registered source that has enough substitutions and credentials to build a request.

#### Scenario: Full registry
- **WHEN** the live suite runs without a source filter
- **THEN** every eligible manifest is attempted and every ineligible source is reported with a reason

### Requirement: Verification follows pagination
A live probe SHALL execute the same pagination strategy as a normal run.

#### Scenario: Paged source
- **WHEN** a source declares a page or offset strategy
- **THEN** the probe reports total records and the number of requests used

### Requirement: Reports contain counts only
A live report SHALL contain source ids, outcomes, status codes, record counts, email counts, domains, bytes, and latency, and SHALL NOT contain addresses, descriptions, or raw bodies.

#### Scenario: Report written
- **WHEN** a live run completes
- **THEN** the JSON report is writable to a local path and contains no personal data

### Requirement: Statistics support comparison
A report SHALL include per-family totals, outcome totals, record totals, latency median, p95, minimum, and maximum, and the fastest and slowest working sources.

#### Scenario: Summary review
- **WHEN** the report is generated
- **THEN** a reader can compare families and identify slow sources without re-running the probe
