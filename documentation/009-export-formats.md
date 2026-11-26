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

src/output/writer.rs encodes NDJSON, JSON, and CSV from the same normalized
jobs. CSV uses a fixed column order and quotes fields containing commas,
quotes, or newlines. write_line emits one complete NDJSON line and flushes it
so a downstream consumer can read records while the run is still active.

Saved artifacts use a private temporary file, mode 0600, and a rename, so a
failed write leaves the final path unchanged. Each artifact gets a manifest
with its size and hash. Reading the manifest and comparing the hash detects a
result file changed after writing.

src/output/stream.rs writes one complete NDJSON line and flushes it for each
job. A cancellation after earlier emits leaves every emitted line parseable.
Title-and-company deduplication lives there as a separate opt-in function, so
URL dedup alone keeps records with different canonical URLs.
