# Spec Delta

## Purpose

Defines the normalized job record, serialization formats, integrity fields, schema versioning, and safe artifact writing so downstream automation can consume results without ad hoc parsing.

## ADDED Requirements

### Requirement: Normalized job record
Every emitted job SHALL contain a schema version, source identifier, canonical URL, title, company, location, description, fetched timestamp, and content hash. Required fields SHALL be validated before emission.

#### Scenario: Required field missing
- **WHEN** a parsed job lacks a required field
- **THEN** the record is rejected and a parse-failure metric is recorded

#### Scenario: Record has provenance
- **WHEN** a job is emitted
- **THEN** its source identifier and fetched timestamp are present

### Requirement: NDJSON serialization
The default output format SHALL be newline-delimited JSON with one object per line. JSON and CSV formats SHALL be available through explicit selection. Output SHALL be valid for the selected format in every destination.

#### Scenario: Default output is NDJSON
- **WHEN** a user runs a scrape without a format flag
- **THEN** stdout contains one JSON object per line

#### Scenario: CSV escapes special characters
- **WHEN** a field contains a comma, quote, or newline
- **THEN** the CSV output escapes that field correctly

### Requirement: Schema versioning and migration
The output schema SHALL carry a version. Breaking changes SHALL increment the version and include migration notes. Older versions SHALL remain readable for a documented period.

#### Scenario: Version is emitted
- **WHEN** any result is written
- **THEN** every record includes the output schema version

#### Scenario: Unknown schema version is rejected
- **WHEN** a consumer reads a record with an unsupported version
- **THEN** the reader reports a clear version error

### Requirement: Safe artifact writes
Output files SHALL be written atomically through a temporary file and rename. Files containing personal data SHALL use mode 0600. A failed write SHALL NOT leave a partial final artifact.

#### Scenario: Interrupted write leaves no partial file
- **WHEN** a write fails before rename
- **THEN** the final output path is unchanged

#### Scenario: Personal data file is private
- **WHEN** an output file containing job-seeker data is written
- **THEN** its mode is 0600

### Requirement: Integrity of saved artifacts
Saved result artifacts SHALL include a content hash and a manifest that allows verification of completeness. Checkpoint and result files SHALL be usable only when their hash matches.

#### Scenario: Corrupted artifact is detected
- **WHEN** a saved result file is modified after writing
- **THEN** verification fails and names the artifact

#### Scenario: Complete artifact verifies
- **WHEN** a saved artifact is unchanged
- **THEN** verification succeeds using the recorded hash

### Requirement: Downstream automation fields
Every emitted record SHALL include external_id, canonical_url, is_remote, date_posted, compensation, apply_method, job_url_direct, company_url, company_domain, company_description, industry, quality_score, source_ids, and an emails array. The company_domain SHALL be canonicalized from source identity or company_url before any address is extracted.

#### Scenario: Required automation fields are present
- **WHEN** a job is emitted
- **THEN** its record contains every downstream automation field named in this requirement

#### Scenario: Guessed address is tagged
- **WHEN** an address was inferred rather than observed
- **THEN** the email entry marks provenance as inferred and source as guessed

#### Scenario: Company domain is authoritative
- **WHEN** an extracted email has a domain unrelated to the company_url
- **THEN** company_domain remains derived from company_url and quality_score does not gain a domain-match point from the extracted address

### Requirement: Email provenance entries
Each email entry SHALL contain addr, verified, source, role, and provenance. Source SHALL be one of description, company_page, mailto, direct, or guessed. Provenance SHALL distinguish observed addresses from inferred ones.

#### Scenario: Observed address is tagged
- **WHEN** an address is found directly in a job description
- **THEN** its email entry marks source as description and provenance as observed

### Requirement: Verified streaming output
Each stdout line SHALL be flushed immediately and SHALL be a complete JSON object. Cancellation or abnormal exit SHALL NOT emit a partial line or withdraw records already emitted. A terminal progress event SHALL report the final emitted count.

#### Scenario: Consumer reads while the run proceeds
- **WHEN** a caller reads stdout before the process exits
- **THEN** complete record lines are available without waiting for process termination

#### Scenario: Cancellation preserves emitted records
- **WHEN** a run is cancelled after records were emitted
- **THEN** stdout remains parseable NDJSON and contains every record emitted before cancellation

### Requirement: Explicit dedup policy
URL and content-hash deduplication SHALL be an explicit option with a documented default. Title-and-company deduplication SHALL run only when separately requested and SHALL NOT be implied by the URL dedup option. Disabling dedup SHALL preserve all distinct records, including records that share title and company.

#### Scenario: URL dedup disabled preserves distinct records
- **WHEN** URL dedup is disabled and two records share title and company
- **THEN** both records are emitted

#### Scenario: Company dedup is opt-in
- **WHEN** URL dedup is enabled and company dedup is not requested
- **THEN** records with different canonical URLs are retained

### Requirement: Consumer-ready lightweight formats
NDJSON SHALL remain the default stdout format. JSON and CSV SHALL be selectable
for file artifacts through explicit output flags. CSV SHALL use a header row
with the same field names as the normalized record and SHALL be readable by
standard spreadsheet and CSV tools. The binary SHALL expose no product-specific
output mode, so a downstream consumer can adopt new fields without a scraper
change.

#### Scenario: CSV has a stable header
- **WHEN** a user writes CSV output
- **THEN** the first line names every exported field in a documented order

#### Scenario: Records remain process-consumable
- **WHEN** a consumer reads NDJSON from stdout while the run is active
- **THEN** each line is a complete normalized record with stable field names
