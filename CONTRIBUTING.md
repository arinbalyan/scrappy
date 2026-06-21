# Contributing to scrappy

Thank you for your interest in contributing to **scrappy**! This document covers guidelines for bug reports, feature requests, code changes, and pull requests.

> **License**: This project is for **personal, non-commercial use only**. By contributing, you agree that your contributions will be licensed under the same terms.

---

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [How Can I Contribute?](#how-can-i-contribute)
  - [Reporting Bugs](#reporting-bugs)
  - [Requesting a New Job Board](#requesting-a-new-job-board)
  - [Suggesting Enhancements](#suggesting-enhancements)
  - [Fixing a Bug or Adding a Feature](#fixing-a-bug-or-adding-a-feature)
- [Development Setup](#development-setup)
- [Code Style & Conventions](#code-style--conventions)
  - [Go](#go)
  - [Scraper Implementation](#scraper-implementation)
  - [Testing](#testing)
- [Pull Request Process](#pull-request-process)
- [Branch Strategy](#branch-strategy)
- [Release Process](#release-process)
- [Getting Help](#getting-help)

---

## Code of Conduct

This project follows a **be excellent to each other** policy. We expect all contributors to:

- Be respectful and constructive in discussions
- Focus on the code, not the person
- Accept feedback gracefully and offer feedback kindly
- Assume good faith

## How Can I Contribute?

### Reporting Bugs

Before filing a bug, please:

1. **Search existing issues** — your bug may already be reported
2. **Check the latest release** — it may already be fixed
3. **Include logs** — run with `--log-level DEBUG` and paste the relevant output

Open a [bug report](https://github.com/arinbalyan/scrappy/issues/new?template=bug_report.md) with:

- **Scrappy version** (`scrappy --version`)
- **Command line** you ran (with search terms, flags, etc.)
- **Expected behavior** vs **actual behavior**
- **Full error output** (with `--log-level DEBUG` if possible)
- **Environment**: OS, Go version, whether you're running from source or binary

### Requesting a New Job Board

To add a new job board:

1. Open a [feature request](https://github.com/arinbalyan/scrappy/issues/new?template=feature_request.md) with the board's URL and any known API details
2. If you're comfortable writing Go code, see [Scraper Implementation](#scraper-implementation) below
3. Check the [board coverage docs](https://arinbalyan.github.io/scrappy/sites/) to see if similar boards already exist

### Suggesting Enhancements

Open a [feature request](https://github.com/arinbalyan/scrappy/issues/new?template=feature_request.md) with:

- **The problem** you're trying to solve
- **Why** the current approach doesn't work for you
- **A sketch** of the solution you have in mind (optional)

### Fixing a Bug or Adding a Feature

1. **Comment on the issue** you want to work on (or open one if none exists) to avoid duplicate effort
2. **Fork the repo** and branch from `main`
3. **Write the code** following the [conventions below](#code-style--conventions)
4. **Include tests** — every scraper needs a contract test; every utility needs unit tests
5. **Run the full test suite** before submitting (`go test ./... && go vet ./...`)
6. **Open a pull request** against the `main` branch
7. **After merge**, your feature branch is deleted automatically

---

## Development Setup

### Prerequisites

- **Go 1.26+** — the project uses Go 1.26 features (`sync.Map`, `iter`, etc.)
- **Make** (optional, for convenience targets)
- **Playwright** (optional, only for browser-fallback scrapers like monster)

### Getting started

```bash
# Clone the repo
git clone https://github.com/arinbalyan/scrappy.git
cd scrappy

# Default branch is main; no need to switch
git checkout main

# Build
go build ./cmd/scrappy/

# Quick smoke test
./scrappy --email --results-wanted 10 --site hackernews
```

### Project structure

```
.
├── cmd/scrappy/            # CLI entry point (cobra commands)
├── config/                 # Company slugs, per-site config
│   └── company_slugs.yaml
├── config.yaml             # Main configuration (site search terms, locations)
├── docs/                   # MkDocs documentation
│   ├── sites/              # Per-scraper documentation
│   └── *.md
├── internal/               # Internal packages
│   ├── browser/            # Playwright browser automation
│   ├── doctor/             # Diagnostics subsystem
│   ├── email/              # Email extraction and MX verification
│   ├── export/             # Output formats (JSONL, CSV, XLSX, Parquet)
│   ├── model/              # Shared types (JobPost, Site, Location)
│   ├── normalize/          # Salary normalization
│   ├── scraper/            # All scraper implementations (one subdirectory per board)
│   │   ├── adsite/         # Example: one scraper per board
│   │   ├── linkedin/
│   │   ├── ...
│   │   └── ats/            # Shared ATS board infrastructure
│   └── util/               # HTTP client, logging, text helpers
├── pkg/scrappy/            # Public API (engine, types)
│   ├── engine.go           # Core orchestration, concurrency, memory management
│   ├── engine_test.go
│   └── types.go
├── tests/                  # External test packages
│   ├── email/
│   ├── export/
│   ├── normalize/
│   └── scraper/            # Contract tests per scraper
├── AGENTS.md               # AI agent context (internal use)
└── mkdocs.yml              # Documentation config
```

---

## Code Style & Conventions

### Go

- **Format**: `gofumpt` or `go fmt` — we don't enforce a strict style, but keep it clean
- **Imports**: standard library first, then third-party, then internal — separated by blank lines
- **Error handling**: wrap errors with context using `fmt.Errorf("context: %w", err)`
- **Naming**: follow Go conventions (camelCase, acronyms all-caps: `HTTP`, `URL`, `ID`)
- **Context**: pass `context.Context` as the first parameter to any blocking function
- **No panics**: handle errors explicitly; panic only in `init()` or `main()` for fatal setup

### Scraper Implementation

Every scraper lives in `internal/scraper/<sitename>/` and must implement the `Scraper` interface:

```go
type Scraper interface {
    SiteName() model.Site
    Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error)
}
```

**Conventions**:

- **Package name**: lowercase, no hyphens (e.g., `linkedin`, `indeed`, `hackernews`)
- **New function**: `func New(client *http.Client) *Scraper` — if `client` is nil, create a default one
- **Site name constant**: add `SiteFoo Site = "foo"` in `internal/model/types.go`
- **Config entry**: add a `foo:` section in `config.yaml` with search terms and location
- **Registration**: add `fooscraper.New(nil)` to the scraper list in `pkg/scrappy/engine.go`
- **Contract test**: add a test in `tests/scraper/foo/contract_test.go` that mocks HTTP responses
- **Documentation**: add `docs/sites/foo.md` with endpoint details and maintenance notes
- **HTTP retries**: create the client with `util.NewHTTPClient(util.ClientOptions{Retries: 2, Timeout: 20 * time.Second})`
- **Rate limiting**: if the board has known rate limits, use a `time.Ticker` shared across API and detail fetches
- **Content type**: set `Accept` and `Content-Type` headers explicitly per endpoint

**Parsing order** (preference):

1. JSON API → 2. JSON-LD (structured data) → 3. `__NEXT_DATA__` (Next.js SSR) → 4. HTML regex

### Testing

- **Every scraper needs a contract test** with a realistic HTML/JSON mock
- **Tests live in `tests/`** (external test package to avoid import cycles)
- **Use `httptest.NewServer`** for mocking endpoints
- **Don't hit real endpoints** in CI — mock everything
- **Race tests**: run `go test -race ./...` before pushing

---

## Pull Request Process

1. **Branch from `main`**, not `dev` or `beta`
2. **Use a descriptive branch name** (one branch per issue or per logical change):
   - `feat/<scope>/<short-name>` — new feature (e.g. `feat/email/multi-page-crawl`)
   - `fix/<scope>/<short-name>` — bug fix
   - `docs/<scope>/<short-name>` — documentation
   - `perf/<scope>/<short-name>` — performance improvement
   - `refactor/<scope>/<short-name>` — refactoring
   - `chore/<scope>/<short-name>` — tooling, CI, etc.
3. **Keep PRs focused** — one logical change per PR
4. **Write a clear title and description** explaining what and why
5. **Reference related issues** with `Closes #123` or `Relates to #456` so the issue auto-closes on merge
6. **Pass all CI checks** — build, test, race, vet, gitleaks
7. **Await review** — a maintainer will review and may request changes
8. **After approval**, a maintainer will merge to `main` and the source branch is deleted

---

## Branch Strategy

```
main   ── active development AND releases (PR target, default branch)
staging ── docs deployment branch (auto-deploys GitHub Pages on push)
beta    ── frozen (legacy, no longer used for normal flow)
dev     ── frozen (legacy, kept for historical reference only)
```

- **All PRs target `main`**. Feature branches branch from `main` and merge back to `main`.
- The feature branch is **deleted on merge** (keep the repo clean).
- `staging` is used only by the docs deploy workflow (`docs.yml`). Docs source-of-truth lives on `main`; the workflow pushes built HTML to `staging` to trigger GitHub Pages.
- `dev` and `beta` are kept as frozen historical branches. Do not target them with new PRs.
- Releases are auto-tagged by CI when a release workflow is triggered on `main`.

### Why this change

Previously the project used a `dev → beta → main` flow, which made sense when releases were infrequent. The current model is simpler: every merge to `main` is shippable, and releases are explicit (via the `Publish Release` workflow). This keeps the dev loop tight and the branch surface small.

---

## Release Process

Releases are **fully automated** via the CI pipeline:

1. Merge to `main` triggers the release workflow
2. CI computes the next patch version (`v0.1.x`)
3. Cross-compiles binaries for Linux, macOS, and Windows
4. Creates a GitHub tag and draft release with pre-built binaries

Maintainers can trigger a manual release by pushing to `main` or using `workflow_dispatch`.

---

## Getting Help

- **Issues**: open a [bug report](https://github.com/arinbalyan/scrappy/issues/new?template=bug_report.md) or [feature request](https://github.com/arinbalyan/scrappy/issues/new?template=feature_request.md)
- **Discussions**: use GitHub Discussions for questions that aren't bugs or features
- **Documentation**: read the [docs site](https://arinbalyan.github.io/scrappy/)
- **Internal AI context**: see [`AGENTS.md`](./AGENTS.md) for agent-oriented project context

---

*Happy scraping!*
