# Export formats

src/output/mod.rs writes NDJSON by default, with JSON and CSV available explicitly.
CSV is a first-class format for spreadsheet and automation consumers. The
record fields are specified in
openspec/specs/output-contract/spec.md.

src/output/record.rs builds the normalized job. Every job carries
schema_version 1, source_id, canonical_url, fetched_at, a content hash, and
the downstream automation fields, including emails with source and
provenance. company_domain comes only from company_url, and quality_score is
deterministic from present fields and observed addresses. A guessed address
cannot set the domain or earn the domain-match point.

src/output/record.rs also reads serialized jobs and rejects records whose
schema_version is unknown before any other field is interpreted.

src/output/writer.rs encodes NDJSON, JSON, and CSV from the same normalized
jobs. CSV uses a fixed column order and quotes fields containing commas,
quotes, or newlines. write_line emits one complete NDJSON line and flushes it
so a downstream consumer can read records while the run is still active.

CSV_COLUMNS fixes the header order as schema_version, source_id, external_id,
canonical_url, title, company, location, description, fetched_at, content_hash,
is_remote, date_posted, compensation, apply_method, job_url_direct, company_url,
company_domain, company_description, industry, quality_score, source_ids, and
emails. The CSV integration test in tests/csv_output.rs checks that order and
the escaping of commas, double quotes, and newlines. NDJSON keeps field
consistency with those CSV columns: each line is one complete normalized record
with the same field names, and neither format adds format-specific fields.

Saved artifacts use a private temporary file, mode 0600, and a rename, so a
failed write leaves the final path unchanged. Each artifact gets a manifest
with its size and hash. Reading the manifest and comparing the hash detects a
result file changed after writing.

src/output/stream.rs writes one complete NDJSON line and flushes it for each
job. A cancellation after earlier emits leaves every emitted line parseable.
Title-and-company deduplication lives there as a separate opt-in function, so
URL dedup alone keeps records with different canonical URLs.

## Schema version 2

Output schema version 2 adds `source_job_id`, `salary_min`, `salary_max`, `salary_currency`, `salary_period`, and `apply_route`. `external_id` uses `<source_id>:<native id>` when a native id exists and keeps the content-hash fallback otherwise. Readers accept versions 1 and 2. `fetched_at` is the ingest timestamp and `date_posted` is the source posting time.
