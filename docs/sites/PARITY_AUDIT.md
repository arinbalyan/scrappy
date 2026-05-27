# Python Parity Audit (file-by-file baseline)

Reference source: `tmp/upstream/jobspy/*`

## Implemented parity
- `indeed/__init__.py` -> cursor GraphQL search + filter composition + compensation mapping
- `linkedin/__init__.py` -> guest endpoint search params, pagination cap behavior, optional detail fetch
- Site set implemented: LinkedIn, Indeed, Google, ZipRecruiter, Wellfound, RemoteOK, Remotive, BuiltIn
- Export paths + schema fields wired through `model.JobPost`

## Partial parity / hardening in progress
- Full rich parsing depth for non-Indee/LinkedIn sites (currently lighter)
- Full country matrix behavior for Indeed by domain mapping
- Full upstream Python project-equivalent field population for all site-specific optional fields
- TLS-fingerprint equivalence (Go stdlib transport hardened, but not identical to python tls-client behavior)

## Smart upgrades over upstream Python project
- Shared client-level retry/backoff + UA rotation + cookie reset cadence in `internal/util/http.go`
- Constraint evaluation layer in `pkg/scrappy/constraints.go` with explicit CLI warnings
- Reusable engine API (`pkg/scrappy`) for orchestration/AI post-processing hooks

## Next hardening checkpoints
1. Replace regex-first scrapers with API-first/site-embedded-JSON-first extraction where available.
2. Add per-site fixture corpus from real captured responses and regression tests.
3. Add adaptive pacing based on status-code telemetry per host.
4. Add stronger proxy orchestration (health decay/rehabilitation windows, sticky sessions).
