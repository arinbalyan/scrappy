# Changelog

All notable changes to scrappy are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Added

- Memory budget throttling with configurable `--memory-cap` flag
- Memory monitor goroutine with 10s interval and 80% cap warning
- `sync.Pool` for `strings.Builder` in `StripHTML` (GC pressure reduction)
- Pre-allocation of result slices with `make(..., ResultsWanted)`
- On-the-fly dedup using `seenGlobal` map (replaced intermediate `processedBySite`)
- LinkedIn rotation: permutes `IsRemote`, `HoursOld`, `JobType` (up to 12 passes)
- Browser fallback hardening: retry, URL validation, better error messages
- Salary normalization module at `internal/normalize/salary.go`
- Per-site result caps via `--site-results-wanted` flag
- CSV emails-only export via `--csv-emails-only` flag
- `EnforceAnnualSalary` flag wired through CLI to engine
- Structured logging with `resHeader()` (elapsed, goroutines, RSS)
- Log level CLI flag `--log-level`
- Email extractor with strict validation, obfuscation detection, HTML entity decoding, blocked-domain list, MX verification
- Company-page enricher producing jobs with `source: "company_page"`
- `ExtractFromHTML` called on raw HTML before `StripHTML` (captures mailto: hrefs)
- TLD validation map (80+ common TLDs) for email validation
- Parallel MX verification with `--verify-concurrency` flag (default 5)
- YCJobs scraper rewritten to use Algolia API queries (index `WaaSPublicCompanyJob`)
- MkDocs configuration with 164+ sites organized by category
- Custom LICENSE file (personal, non-commercial use only)
- README badges for CI, Go version, license, stars, forks
- All branches (dev, beta, main) synchronized via merge commits
- Unified site implementation guidelines across all scrapers
- Fail-fast DNS/NXDOMAIN detection for dead boards
- 429 fail-fast logic (max 2 attempts)
- Timeout configuration per-site basis
- MX verification concurrency control
- CONTRIBUTING.md with code style, scraper conventions, testing guidelines
- CODE_OF_CONDUCT.md (Contributor Covenant v2.0)
- SECURITY.md with vulnerability reporting policy
- Issue templates for bug reports, feature requests, new boards, performance issues
- PR review workflow with label requirements, size labeling, title validation
- Stale issue management workflow (60d issues, 30d PRs)
- Dependabot configuration for Go modules, GitHub Actions, and Docker
- Makefile with targets for build, test, vet, lint, fuzz, bench, docker, release
- PR template with checklist for contributors
- Release drafter for automatic changelog generation
- Manual release workflow with version bump selection (patch/minor/major)

### Fixed

- Memory leak: `gc_cycles` now reads actual `/gc/cycles/automatic:gc-cycle` from runtime/metrics instead of hardcoded 0
- Memory throttle: `waitForMemoryBudget` re-reads heap after `runtime.GC()` instead of using stale pre-GC value
- Memory pressure: forced `runtime.GC()` when heap exceeds 80% of cap
- Eager trim of `all[]` slice at 2x `ResultsWanted` with forced GC
- Dribbble removed (design portfolio, not a job board - HTTP 405)
- Jobspresso search terms broadened from AI/ML only to general tech roles
- LinkedIn scraper rate limit handling
- Email validation false positives with TLD whitelist
- Dice enrichment with proportional detail-fetch cap and shared rate limiter

### Changed

- YCJobs from HTML parsing to Algolia API direct queries
- Email extraction pipeline: three-stage (raw HTML -> stripped text -> company page)
- MX verification from sequential to parallel (semaphore-bounded goroutine pool)
- Debug logging now includes resource headers

### Removed

- Dribbble scraper and all related code (model type, engine registration, config, docs)
- Stale MIT license reference from README

### Security

- Gitleaks scan in CI pipeline
- SECURITY.md with vulnerability disclosure process
- Sensitive config files saved with 0600 permissions

---

## [0.1.26] - 2026-06-11

### Added

- Dependabot configuration for automated dependency updates
- PR review workflow with title validation and label requirements
- Stale issue management workflow
- CONTRIBUTING.md with comprehensive developer guide
- CODE_OF_CONDUCT.md (Contributor Covenant v2.0)
- SECURITY.md with vulnerability reporting policy
- Issue templates for bug reports, feature requests, and new boards
- Makefile with build, test, lint, fuzz, and release targets
- EditorConfig and Dockerignore for consistent developer experience
- GitHub Sponsors funding configuration

### Fixed

- Dribbble board removed (confirmed design portfolio, not a job board)
- Jobspresso search terms broadened to return meaningful results
- Memory leak: GC cycle monitoring now reads actual runtime metrics
- Memory throttle now correctly re-checks heap after GC
- Eager trimming of result buffer at 2x target prevents 2GB heap growth
- Dependabot PRs now target `dev` branch instead of `main`
- CI release workflow handles duplicate tag pushes gracefully

### Changed

- Release workflow separated from CI (manual trigger with version bump selection)
- Release drafter auto-generates changelogs from conventional commit PR titles

---

## [0.1.21] - 2026-05-28

### Added

- Memory budget throttling with `--memory-cap` flag
- Salary normalization module
- Per-site result caps
- CSV emails-only export
- `EnforceAnnualSalary` flag
- Structured logging with resource headers
- Log level CLI flag
- Email extractor with MX verification
- Company-page enricher
- TLD validation map for email extraction
- Parallel MX verification
- YCJobs Algolia API scraper
- MkDocs documentation with 164+ sites

### Fixed

- LinkedIn rate limit handling with rotation strategy
- Browser fallback hardening for anti-bot sites
- Email extraction false positives with TLD whitelist
- Dice enrichment with proportional detail-fetch cap

---

## [0.1.0] - 2026-05-20

### Added

- Initial release with 100+ job board scrapers
- Linkedin, Indeed, Google Jobs, and ATS suite support
- Email enrichment from descriptions and company pages
- Quality scoring (deterministic, 0-100)
- Multiple export formats: JSONL, CSV, XLSX, Parquet
- Proxy support with SOCKS5/HTTP health checks
- Browser fallback via Playwright
- Interactive setup wizard
- Docker deployment with docker-compose
- CLI with Cobra command framework
