# Security

The contract for this subsystem is in
openspec/changes/oculus-foundation/specs/security-privacy/spec.md.

## Redaction helpers

src/security/mod.rs provides the redaction and validation helpers. `Secret` is
a credential newtype built with `From<&str>` or `From<String>`. Its `Debug`
and `Display` implementations emit only `[redacted]`; `as_str()` exposes the
value only at the authentication boundary.

`redact_url` keeps the scheme, host, and path while replacing URL userinfo
with `[redacted]`, query text with `[query-redacted]`, and dropping fragments.
`redact_text` delegates to the challenge scrubber in src/engine/challenge.rs,
so long identifier-like tokens and query strings are masked and the result is
capped at that module's evidence limit.

## Outbound target checks

`validate_target` accepts an HTTP or HTTPS URL, parses userinfo and ports
including bracketed IPv6 hosts, and resolves hostnames with `ToSocketAddrs`.
It rejects literal or resolved addresses in private, loopback, link-local,
carrier-grade NAT, documentation, multicast, reserved, and unspecified ranges.
That includes IPv4 ranges such as 10/8, 172.16/12, 192.168/16, 127/8,
169.254/16, 0/8, 100.64/10, 192.0.0/24, 192.0.2/24, 198.18/15,
198.51.100/24, 203.0.113/24, 224/4, and 240/4, and IPv6 ranges such as ::1,
fc00::/7, fe80::/10, ::, 2001:db8::/32, and IPv4-mapped private addresses.
The function returns the validated addresses. Callers must connect only to
one of those addresses and must validate each redirect target separately.
DNS-dependent tests are ignored by default; literal cases run offline.
`resolve_public_host` is used by the HTTP clients at connection time so DNS
rebinding cannot replace a vetted hostname with a private address. Redirects
are handled manually and capped at ten hops; every hop and resolved location
is validated again. URLs are never rewritten to literal addresses, so TLS SNI
and certificate checks remain intact.

Response bodies are read in bounded chunks and rejected before they exceed
`MAX_BODY_BYTES` in src/fetching/mod.rs. A configured `memory_mb` ceiling is
checked before each source starts in src/engine/memory.rs using the process
resident size from `/proc/self/status`.

## Fixtures and challenge events

`scrub_fixture` replaces email-shaped strings, phone-number-like digit runs,
and long token-like strings with `[email]`, `[phone]`, and `[token]`. The
replacement is heuristic, so fixtures still need review before commit.

`scripts/check-fixture-pii.sh` is the offline fixture check. It scans every
file under `tests/fixtures/` for email addresses, phone-like numbers,
credential values, and long token-like strings, and prints the offending file
and matching line on failure. Recognized placeholders such as
`Bearer sample-token` are allowed. Run `sh scripts/check-fixture-pii.sh` from
the repository root, or `--self-test` to verify that injected synthetic PII is
detected; both run in CI. ISO dates, epoch timestamps, and ordinary job text
do not trigger the patterns. As of 2026-11-27 all committed fixtures are
synthetic, so no fixture files needed sanitizing.

Fetching-level body caps live in src/fetching/mod.rs, which bounds request
and response bodies in `Request` and `Response`. Challenge evidence
sanitization lives in src/engine/challenge.rs. src/engine/attention.rs builds
a machine-readable challenge event that carries the source id, challenge kind,
status, sanitized evidence, and an opaque resume token derived from the
configuration fingerprint in src/checkpoint/mod.rs. The token cannot be
reused with a different configuration, and the event pauses only the
challenged source.

## Diagnostic masking

src/config/secret.rs defines `Credential`, a wrapper around `Secret` whose
`Debug` and `Display` both emit `[redacted]`. Its `as_str` method is the only
read path and belongs at the authentication boundary. Credential names
recognized as `SCRAPPY_*_KEY` or `SCRAPPY_*_TOKEN` are loaded from the
environment into this type by `from_environment` and `environment_credentials`.

src/cli/diagnostic.rs and src/cli/trace.rs mask tracing fields by name.
Sensitive fields render as `[redacted]`; URL-shaped fields pass through
`redact_url`, which removes userinfo, query text, and fragments. Untrusted
free text passes through `safe_text`, so long token-like strings are masked
before they reach stderr or a doctor report.
