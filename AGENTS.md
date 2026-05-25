# Repository Guidelines

This project is `scrappy` — a private Go library for job-board scraping, porting and extending [JobSpy](https://github.com/speedyapply/JobSpy) (the upstream Python reference). It targets better memory efficiency, faster concurrency, fewer dependencies, and bulk-first operations using Go's native `net/http`, `encoding/json`, and `context` packages. It is the shared dependency of the public `jobhunter` repo (rewritten in Go + future Rust orchestration). No Python is used in either.

```
github.com/you/scrappy        ← use your actual GitHub org/user
```

This repo URL: `git@github.com:arinbalyan/scrappy.git` (empty repo, `git init` + remote added, ready to push).

---

## Commit Strategy

Make small, focused commits — one feature or file per commit. Each commit should carry its own documentation update.

### Commit cadence

```
commit  1 – scaffold: go.mod, AGENTS.md, directory layout
commit  2 – model:   JobPost, Site, Country, ScraperInput types
commit  3 – rate:    per-site token-bucket rate limiter
commit  4 – proxy:   proxy pool, SOCKS5 support, health probes
commit  5 – email:   email extractor, MX validator, normalization
commit  6 – dedup:   URL deduplicator + company dedup
commit  7 – quality: deterministic score 0-100
commit  8 – export:  CSV + JSONL writers
commit  9 – export:  XLSX + Parquet writers
commit 10 – scraper: Indeed scraper (first site)
commit 11 – scraper: LinkedIn scraper
commit 12 – scraper: Glassdoor, ZipRecruiter, Google scrapers
commit 13 – scraper: new sites (Wellfound, RemoteOK, Remotive, BuiltIn)
commit 14 – cli:     wire all CLI flags with cobra
commit 15 – e2e:     integration tests + CI GitHub Actions workflow
commit 16 – doc:     README.md with usage examples
commit 17 – doc:     docs/ folder — per-feature markdown guides (see below)
```

Commit message format: `feat:`, `fix:`, `refactor:`, `docs:`, `chore:`, `test:`.

---

## Documentation (`docs/` folder)

Every non-trivial feature gets a page in `docs/`. The folder mirrors the codebase structure.

```
docs/
├── overview.md          # Architecture diagram, quick-start, bulk-first philosophy
├── architecture.md      # Pipeline walkthrough, data flow, packaging
├── installation.md      # Go install, Docker, CI setup
├── cli.md               # Full CLI flag reference with examples
├── scraping.md          # Scraper interface, site-specific notes, rate limits
├── email.md             # Email pipeline: extract → normalize → MX → enrich
├── quality.md           # Quality score formula, agency blocklist
├── dedup.md             # URL and company deduplication strategy
├── export.md            # Output formats, Writer interface, column schema
├── proxy.md             # Proxy setup (local, CI, health checks, SOCKS5)
├── sites/               # One page per site
│   ├── indeed.md        # Indeed-specific notes, GraphQL cursor pagination
│   ├── linkedin.md      # LinkedIn limit, rotate strategy, header requirements
│   ├── glassdoor.md
│   ├── ziprecruiter.md
│   ├── wellfound.md
│   ├── remoteok.md
│   ├── remotive.md
│   └── builtin.md
└── contributing.md      # Dev setup, test workflow, PR checklist
```

Each page covers: purpose, API / code snippet, CLI flags, known limitations, and troubleshooting.

---

## README.md Requirements

Keep it scannable and practical:

```
# scrappy

Bulk job-board scraper in Go. Port and extension of [JobSpy](https://github.com/speedyapply/JobSpy).

## Why scrappy

- Bulk-first: fan out across 10+ sites concurrently, process thousands of postings
- Go-native: 10 MB static binary, zero Python dependency
- Email enrichment: MX-validated contact addresses from description + company pages
- Exports: CSV, JSONL, XLSX, Parquet
- Private Go lib → public Go+Rust JobHunter app

## Quick start

  go install github.com/arinbalyan/scrappy/cmd/scrappy@latest
  scrappy scrape --sites linkedin,indeed --search "software engineer" --results-wanted 500

## Features  (link to docs/)

## Installation  (Go, Docker, CI)  → link to docs/installation.md
## Sites supported  → table with links to docs/sites/
## CLI reference  → link to docs/cli.md
## Go library usage  → code snippet
## License
```

---

## Repo bootstrap commands

```bash
# In /home/nemesis/projects/scrappy
cd /home/nemesis/projects/scrappy

git init
git remote add origin git@github.com:arinbalyan/scrappy.git
git add .
git status                        # review staged files
git commit -m "chore: scaffold — AGENTS.md, .gitignore, directory layout, tmp/"
git branch -M main
git push -u origin main
```

Then for each feature commit:

```bash
# Example: commit the Go model
git add internal/model/
git commit -m "feat(model): JobPost, Site, Country, ScraperInput Go types"
git push

# Example: commit the email verifier
git add internal/email/
git commit -m "feat(email): extract → normalize → MX verify pipeline"
git push
```

Every feature commit updates at least one page in `docs/`. Example:

```
commit : feat(email): extract → normalize → MX verify pipeline
docs/   : + docs/email.md  (pipeline docs, CLI flags, known-issues)
```

Before each push:

```bash
git diff --cached    # inspect staged changes — no secrets, no tmp/ files
git status           # confirm only intended files are staged
```

**Never commit to `main` directly for larger features.** Create a branch:

```bash
git switch -c feat/email-verification
# ... commit one or more times ...
git push -u origin feat/email-verification
gh pr create --base main --head feat/email-verification
```

---

## CI / GitHub Actions

```yaml
# .github/workflows/ci.yml
name: CI
on: [push, pull_request]
jobs:
  build-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: 1.24
          cache: true
      - run: go mod tidy
      - run: go build ./...
      - run: go test -race -cover ./...
      - run: go vet ./...
```

**v0.1.0 finalization rule:** create/enable this CI workflow at release-finalization time (when v0.1.0 is declared complete), and ensure it runs on every push/PR commit.

Add a Docker build step when the `Dockerfile` exists.

```
scrappy/
├── cmd/
│   ├── scrappy/          # Main CLI binary
│   └── example/          # Programmatic usage example
├── internal/
│   ├── scraper/          # Site scrapers (one package per site)
│   │   ├── indeed/
│   │   ├── linkedin/
│   │   ├── glassdoor/
│   │   ├── google/
│   │   ├── ziprecruiter/
│   │   ├── wellfound/
│   │   ├── remoteok/
│   │   ├── remotive/
│   │   ├── builtin/
│   │   ├── otta/
│   │   ├── lever/
│   │   └── greenhouse/
│   ├── model/            # JobPost, ScraperInput, Site, Country (Go structs)
│   ├── rate/             # Per-site rate limiters (token bucket)
│   ├── proxy/            # Proxy pool + health probes + SOCKS5 support
│   ├── email/            # Extract → normalize → validate (MX lookup)
│   ├── dedup/            # URL and company deduplication
│   ├── export/           # Writers: jsonl, csv, xlsx, parquet
│   ├── normalize/        # Salary normalization to annual USD, job title normalization
│   ├── quality/          # Deterministic score 0-100 per posting
│   └── util/             # Retry with backoff, structured logging
├── pkg/                  # Exported reusable packages (library users)
├── tests/                # Integration / end-to-end tests
├── Dockerfile            # Multi-stage: builder → distroless scratch (<10 MB)
├── docker-compose.yml    # Docker Compose for scheduled bulk runs
├── go.mod
├── go.sum
└── AGENTS.md
```

---

## Build, Test, and Development Commands

```bash
go mod tidy              # Sync dependencies
go build ./...           # Build all packages + binary
go run cmd/scrappy/      # Run the CLI locally
go run cmd/example/      # Run the programmatic usage example
go test ./...            # Run all unit + integration tests
go test -race ./...      # Run tests with race detector
go benchmark ./...       # Run benchmarks
go vet ./...             # Static analysis / lint

# Docker
docker build -t scrappy .              # Build image
docker run scrappy scrape --sites linkedin,indeed --search "software engineer" --location "SF"
docker run scrappy export --format parquet --emails-only --out /out/jobs.parquet
docker compose up                      # Scheduled bulk run via docker-compose
```

## Coding Style & Naming Conventions

- **Formatter:** `gofmt` or `goimports` — no manual reformatting.
- **Style:** Standard Go conventions; idiomatic Go over clever abstractions.
- **Indentation:** tabs (not spaces).
- **Naming:** `camelCase` for unexported, `PascalCase` for exported identifiers. Packages use short, lowercase, single-word names.
- **Error handling:** always return `error` explicitly; wrap with context using `fmt.Errorf("context: %w", err)`.
- **Concurrency:** prefer `context.Context` for cancelation and timeouts over `time.After` or goroutine leaks.

## Testing Guidelines

- **Framework:** standard `testing` package; use `testify` (`assert`/`require`) if needed.
- **Current file naming/layout (pre-v0.1.0):** `*_test.go` may live adjacent to implementation while features are being built.
- **v0.1.0 finalization rule:** move tests into the dedicated `tests/` tree (mirror package structure) so main source folders stay test-file-free.
- **Coverage:** aim for >= 80% on new code. Run `go test -cover ./...`.
- **Table-driven tests** are preferred.
- **Integration tests:** place under `tests/`; tag them, e.g. `//go:build integration`.
- **Benchmarks:** name `BenchmarkXxx` and keep them deterministic.

## Pre-Commit DocWriter Workflow (MANDATORY)

Before EVERY commit (not for scratch/throwaway work-in-progress), run `DocWriter` to keep project docs current:

1. **Stage changes** that will be committed: `git add <files>`
2. **Run `DocWriter` agent** by invoking:
   ```
   task(
     subagent_type="DocWriter",
     description="Update docs for <feature>",
     prompt="Review staged changes in this repo. Update README.md and any docs/ files that reference changed code, sites, or features. Verify docs match the new implementation."
   )
   ```
3. **Review and commit** the doc updates output by DocWriter alongside code changes
4. **Push**

> If DocWriter is skipped for expedience, open a follow-up ticket to update docs before merging into `beta` or `main`.

---

## Commit & Pull Request Guidelines

- **Format:** [Conventional Commits](https://www.conventionalcommits.org) e.g. `feat: add Glassdoor search filter`, `fix: race condition in concurrent fetcher`, `chore: update go.mod`.
- **Code Review:** Always run the code reviewer (via `droid run code-reviewer` or equivalent) before committing to ensure code quality and consistency.
- **Body:** explain *why*, not *what*.
- **PRs:** fill out the PR description template, link related issues, and include `go test -race & pass` confirmation before requesting review.
- **Screenshots:** include CLI output examples for scraper changes.

### Active branch workflow (current)

- **Working branch:** `dev` (commit here for all changes).
- **Staging branch:** `beta` (merge `dev` into `beta` regularly).
- **Release branch:** `main` (merge `beta` into `main` after some days).
- **Flow:** `dev` → `beta` → `main`
- **Migration source:** `tmp/ever-jobs`.
- **Execution mode:** implement **one scraper at a time** from `tmp/ever-jobs` into `scrappy`.
- For each scraper migration:
  1. Add scraper package and wire it into runtime.
  2. Add/adjust tests for that scraper.
  3. Run focused tests for that scraper, then run `go test ./...` and `go vet ./...`.
  4. If green, create a **small commit** for that scraper only.
  5. Push `dev` after each successful scraper commit.
  6. Periodically merge `dev` → `beta`.
  7. After some days, merge `beta` → `main` (creates release).

## Security & Configuration Tips

- Respect `robots.txt` and site Terms of Service before adding new provider scrapers.
- Rotate user-agents and add backoff/retry logic — do not hammer external sites.
- Never commit API keys, cookies, or proxy credentials. Use environment variables (`os.Getenv`) and mirror the `.env.example` pattern in `.env`.
- Prefer Go stdlib over third-party HTTP/HTML dependencies to reduce attack surface.

---

## Usage Modes

`scrappy` is a **bulk scraper** — its primary purpose is to fan out across many sites concurrently, process thousands of postings, and ship the result to disk or stdout. Unlike the upstream Python reference (single-shot, in-memory DataFrame), `scrappy` is designed for repeated, scheduled, and volume-first operations.

### CLI (primary)

```bash
scrappy scrape \
  --sites linkedin,indeed,glassdoor,google \
  --search "software engineer" \
  --location "San Francisco, CA" \
  --results-wanted 500 \
  --max-rps 10 \
  --format parquet \
  --out /data/jobs.parquet \
  --emails-only \
  --min-score 60 \
  --dedup \
  --retries 3
```

### As a Go library (secondary)

```go
import "github.com/you/scrappy/internal/scraper"

jobs, err := scraper.Scrape(ctx, ScraperInput{
    Sites:        []string{"indeed", "linkedin"},
    Search:       "software engineer",
    Location:     "San Francisco, CA",
    ResultsWanted: 500,
})
```

### Docker (CI / scheduled bulk)

```dockerfile
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /scrappy ./cmd/scrappy

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /scrappy /scrappy
ENTRYPOINT ["/scrappy"]
```

```yaml
# docker-compose.yml — nightly bulk scrape to a mounted volume
services:
  scrappy:
    build: .
    volumes: ["./data:/out"]
    command: scrape --sites linkedin,indeed,glassdoor,remoteok \
                    --search "software engineer" --location "Remote" \
                    --results-wanted 1000 --format jsonl --out /out/jobs.jsonl \
                    --emails-only
    environment:
      - SCRAPPY_PROXIES=user:pass@proxy1:8080,user:pass@proxy2:8080
      - SCRAPPY_RETRIES=3
```

### CI / GitHub Actions

```yaml
- uses: docker://ghcr.io/you/scrappy:v0.1.0@sha256:...
  with:
    args: scrape --sites indeed,glassdoor --search "developer" --results-wanted 200 --format csv --out /github/workspace/jobs.csv
```

---

## Site-Specific Result Limits

Each board has a different hard or soft cap. The upstream Python reference silently truncates; `scrappy` exposes per-site overrides.

### LinkedIn — hard cap ~1,000 results

LinkedIn's guest search endpoint caps `start` at < 1,000 (~100 pages x 10 jobs/page). JobSpy source confirms: `continue_search = lambda: ... and start < 1000`.

**Workaround:** `--linkedin-strategy rotate` — permutes filter combinations (`f_AL`, `f_JT`, `f_TPR`, distance, location radius) and re-scrapes each permutation as a separate pass, aggregating all results.

### Indeed — cursor-based GraphQL, effectively unbounded

Indeed uses a `nextCursor`-based GraphQL API with no observed hard cap. More requests = higher 429 risk; configure `--site-rps indeed:3` for sustained runs. LinkedIn rate-limits around page 10 (first ~100 jobs, returning 429); `scrappy` silently stops at that point (does not fail).

### Google Jobs

Results are embedded in the SERP HTML. No documented hard cap, but Google aggressively rate-limits. Treat as best-effort via `--max-rps google:2`.

### Per-site `results_wanted` override flags

```
--results-wanted-global 500          # default for all sites
--results-wanted-indeed 5000         # Indeed can go higher (cursor-based)
--results-wanted-linkedin 1000       # LinkedIn hard cap
--results-wanted-glassdoor 1000      # Glassdoor rounds to next day
```

### Per-site concurrency caps

| Site | Max concurrent | Max RPS |
|---|---|---|
| LinkedIn | 1-2 goroutines | 1 req/3s |
| Indeed | 10 goroutines | 3 req/s |
| Glassdoor | 4 goroutines | 2 req/s |
| Google | 2 goroutines | 1 req/2s |
| ZipRecruiter | 4 goroutines | 2 req/s |
| Wellfound / RemoteOK / Remotive | 8 goroutines | 5 req/s |

Use `errgroup.Group` + per-site semaphore — not a single global pool.

---

## Local Proxy + CI Proxy Setup

`scrappy` must work identically on a personal dev machine, a VPS, and GitHub Actions — all through the same `--proxy` flag.

### Local proxy on personal machine / VPS

Deploy [goproxy](https://github.com/snail007/goproxy) — a single Go binary, SOCKS5/HTTP proxy, no install required:

```bash
# Start a SOCKS5 proxy on localhost:7890, optionally chaining an upstream exit proxy
./goproxy -t socks5 -b 0.0.0.0:7890 --auth user:pass \
    -u socks5://exit-proxy-host:1080
```

```bash
# scrappy uses it immediately
scrappy scrape \
  --sites linkedin,indeed,remoteok \
  --proxy socks5://localhost:7890 \
  --search "software engineer" \
  --results-wanted 500
```

`scrappy` also supports a comma-separated list for direct remote proxies (same as JobSpy):
`--proxy socks5://localhost:7890,socks5://proxy2:1080`

Use `--local-proxy-port 7890` to auto-read `socks5://localhost:<port>` and prepopulate the proxy list.

### Local proxy in GitHub Actions

Start `goproxy` as a background process in the same job step — it is reachable at `localhost:7890` for both the runner and `docker run --network host` containers on the same host:

```yaml
- name: Install goproxy
  run: |
    curl -fsSL https://github.com/snail007/goproxy/releases/download/v1.1.7/goproxy_1.1.7_linux_amd64.tar.gz | tar xz
    chmod +x goproxy

- name: Start local proxy (background)
  run: |
    ./goproxy -t socks5 -b 0.0.0.0:7890 --auth ${{ secrets.PROXY_USER }}:${{ secrets.PROXY_PASS }} -u socks5://${{ secrets.EXIT_PROXY }} &
    sleep 3

- name: Run scrappy
  run: |
    docker run --network host scrappy:latest \
      scrape --sites indeed,glassdoor,remoteok \
             --search "software engineer" --location "Remote" \
             --results-wanted 200 --format csv --out /out/jobs.csv \
             --proxy socks5://localhost:7890

- name: Upload results
  uses: actions/upload-artifact@v4
  with:
    name: jobs
    path: /tmp/jobs.csv
```

### Proxy health checks

Before any proxy enters the rotation pool, probe it:

```go
// internal/proxy/health.go
func (p *ProxyPool) probe(ctx context.Context, proxyURL string) bool {
    req, _ := http.NewRequestWithContext(ctx, http.MethodHead, "https://httpbin.org/ip", nil)
    req.URL = p.viaProxy(req.URL, proxyURL)
    resp, err := http.DefaultTransport.RoundTrip(req)
    return err == nil && resp.StatusCode == http.StatusOK
}
```

Dead proxies are marked `unhealthy` for the rest of the run. Use `--proxy-health-check=false` to skip the probe and round-robin without health-gating.

---

## Email Verification

JobSpy extracts emails with a bare regex and returns them with zero validation. `scrappy` validates each candidate address and tags its provenance before it ever ships to output.

### Extracted-emails coverage (ground truth)

| Source | Email presence |
|---|---|
| Job description body | ~10-25% — recruiter HR emails, obfuscated patterns |
| Company career/contact pages (2-hop fetch) | **~60-80%** — careers@, hr@, recruiting@ |
| `mailto:` links | Embedded in some listing pages |
| LinkedIn Easy Apply / Indeed | **Zero** — platforms hide contact emails by design |

### Validation pipeline

```
extract_emails()          — regex from description + mailto links
       ↓
normalize_emails()        — [at] → @, strip whitespace, lowercase domain
       ↓
validate_mx_async()       — net.LookupMX(domain), 50-goroutine fan-out
       ↓
Result: Email{Addr, Verified bool, Source string}
```

**Implementation sketch:**

```go
// internal/email/verify.go
func ValidateMX(addr string) (bool, error) {
    domain := strings.SplitN(addr, "@", 2)[1]
    mxs, err := net.LookupMX(domain)
    return len(mxs) > 0, err
}

func VerifyBatch(ctx context.Context, addrs []string, concurrency int) []Email {
    sem := make(chan struct{}, concurrency)
    var wg sync.WaitGroup
    results := make([]Email, len(addrs))
    for i, a := range addrs {
        wg.Add(1)
        sem <- struct{}{}
        go func(i int, a string) {
            defer wg.Done()
            defer func() { <-sem }()
            verified, _ := ValidateMX(a)
            results[i] = Email{Addr: a, Verified: verified}
        }(i, a)
    }
    wg.Wait()
    return results
}
```

**Pre-MX exclusions (discarded before DNS check):**

- Role-only addresses: `info@`, `support@`, `admin@`, `noreply@`, `no-reply@` — tag `role: true`, skip for sourcing.
- Platform routing addresses: `*@indeed.com`, `*@glassdoor.com`, `*@linkedin.com` — always discard.
- Disposable/malicious TLDs: `*@10minutemail.com`, `*@guerrillamail.com` — configurable blocklist.

**Email source tags:**

| Tag | Description |
|---|---|
| `description` | Found verbatim in the job body text |
| `company_page` | Scraped from `company.com/careers` or `company.com/contact` |
| `mailto` | Extracted from an `href="mailto:..."` link |
| `direct` | From a `mailto:` link on the job listing page itself |

**CLI flags:**

```
--verify-email          # Enable MX lookup (default: true)
--verify-concurrency 50 # Concurrent MX DNS lookups per batch
--exclude-roles         # Skip info@, admin@, support@ (default: true)
--email-max-per-job 3   # Cap extracted emails per posting
--email-enrich          # Enable company-page follow-up (default: true)
--email-enrich-domains careers,contact,about,team  # Pages to probe
```

**Output:** `emails` + `emails_verified` as parallel JSON arrays in Parquet/JSONL; in CSV, use semicolon-joined columns.

---

## Job Quality Score (no LLM needed)

Deterministic score 0-100 computed per posting. `JobHunter` calls `--min-score N` before ever touching the LLM pipeline.

```
Salary mentioned                +30
Direct apply link present       +20
Email domain == company domain  +15
Posted within last 24h          +15
Description length > 200 chars  +10
NOT a staffing/agency posting   +10
```

Staffing/agency domain blocklist lives at `internal/quality/agency_domains.txt`, updated at build time.

---

## Feature Priorities

### Priority 1: Core Parity + Go-Native Improvements

JobSpy (Python) covers 8 sites as one scraper per sub-package. `scrappy` mirrors this structure and adds features that are expensive or impossible in Python.

#### Bulk Scraping Enhancements

| Feature | Why in Go | Implementation |
|---|---|---|
| **Per-site rate limiter** | Goroutine pool per domain, no GIL stall | `golang.org/x/time/rate` — token bucket per hostname, configurable RPS |
| **Resume / incremental offset** | JobSpy has `offset` but no crash-resume | JSON state file at `tmp/checkpoint.json` tracking last-scraped URL per site; resumes where it left off |
| **Cross-site dedup by job URL** | Python only does per-DataFrame dedup | `sync.Map` or `map[string]bool` deduplicator keyed on job URL before output |
| **Smart per-site max concurrency** | Python `ThreadPoolExecutor` is OS-thread bound | `errgroup.Group` + per-site semaphore |
| **Proxy pool with health checks** | Python round-robins blindly | Probe with HEAD /robots.txt before use; blacklist failing proxies mid-run |

#### New Job Sites (Priority 2)

Each site lives in `internal/scraper/{name}/` implementing:

```go
type Scraper interface {
    Scrape(ctx context.Context, input ScraperInput) ([]JobPost, error)
    SiteName() Site
}
```

| Site | Effort | Notes |
|---|---|---|
| **Wellfound** (wellfound.com) | easy | Public HTML listings, startup-focused |
| **RemoteOK** (remoteok.com) | easy | Scraper-friendly HTML, largest remote-only board |
| **Remotive** (remotive.com) | easy | JSON-embedded job data in pages |
| **BuiltIn** (builtin.com) | easy-medium | Tech-city specific (SF, Boston, NY, Austin) |
| **Otta** (otta.com) | medium | AI/talent matching; may need auth cookie rotation |
| **Lever** (jobs.lever.co) | medium | Pattern `{company}.jobs.lever.co` |
| **Greenhouse** (boards.greenhouse.io) | medium | Same lever-like pattern; mid-size+ companies |

---

### Priority 3: Extended Data Model

```go
type JobPost struct {
    // --- JobSpy fields ---
    ID, Title, CompanyName, JobURL, Location, Description string
    IsRemote bool; DatePosted time.Time; JobType, Emails []string
    Compensation *Compensation

    // --- new fields ---
    EmailSource      string     `json:"email_source,omitempty"`    // description | company_page | mailto | direct
    EmailVerified    bool       `json:"email_verified,omitempty"`   // MX lookup passed
    Seniority        string     `json:"seniority,omitempty"`        // entry | mid | senior | lead
    Department       string     `json:"department,omitempty"`       // eng | data | product | ...
    Domain           string     `json:"domain,omitempty"`            // company domain
    Industry         string     `json:"industry,omitempty"`
    CompanyLogoURL   string     `json:"company_logo_url,omitempty"`
    ApplyMethod      string     `json:"apply_method,omitempty"`     // easy_apply | email | external_url
    SalaryNormalized *Salary    `json:"salary_normalized,omitempty"` // always-annual, always-USD
}
```

---

### Priority 4: All CLI Flags

```
# Scrape target
--sites linkedin,indeed,glassdoor,google,wellfound,remoteok,arbeitnow
--search "software engineer"
--location "San Francisco, CA"
--google-search-term "..."          # Google-specific search string
--country-indeed usa                # Indeed/Glassdoor country
--distance 50                       # Miles
--job-type fulltime|parttime|internship|contract
--is-remote false
--easy-apply false
--hours-old 72                      # Filter by recency
--offset 0                          # Skip first N results
--linkedin-company-ids 123,456      # LinkedIn company ID filter
--linkedin-fetch-description false  # Fetch full LinkedIn description (O(n) requests)

# Volume
--results-wanted 15
--results-wanted-global 500
--results-wanted-indeed 5000
--results-wanted-linkedin 1000
--results-wanted-glassdoor 1000

# Rate limiting
--max-rps 5
--site-rps linkedin:1,indeed:10

# Proxy & resilience
--proxy socks5://localhost:7890,...  # Comma-separated list
--local-proxy-port 7890              # Auto-read socks5://localhost:<port>
--proxy-health-check true           # Probe proxies before use
--retries 3                          # Exponential backoff on 429/5xx
--linkedin-strategy rotate           # Permute filters to beat the 1k cap

# Email
--emails-only                        # Only jobs with >=1 verified email
--no-email                           # Only jobs with NO email (for sourcing)
--verify-email true                  # MX lookup (default: true)
--verify-concurrency 50
--exclude-roles true
--email-max-per-job 3
--email-enrich true                  # Company-page follow-up
--email-enrich-domains careers,contact,about,team

# Filtering
--dedup                              # Drop duplicate job URLs across sites
--dedup-by-company                   # Keep 1 posting per company
--min-score 60                       # Quality score floor (0-100)
--remote-only
--domain mycompany.com
--seniority senior
--department engineering

# Output
--format csv|xlsx|jsonl|parquet
--out /path/to/output
--csv-emails-only                    # Flat email list in CSV
--description-format markdown|html|plain
--enforce-annual-salary false

# Logging
--verbose 0|1|2
```

---

### Priority 5: Output Column Order

```
site | title | company_name | location | is_remote | job_type | date_posted |
description | compensation (interval/min_amount/max_amount/currency) |
job_url | emails | emails_verified | email_source | apply_method |
seniority | department | company_url | job_url_direct |
company_industry | company_logo | company_revenue |
company_num_employees | company_addresses | company_description |
skills | experience_range | company_rating | company_reviews_count |
vacancy_count | work_from_home_type
```

---

## Scraper Interface

```go
type Scraper interface {
    Scrape(ctx context.Context, input ScraperInput) ([]JobPost, error)
    SiteName() Site
}
```

Each concrete scraper is responsible for: (1) building the site-specific HTTP request, (2) applying the site's pagination strategy (offset, cursor, start), (3) normalizing the raw response into `[]JobPost`, (4) respecting the site-specific RPS limit injected via `context`.

---

## Architecture Summary

```
scrappy scrape (CLI or Go library)
  |
  +-- [rate/]     Per-site token-bucket limiter
  +-- [proxy/]    Proxy pool + health probe + SOCKS5
  +-- [scraper/*] Site scrapers (concurrent, bounded by per-site semaphores)
  |     |
  |     +-- [email/]       Email extractor + MX validator
  |     +-- [normalize/]   Salary normalization, title normalization
  +-- [dedup/]    URL deduplicator + company dedup (sync.Map, cross-site)
  +-- [quality/]  Deterministic score 0-100
  +-- [export/]   Writer → csv / xlsx / jsonl / parquet
```
