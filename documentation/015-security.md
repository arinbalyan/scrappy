# Security

Credentials are redacted by types in src/security.rs. Fetching validates
resolved addresses before connecting and caps response bodies. Fixtures are
scrubbed of personal data. The contract is in
openspec/changes/oculus-foundation/specs/security-privacy/spec.md.

src/engine/attention.rs builds a machine-readable challenge event when a
source is challenged. It carries the source id, challenge kind, status,
sanitized evidence, and an opaque resume token derived from the configuration
fingerprint in src/checkpoint/mod.rs. The token cannot be reused with a
different configuration, and the event pauses only the challenged source.
