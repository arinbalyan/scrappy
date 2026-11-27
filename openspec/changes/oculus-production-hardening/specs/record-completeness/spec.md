# Spec Delta

## Purpose

Define the job fields that must survive parsing so downstream automation can rank, deduplicate, and apply without re-scraping.

## ADDED Requirements

### Requirement: Ingest and posting times are distinct
Every emitted record SHALL carry `fetched_at` as the run ingest timestamp and SHALL carry `date_posted` when the source supplies a posting time.

#### Scenario: Source supplies a posting date
- **WHEN** a response contains a recognized posting-date field
- **THEN** the emitted record has a non-empty `date_posted` separate from `fetched_at`

#### Scenario: Source has no posting date
- **WHEN** no recognized posting-date field exists
- **THEN** `date_posted` is null and `fetched_at` is still populated

### Requirement: Application and company metadata are preserved
A record SHALL retain salary or compensation text, apply instructions, direct apply URL, company URL and domain, industry, location, and remote status when the source provides them.

#### Scenario: Salary and direct apply URL
- **WHEN** a response supplies salary and apply URL fields
- **THEN** the emitted record carries both values

### Requirement: External identifiers are source-native when available
`external_id` SHALL use the source job identifier when one exists, and SHALL fall back to a content hash only when no stable source identifier is present.

#### Scenario: Source identifier present
- **WHEN** a response item supplies a stable job id
- **THEN** `external_id` contains the source id and the job id

### Requirement: Multi-source records merge attribution
When the same canonical job arrives from multiple sources, the emitted record SHALL retain every contributing source id in stable order.

#### Scenario: Duplicate across sources
- **WHEN** two sources return the same canonical URL
- **THEN** one record is emitted with both source ids and no duplicate output line
