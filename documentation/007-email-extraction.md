# Email extraction

Email extraction is implemented in the enrichment stage of src/engine.rs.
Provenance is required for every emitted address. Guessed addresses and
commit-harvested addresses are disabled by default, as specified in
openspec/changes/oculus-foundation/specs/security-privacy/spec.md.
