# Export formats

src/output.rs writes NDJSON by default, with JSON and CSV available explicitly.
CSV is a first-class format for spreadsheet and automation consumers. The
record fields are specified in
openspec/changes/oculus-foundation/specs/output-contract/spec.md.

src/output/record.rs builds the normalized job. Every job carries
schema_version 1, source_id, canonical_url, fetched_at, a content hash, and
the downstream automation fields, including emails with source and
provenance. company_domain comes only from company_url, and quality_score is
deterministic from present fields and observed addresses. A guessed address
cannot set the domain or earn the domain-match point.

src/output/record.rs also reads serialized jobs and rejects records whose
schema_version is unknown before any other field is interpreted.
