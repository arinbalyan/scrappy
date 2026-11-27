# Quality scoring

Deterministic scoring is implemented in src/output/record.rs. The pipeline in
src/engine/pipeline.rs enriches each record before it is emitted, so stdout
lines are final records. Company domains come from company_url alone, and an
extracted address cannot set or inflate that field. The contract is in
openspec/changes/oculus-foundation/specs/output-contract/spec.md.
