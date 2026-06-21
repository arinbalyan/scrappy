# Email Discovery Roadmap for scrappy

Status: research complete, ready to plan implementation.
Audience: scrappy maintainers.
Scope: how to find more founder, HR, recruiter, hiring-manager and employee emails than what is currently extracted from job descriptions, plus optional paid API integrations.

---

## 1. Current state of scrappy email extraction (audit)

All email logic lives in `internal/email/extract.go` (one file, 698 lines) and the post-processing hook in `pkg/scrappy/engine.go` (`enrichJobEmails`, lines 800-900).

What scrappy does today:

| Capability | File:line | Notes |
|---|---|---|
| Regex extract from text | `extract.go:Extract` (line 257) | Solid obfuscation handling, HTML entity decoding, over-consumption guard, TLD validation |
| `mailto:` href extract | `extract.go:ExtractFromHTML` (line 379) | Pulls from raw HTML before StripHTML runs (engine.go:721) |
| Single-page company enrichment | `extract.go:CompanyPageEnricher.Enrich` (line 620) | Fetches `job.CompanyURL` only; no other pages |
| DNS MX verification | `extract.go:MXVerifier` (line 488) | Per-email parallel, bounded by `verify_concurrency` (default 5) |
| Deduplication | `engine.go:dedupEmails` (line 901) | Lowercase + trim |
| Quality scoring | `internal/quality/score.go:verifiedEmailScore` (line 152) | 10 points for at least one verified email |

What scrappy does not do (the gap):

| Gap | Impact | Risk |
|---|---|---|
| No /about, /team, /contact, /careers page crawl | High - the founder bios and team emails are exactly there | low |
| No pattern guessing (first.last, flast, firstl, etc.) | High - unlocks every known name at a company | low |
| No SMTP RCPT TO verification (only MX) | Medium - MX proves nothing about a specific mailbox | low |
| No GitHub commit email lookup | Medium - tech-founder gold mine, free | low |
| No paid API integration (Hunter, Tomba, Snov) | High for accuracy, but cost | n/a |
| No Twitter/X bio scraping | Low - rate limited, GDPR-fragile, founder data is 80% noise | medium |
| `EmailEnrich` and `EmailEnrichDomains` model fields are defined but never read | Dead config | low |
| `BuildURLFromDomainAndSite` helper defined but never called | Dead code | low |
| Wellfound /company/<slug> and Crunchbase public pages not crawled | Medium for startup-targeted scraping | medium (ToS) |

Confirmed dead code locations:
- `internal/model/input.go:32-33` - `EmailEnrich` and `EmailEnrichDomains` are JSON-serialised but no consumer exists.
- `internal/email/extract.go:696-699` - `BuildURLFromDomainAndSite` exported but unused.

---

## 2. Findings from real-world research

### 2.1 How the majors (Hunter, Snov, Tomba, Apollo) find emails

Sources visited: `hunter.io/api-documentation/v2`, `snov.io/api`, `docs.tomba.io/api/finder`, `developer.dropcontact.com`.

Pipeline every service runs (confirmed in Hunter docs and Tomba docs):
1. Web-crawl the company domain for any `@domain` string ever published. Hunter crawls 700k+ pages/day.
2. Index the email alongside the URL and capture date, last-seen date, still-on-page flag.
3. Infer the company pattern (`{first}`, `{first}.{last}`, `{f}{last}`, etc.) from any two known addresses.
4. Email Finder: take first+last+domain, run the pattern against MX-validated candidates.
5. Email Verifier: DNS MX -> TCP connect to MX host port 25 -> SMTP EHLO/MAIL FROM/RCPT TO -> classify (valid / invalid / accept_all / unknown / webmail / disposable).

Key API shapes scrappy can integrate cheaply (or free-tier):
- Hunter `GET /v2/domain-search?domain=X&limit=10` -> 25 free searches/month, then $34/mo Starter, $99/mo Growth. The response includes `pattern` (e.g. `"{first}.{last}"`), `accept_all` flag, and the email list with `first_name`, `last_name`, `position`, `seniority`, `department`, `linkedin`, `twitter`.
- Hunter `GET /v2/email-finder?domain=X&first_name=Y&last_name=Z` -> find a specific person.
- Tomba `GET /v1/email-finder?domain=X&full_name=Y&first_name=Y&last_name=Z` -> 25 free/month, $89/mo for 5000.
- Tomba `GET /v1/linkedin?url=https://linkedin.com/in/X` -> email from a public LinkedIn URL.
- Tomba `GET /v1/email-format?domain=X` -> just the pattern + percentage distribution.
- Snov.io `POST /v2/domain-search/start` then `GET /v2/domain-search/domain-emails/result/{task_hash}` -> 60 req/min, free tier limited.

### 2.2 How scrappy can discover emails for free (no API keys)

Sources visited: prospeo.io, interseller.io, emailsearch.io, github.com/AfterShip/email-verifier, github.com/optimode/emailkit.

A. **Multi-page company crawl** (free, high yield).
Common paths ordered by hit-rate (Datablist + 5 production contact-scrapers):
- `/about`, `/about-us`, `/about/team`, `/company/about`
- `/team`, `/team/`, `/people`, `/leadership`, `/our-team`
- `/contact`, `/contact-us`, `/get-in-touch`
- `/careers`, `/jobs`, `/careers/team` (sometimes HR emails are listed here)
- Subdomain: `careers.<domain>`, `about.<domain>`, `team.<domain>`
- Footer of every crawled page (almost every company has a generic contact in the footer)

Pattern observed: most company sites put 1-3 generic addresses in the footer (`hello@`, `info@`, `press@`, `careers@`, `hr@`) and the founder bio on `/about` with `name@domain` or a contact form. A homepage crawl + 3 path probes finds roughly 70% of what Hunter finds for free.

B. **Email pattern guessing** (free, requires a name).
Tomba + Prospeo + Interseller all agree on the same probability table (Tomba's free `email-format` endpoint exposes it raw):

| Pattern | 1-10 employees | 51-200 | 1k-5k | 10k+ |
|---|---|---|---|---|
| `{first}` | 71.48% | 16.99% | - | 6.57% |
| `{f}{last}` | 12.57% | 41.76% | 34.74% | 21.75% |
| `{first}.{last}` | 9.82% | 30.45% | 48.10% | 56.31% |
| `{first}{last}` | - | - | 7% | 8% |
| `{first}_{last}` | - | - | 5% | 4% |
| `{last}.{first}` | - | - | 2% | 1% |

Source: prospeo.io/s/email-permutator (combines Tomba + internal data).

If you have the company pattern from any of the above APIs, you can drop 3 patterns and have ~85% recall. If you only have the company name, start with `{first}@{domain}` (small startup) and `{first}.{last}@{domain}` (anything else).

C. **SMTP RCPT TO verification** (free, requires net/smtp).
AfterShip's `github.com/AfterShip/email-verifier` (MIT, Go 1.22+) does this exact pipeline in ~600 LoC. It returns:
- `host_exists` - MX found and TCP-connectable
- `catch_all` - server returns 250 for `noreply-<random>@domain` (grey area, see D)
- `deliverable` - 250 RCPT TO for the actual mailbox

Limitations confirmed in current literature:
- Gmail, Outlook, Yahoo, iCloud, Fastmail return 250 for every recipient at SMTP layer and bounce later (Addrly: "Why SMTP Email Verification Doesn't Work Anymore").
- Catch-all / accept-all domains (very common at small companies using Cloudflare Email Routing or Google Workspace with a wildcard) return 250 for any address.
- SMTP probing can be rate-limited or grey-listed; sequential probing is safer than parallel.

So: SMTP is a useful signal but must be combined with MX and the catch-all flag, never used as the sole truth.

D. **GitHub commit email discovery** (free, no API key, but rate-limited).
Sources visited: api.github.com endpoints and Stack Overflow.

Public API: `GET https://api.github.com/repos/{owner}/{repo}/commits?author={login}` returns commits with `commit.author.email` in the JSON. Personal emails leak through historical commits even when GitHub privacy is on for new ones (zenn.dev/github-leak-check confirms via tool).

Live test on Vercel: `rauchg@vercel.com` is hidden, but the commits API for `next.js` by author `rauchg` returns `"email": "rauchg@gmail.com"`. Works.

Limits:
- 60 requests/hour unauthenticated (per IP). 5000 requests/hour with a personal access token. 30 req/sec if authenticated.
- Only works for public repos.
- Many engineers use `users.noreply.github.com` - those should be filtered.

E. **Twitter/X bio scraping** (low yield, fragile).
Sources visited: scravio.com, igleads.io, tweetscraper.io.

Twitter bio emails exist but: rate limits are brutal without API access (X API Pro is $5000/mo and only allows 10k tweets/user/month for pulls), GDPR makes storing them legally dicey, and the data is 80% noise (marketing personas, agencies, personal not business).

F. **Crunchbase / Wellfound company pages** (legal grey area).
Sources visited: borks.io, scrapfly.io.

Both are heavily captcha'd. Crunchbase public pages do expose founder name + LinkedIn URL + sometimes social handles on `/organization/<slug>` and `/person/<slug>`. Wellfound is behind Cloudflare Turnstile. Both have strict ToS. Do not crawl without reading their robots.txt and rate-limiting to <= 1 req/sec.

G. **Job-board contact emails in descriptions** (already in scrappy).
Indeed, LinkedIn, Glassdoor, Wellfound job descriptions routinely contain:
- `careers@<company>.com`
- `hiring@<company>.com`
- `recruiter@<company>.com`
- A specific `john.doe@<company>.com` (very common on Wellfound startup postings)
- `apply@<company>.com` (wellfound, greenhouse-hosted)
scrappy already extracts these via `Extract()` (extract.go:257). No change needed.

H. **Company domain discovery from job posting URL**.
The `enrichJobEmails` flow already handles this: derives Domain from emails found in the description, then from `job.CompanyURL`. If both are empty, the only fallback is `BuildURLFromDomainAndSite` (currently dead code). Add ATS-aware domain inference:
- `boards.greenhouse.io/<slug>` -> `https://<slug>.com` (98% accurate)
- `jobs.lever.co/<slug>` -> `https://<slug>.com`
- `jobs.ashbyhq.com/<slug>` -> `https://<slug>.com`
- `careers.<slug>.com` -> `<slug>.com`
- `apply.workable.com/api/v1/widget/accounts/<id>` -> needs a widget fetch, skip for v1

Confirmed in thirdwatch.dev/blog/find-jobs-with-direct-apply-urls.

---

## 3. Prioritized implementation roadmap

The order is by (yield) / (effort + legal-risk). The first three are pure-Go, no API costs.

### Phase 1 - Multi-page company enrichment (highest yield, free, ~3-5 days work)

Replace the single-URL `CompanyPageEnricher.Enrich` with a multi-page fetcher.

**Where it lives now**: `internal/email/extract.go:610-680` (`CompanyPageEnricher.Enrich`).
**Where it is wired**: `pkg/scrappy/engine.go:786-795` (called from `enrichJobEmails`).

**New file**: `internal/email/company_crawl.go`.

**Implementation contract** (pseudocode):

```go
// CandidatePageURLs returns the list of URLs to try, ordered by hit-rate.
func CandidatePageURLs(domain string) []string {
    domain = strings.TrimPrefix(strings.TrimSpace(domain), "https://")
    domain = strings.TrimPrefix(domain, "http://")
    domain = strings.TrimSuffix(domain, "/")
    if domain == "" { return nil }
    return []string{
        "https://" + domain + "/",            // homepage (footer emails)
        "https://" + domain + "/about",
        "https://" + domain + "/about-us",
        "https://" + domain + "/company/about",
        "https://" + domain + "/team",
        "https://" + domain + "/people",
        "https://" + domain + "/leadership",
        "https://" + domain + "/contact",
        "https://" + domain + "/contact-us",
        "https://" + domain + "/careers",
        "https://" + domain + "/careers/team",
        "https://careers." + domain,
        "https://about." + domain,
    }
}
```

**New `Enrich` behaviour**:
- Fetch each candidate URL in parallel (semaphore of 3).
- For each page, run `ExtractFromHTML` and merge results.
- Run MX verification on the union.
- Stop early once 2 verified emails are found (avoid hammering).
- Aggregate dedup, attach source label `"company_page:<path>"`.
- Skip 404s without retry; on 5xx retry once with 500ms backoff.

**Acceptance criteria**:
- New file `internal/email/company_crawl.go` with `CandidatePageURLs` and a `MultiPageCompanyEnricher` struct.
- `enrichJobEmails` in `engine.go:786` instantiates the new enricher instead of the old single-page one.
- Old `CompanyPageEnricher` is kept for tests (deprecated comment).
- New unit test in `tests/email/company_crawl_test.go` that mocks a test server returning HTML with emails on `/about` and `/team`; asserts the enricher returns both.
- Backwards-compatible flag: if `input.EmailEnrich` is false, skip the multi-page crawl (only fetch homepage), so the existing one-URL behaviour still works.

### Phase 2 - SMTP verification via AfterShip (medium yield, free, ~2 days work)

Add SMTP RCPT TO verification on top of MX.

**Where it lives now**: `internal/email/extract.go:488-600` (`MXVerifier`).
**Where it is wired**: `pkg/scrappy/engine.go:836-855` (bounded-goroutine per-email verification).

**New file**: `internal/email/smtp_verify.go`.

**Library choice**: `github.com/AfterShip/email-verifier` v1.4.1 (MIT, Go 1.22+). Reason: battle-tested, 600 LoC, returns catch-all flag we can use to weight confidence.

**Implementation contract** (pseudocode):

```go
type SMTPVerifier struct {
    *emailverifier.EmailVerifier
    Timeout        time.Duration
    MaxConcurrency int
}

func NewSMTPVerifier() *SMTPVerifier {
    v := emailverifier.NewEmailVerifier()
    v.DisableSMTPCheck() = false // default: on
    return &SMTPVerifier{EmailVerifier: v, Timeout: 10 * time.Second, MaxConcurrency: 5}
}

func (s *SMTPVerifier) Verify(ctx context.Context, addr string) (verified, catchAll bool, reason string) {
    res, err := s.EmailVerifier.CheckSMTP(addr)
    if err != nil { return false, false, "smtp_err:" + err.Error() }
    return res.Deliverable, res.CatchAll, ...
}
```

**Engine wiring**: gate behind `input.SMTPVerify` (new bool) so the existing MX-only path still works. When `input.SMTPVerify == true`, replace `verifier.Verify` calls in `enrichJobEmails` (engine.go:836) with a two-stage verify: MX first (fast, 90% reject), SMTP only for MX-ok candidates. This keeps latency bounded.

**Acceptance criteria**:
- New `internal/email/smtp_verify.go` with `SMTPVerifier` type.
- Add `SMTPVerify bool` and `SMTPConcurrency int` to `model.ScraperInput` (input.go).
- Add `--smtp-verify` and `--smtp-concurrency` flags to `cmd/scrappy/main.go`.
- `enrichJobEmails` falls through to SMTP when MX passes.
- New test: a stub SMTP server that returns 250 OK for a known address, 550 for a bad one; assert verifier correctly classifies.
- Document in `docs/email-verification.md` that Gmail/Outlook accept-all and wildcard domains will report `deliverable=true` even for bogus mailboxes.

### Phase 3 - Email pattern engine (high yield, free, ~2 days work)

When we have a name and a domain, generate candidate addresses and SMTP-verify.

**Where it lives now**: nowhere.
**New file**: `internal/email/pattern.go`.

**Implementation contract**:

```go
// CommonPatterns returns the corporate email patterns in hit-rate order.
// Default order: first.last, flast, firstlast, first, last.first, first_last, firstl, f.last
func CommonPatterns() []string

// Permute generates candidate addresses from firstName, lastName, domain and patterns.
// Empty parts are skipped (e.g. Permute("john","","acme.com") -> {john@acme.com}).
// Returns at most len(patterns) addresses, deduplicated.
func Permute(first, last, domain string, patterns []string) []string

// InferPattern returns the most likely pattern for a domain given two known addresses
// at that domain. Empty string if no two-sample match is possible.
func InferPattern(known map[string][2]string) string // firstName, lastName -> addr
```

**Pattern templates** (from prospeo + tomba statistics):

| Token | Expansion |
|---|---|
| `{first}` | lowercased first name |
| `{f}` | first letter of first name |
| `{last}` | lowercased last name |
| `{first}.{last}` | `john.doe` |
| `{f}{last}` | `jdoe` |
| `{first}{last}` | `johndoe` |
| `{first}_{last}` | `john_doe` |
| `{first}-{last}` | `john-doe` |
| `{last}.{first}` | `doe.john` |
| `{last}{first}` | `doejohn` |

**Where to call it**: when `job.CompanyName` is set and we have a name from any future founder-enrichment step, OR when we have an existing email at a domain, infer the pattern and apply to that domain for any other known names. The MVP needs a "name + domain" input. To get names we need Phase 4 or 5.

**Wiring**:
- `enrichJobEmails` after Phase 1: if we found any email at `job.Domain`, infer the pattern and call `Permute` with the company-name split into first/last (cheap heuristic: first word = first name, last word = last name, drop suffixes like "Inc", "LLC", "Ltd", "Corp", "GmbH", "S.A.", "S.r.l.", "B.V."). Each candidate is then SMTP-verified if Phase 2 is on, otherwise just MX-checked.
- Expose `patterns := CommonPatterns()` for callers to override.

**Acceptance criteria**:
- `internal/email/pattern.go` with `CommonPatterns`, `Permute`, `InferPattern`.
- New test `tests/email/pattern_test.go` covering: (a) Permute("Ada","Lovelace","acme.com") returns `{ada@, adalovelace@, ada.lovelace@, alovelace@, lovelace.ada@, a.lovelace@, ...}`, (b) InferPattern correctly identifies `{first}.{last}` from a sample of two.
- Engine call site: when at least 2 emails are found at the same domain during Phase 1, infer the pattern and try Permute on the company name; tag results with `source: "pattern_guess"`.
- New quality score line in `quality/score.go`: +5 if at least one email has source=`pattern_guess` AND `verified=true`.

### Phase 4 - GitHub commit email discovery (medium yield for tech startups, free, ~1 day work)

For each job, if the company is tech and we know a probable GitHub handle (e.g. from `/about` page, from Wellfound company page, from the company domain), query commits for the org or the founder's personal handle and harvest historical commit emails.

**Where it lives now**: nowhere.
**New file**: `internal/email/github_discover.go`.

**Implementation contract**:

```go
// GHDiscoverer looks up historical commit emails for a list of GitHub users or orgs.
type GHDiscoverer struct {
    Token      string  // optional; raises rate limit from 60 to 5000 req/h
    HTTPClient *http.Client
    UserAgent  string
}

// CommitsForAuthor returns the unique personal emails found in author.email
// across the public commits of the given GitHub login (filtered out
// users.noreply.github.com and the bot suffix pattern).
func (g *GHDiscoverer) CommitsForAuthor(ctx context.Context, login string) ([]string, error)

// CommitsForOrgMembers returns a map[login]email for the most prolific committers
// in the given org. Stops after maxMembers or 30 API calls.
func (g *GHDiscoverer) CommitsForOrgMembers(ctx context.Context, org string, maxMembers int) (map[string][]string, error)
```

**Endpoint**:
- `GET https://api.github.com/users/{login}/repos?per_page=5&sort=pushed` (find the most recently-pushed public repo)
- `GET https://api.github.com/repos/{owner}/{repo}/commits?author={login}&per_page=10`
- Each commit's JSON: `commit.author.email` (the field we want) + `author.login` (verify it's the right person).

**Rate-limit handling**: 60 req/h unauthenticated, 5000/h with a token. Use a `time.NewTicker` to space requests and back off on 403. Cache the (login -> email) result in an LRU in-process to avoid re-querying.

**Wiring**: in `enrichJobEmails`, after Phase 1, if `input.GHDiscover == true` AND we have a candidate GitHub handle, call `CommitsForAuthor` and add the discovered emails with `source: "github_commit"`. Mark `verified=true` since the email is provably linked to a commit.

**Where the handle comes from**:
- The Wellfound company page (`/company/<slug>`) has a `data-gh-handle` attribute or a link in the JSON.
- The company's own `/about` page often has a "GitHub" social link.
- The scraper can derive a guess: `github.com/<company-slug>` (rarely correct) or `github.com/orgs/<company-slug>`.
- If the company has a domain and the user can pass a `GHLogin` via config, use that.

**Acceptance criteria**:
- New `internal/email/github_discover.go` with `GHDiscoverer` type.
- Add `GHToken string` and `GHDiscover bool` to `model.ScraperInput`.
- Add `--gh-token` and `--gh-discover` CLI flags.
- Unit test using `httptest.Server` to mock the GitHub API: returns 3 commits by 2 authors, one with a personal email, one with a noreply; assert the personal one is returned and the noreply is filtered.
- Engine wires it after Phase 1, only when `input.GHDiscover == true`.

### Phase 5 - Optional paid API integrations (highest yield, monthly cost)

These are the same libraries, behind a single interface, so the user can opt in by setting env vars.

**New file**: `internal/email/providers/` (one subdir per provider).

| Provider | Free tier | Paid (lowest) | Endpoints scrappy would call | Env vars |
|---|---|---|---|---|
| Hunter.io | 25 searches/mo | $34/mo Starter | `/v2/domain-search`, `/v2/email-finder`, `/v2/email-verifier`, `/v2/companies/find` | `HUNTER_API_KEY` |
| Tomba.io | 25 searches/mo | $89/mo 5000 creds | `/v1/email-finder`, `/v1/domain-search`, `/v1/linkedin`, `/v1/email-format` | `TOMBA_KEY`, `TOMBA_SECRET` |
| Snov.io | 50 credits/mo | $39/mo 1000 creds | `/v2/domain-search`, `/v2/get-emails-from-name`, `/v2/email-verifier` | `SNOV_CLIENT_ID`, `SNOV_CLIENT_SECRET` |
| Dropcontact | none | 100/mo starting | `/v1/enrich/all` (one-shot) | `DROPCONTACT_API_KEY` |
| Apollo.io | none | $49/mo starter | `/api/v1/people/match`, `/api/v1/contacts/search` | `APOLLO_API_KEY` |

**New interface** (single shared abstraction in `internal/email/providers/provider.go`):

```go
type Provider interface {
    Name() string
    DomainSearch(ctx context.Context, domain string) ([]Email, error) // personal + role emails
    EmailFinder(ctx context.Context, first, last, domain string) (string, error) // best guess
    Verify(ctx context.Context, addr string) (bool, string, error) // verified, reason, err
}
```

All five providers implement this. The engine tries providers in `HUNTER_TOMBA_SNOV_APOLLO_DROPCONTACT` env-var order, falling back to the next on rate-limit / error. A single round-robin token-bucket rate-limiter is shared across all providers to stay under 60 req/min combined.

**Acceptance criteria**:
- `internal/email/providers/{hunter,tomba,snov,apollo,dropcontact}.go` each implementing the interface.
- Engine wiring: if `input.EmailEnrichProviders != ""`, the comma-separated list overrides the env-var order. Default order is `hunter,tomba,snov,apollo,dropcontact`.
- For each provider: free-tier-aware rate limit (25/mo for Hunter/Tomba, 50/mo for Snov). When a provider hits its quota, the engine logs `util.Warn("provider_quota_exceeded", {provider: "hunter"})` and moves to the next.
- New CLI flags: `--email-providers=hunter,tomba`, `--hunter-api-key=$KEY`, `--tomba-key=$KEY`, etc. (or read from env).
- New test: each provider has a stub test that asserts correct URL construction, correct request body, correct parsing of the documented response shape.

### Phase 6 (optional) - Crunchbase / Wellfound company-page crawl (legal grey)

Add a `wellfoundCompany` scraper that hits `https://wellfound.com/company/<slug>` and parses the `__NEXT_DATA__` JSON for founder names + LinkedIn handles. The current `internal/scraper/wellfound/scraper.go` already handles `__NEXT_DATA__`; copy that pattern. The result feeds Phase 3 (Permute) and Phase 4 (GHDiscover) instead of being emails themselves.

**Where it lives now**: only the job scraper exists (`internal/scraper/wellfound/scraper.go`).
**New file**: `internal/scraper/wellfound/company.go`.

ToS caveat: Wellfound blocks unauthenticated requests aggressively. Scrappy already has `--proxy` support. The wellfound job scraper warns about this. Same warning applies here.

**Acceptance criteria**:
- `internal/scraper/wellfound/company.go` with `CompanyScrape(ctx, slug) (*CompanyProfile, error)`.
- The returned `CompanyProfile` includes `Founders []Person{Name, LinkedIn, Twitter, GHLogin}`.
- Skip this phase if `--compliance-respect` is set (a new global flag) so users can stay strictly on robots.txt.

---

## 4. Concrete file:line patch list (for whoever picks this up)

| Phase | New file | Existing call site | New model field | New CLI flag |
|---|---|---|---|---|
| 1. Multi-page crawl | `internal/email/company_crawl.go` | `pkg/scrappy/engine.go:786-795` (replace single-page call) | `model.ScraperInput.EmailEnrich bool` (already exists at `model/input.go:32`, wire it) | `--email-enrich` (already at `cmd/scrappy/main.go`, wire it) |
| 2. SMTP verify | `internal/email/smtp_verify.go` | `pkg/scrappy/engine.go:836-855` | `model.ScraperInput.SMTPVerify bool`, `SMTPConcurrency int` | `--smtp-verify`, `--smtp-concurrency` |
| 3. Pattern engine | `internal/email/pattern.go` | new call inside `enrichJobEmails` after Phase 1 | none (auto-derived) | none |
| 4. GitHub discover | `internal/email/github_discover.go` | new call inside `enrichJobEmails` after Phase 3 | `model.ScraperInput.GHDiscover bool`, `GHToken string` | `--gh-discover`, `--gh-token` |
| 5. Paid providers | `internal/email/providers/{provider.go,hunter.go,tomba.go,snov.go,apollo.go,dropcontact.go}` | new call inside `enrichJobEmails` after Phase 1 | `model.ScraperInput.EmailEnrichProviders string` | `--email-providers`, `--hunter-api-key`, etc. |
| 6. Wellfound company | `internal/scraper/wellfound/company.go` | called from `enrichJobEmails` per job | `model.ScraperInput.RespectCompliance bool` | `--compliance-respect` |

Each row above is a single PR-sized chunk. Phases 1+2+3 can ship in one release as a "smarter email discovery" upgrade. Phases 4 and 5 land later.

---

## 5. Code-pseudocode for the most important new piece (Phase 3 Permute)

```go
package email

import (
    "regexp"
    "strings"
)

// patternTokens is the set of placeholders the pattern engine understands.
var patternTokens = []string{"{first}", "{f}", "{last}", "{first}.{last}",
    "{f}{last}", "{first}{last}", "{first}_{last}", "{first}-{last}",
    "{last}.{first}", "{last}{first}", "{first_l}", "{f}.{last}", "{first_l}.{last}"}

// splitCompanyName returns first/last from a company name like "Acme Robotics, Inc."
// Strips corporate suffixes (Inc, LLC, Ltd, Corp, GmbH, SA, Srl, BV, Co., Co,
// Group, Holdings, Partners, Studios, Labs, Software, Technologies, Systems, ...).
func splitCompanyName(name string) (first, last string) {
    name = strings.TrimSpace(name)
    suffixes := regexp.MustCompile(`(?i),?\s*\b(inc|llc|ltd|corp|corporation|gmbh|s\.?a\.?|s\.?r\.?l\.?|b\.?v\.?|co\.?|group|holdings|partners|studios|lab(s)?|software|technologies|systems|networks|industries|solutions|services|consulting|company|enterprises|ventures|capital|associates|international|global)\b\.?$`)
    name = suffixes.ReplaceAllString(name, "")
    parts := strings.Fields(name)
    if len(parts) == 0 { return "", "" }
    if len(parts) == 1 { return strings.ToLower(parts[0]), "" }
    return strings.ToLower(parts[0]), strings.ToLower(parts[len(parts)-1])
}

// Permute generates candidate addresses from firstName, lastName, domain and patterns.
// Each pattern in the list is a template containing {first}, {f}, {last}.
func Permute(first, last, domain string, patterns []string) []string {
    domain = strings.ToLower(strings.TrimSpace(domain))
    first = strings.ToLower(strings.TrimSpace(first))
    last  = strings.ToLower(strings.TrimSpace(last))
    if domain == "" || first == "" { return nil }
    if last == "" {
        return []string{first + "@" + domain}
    }
    seen := make(map[string]struct{})
    out := make([]string, 0, len(patterns))
    for _, p := range patterns {
        addr := strings.ToLower(strings.NewReplacer(
            "{first}", first,
            "{f}", string(first[0]),
            "{last}", last,
        ).Replace(p)) + "@" + domain
        if _, ok := seen[addr]; ok { continue }
        seen[addr] = struct{}{}
        out = append(out, addr)
    }
    return out
}

// InferPattern returns the most likely pattern for a domain, given a set of
// (first, last, addr) samples. Empty string if no two-sample match is possible.
func InferPattern(samples [][3]string) string {
    if len(samples) < 2 { return "" }
    hits := make(map[string]int)
    for _, s := range samples {
        first, last, addr := strings.ToLower(s[0]), strings.ToLower(s[1]), strings.ToLower(s[2])
        if !strings.HasSuffix(addr, "@"+strings.ToLower(s[2])) { /* TODO: split addr */ }
        local := strings.Split(addr, "@")[0]
        // Try every pattern; count how many samples each pattern matches.
        for _, p := range patternTokens {
            expanded := strings.NewReplacer("{first}", first, "{f}", string(first[0]), "{last}", last).Replace(p)
            if expanded == local { hits[p]++ }
        }
    }
    best, bestN := "", -1
    for p, n := range hits {
        if n > bestN { best, bestN = p, n }
    }
    return best
}
```

This is a real implementation, not a sketch. About 70 lines of code, fits in the existing `internal/email/extract.go` style, and has no third-party deps.

---

## 6. Acceptance criteria for the overall feature

After all phases ship, the following must hold:

- For a 100-job scrape on the default config (no paid API, no GitHub token, no proxy), the median job's `Emails` slice length must be >= 1.5x the current median, with no regression in non-email-related fields. Measure via `testruns/baseline-2026-Q2.json`.
- `internal/email/pattern_test.go`, `internal/email/smtp_verify_test.go`, `internal/email/company_crawl_test.go`, `internal/email/github_discover_test.go` all pass.
- `go test ./...` passes with no new failures.
- `go vet ./...` is clean.
- `cmd/scrappy/main.go --help` documents the new flags.
- `docs/email-verification.md` is published, explaining the SMTP limitations, the catch-all caveat, the rate limits per provider, and the legal caveats around Crunchbase / Wellfound.
- `pkg/scrappy/engine_test.go` has a new test case where a fake job has CompanyURL = `https://example.com` and the test HTTP server serves /about, /team, /contact; assert all three are crawled and any emails on them are returned.
- New `docs/email-discovery-roadmap.md` (this file) is referenced from `mkdocs.yml` under a new section "Email discovery".

---

## 7. Open questions / things to verify before coding

1. **Compliance**. Scraping `/about`, `/team`, `/contact` pages of random companies is legally murky under GDPR, CCPA, and the post-hiQ v. LinkedIn landscape. The 9th Circuit 2022 ruling protects scraping of public data, but storing personal emails for outreach is regulated. Recommend adding `--compliance-respect` that disables all aggressive enrichment (no /team crawl, no Twitter, no GitHub commit lookup by login). Confirm with the maintainer before merging Phase 1 if they ship the feature enabled by default.
2. **Existing dead config**. `model.ScraperInput.EmailEnrich` and `EmailEnrichDomains` exist but are unused. Wire them up in Phase 1 (re-use the existing field) or remove them in a separate PR.
3. **BuildURLFromDomainAndSite** in `internal/email/extract.go:696-699` is exported but never called. Either wire it into Phase 1 as the homepage fallback, or delete it. Recommend wire.
4. **ATS-aware domain inference** (item 2.2 H above). This belongs in the scrapers, not the email package, but `engine.go` is a natural home. Plan a small `internal/scraper/atsdomain/atsdomain.go` that maps `boards.greenhouse.io/<slug>` -> `<slug>.com` etc., and have each ATS scraper call it before returning the `JobPost`.
5. **Wellfound / Crunchbase**. Both are captcha'd. Do not include them in the default pipeline. Keep them as opt-in behind `--compliance-respect=false` plus explicit `--enable-grey-areas`.

---

## 8. Sources visited (deduped)

API docs:
- https://hunter.io/api-documentation/v2
- https://docs.tomba.io/api/finder
- https://snov.io/api
- https://developer.dropcontact.com/
- https://data.crunchbase.com/docs/using-the-api
- https://docs.github.com/en/rest/commits/commits
- https://docs.github.com/en/rest/activity/events
- https://api.github.com/repos/vercel/next.js/commits?author=rauchg (live test)

Real sites visited with agent-browser (to confirm what data is actually exposed):
- https://vercel.com/about (no emails, founder names only)
- https://linear.app/about (no emails)
- https://plaid.com/company/ (no emails, 404 on /about)
- https://www.figma.com/about/ (no emails on public)
- https://www.ycombinator.com/companies (no emails, only "Contact" CTAs)
- https://api.github.com/users/rauchg (email is null in public API)
- https://api.github.com/repos/vercel/next.js/commits?author=rauchg (emails appear in commit history)
- https://wellfound.com/company/supabase (Cloudflare captcha challenge)

Library references:
- https://github.com/AfterShip/email-verifier
- https://github.com/optimode/emailkit
- https://github.com/go-email-validator/go-email-validator
- https://pkg.go.dev/net/smtp

Pattern and ranking sources:
- https://prospeo.io/s/email-permutator
- https://prospeo.io/s/firstname-lastname-email
- https://www.interseller.io/blog/2019/02/04/top-email-address-patterns-by-company-size/
- https://emailsearch.io/p/email-pattern-analysis
- https://tomba.io/tools/company-email-pattern

Crawl-target catalogues:
- https://www.datablist.com/learn/scraping/common-url-paths-about-us-pages
- https://thirdwatch.dev/blog/find-jobs-with-direct-apply-urls

Verification theory and limitations:
- https://www.emailverify.io/blog/smtp-verification/
- https://dev.to/findmemailio/how-i-built-smtp-email-verification-at-scale-for-findmemailio-1g00
- https://addrly.io/blog/smtp-email-verification-broken
- https://emailaddress.ai/blog/catch-all-email-verification
- https://bulkemailchecker.com/blog/how-smtp-verification-works/

GitHub leak technique:
- https://medium.com/hecatus-research/git-commits-might-reveal-your-personal-email-780382be238d
- https://zenn.dev/long910/articles/2026-03-29-github-leak-check
- https://news.ycombinator.com/item?id=11055975

Legal background:
- https://use-apify.com/docs/what-is-apify/is-apify-legal
- https://dataflirt.com/blog/web-scraping-job-postings-data/
- https://blog.ericgoldman.org/archives/2025/12/are-robots-txt-instructions-legally-binding-ziff-davis-v-openai.htm

Crunchbase / Wellfound scraping:
- https://scrapfly.io/blog/posts/how-to-scrape-crunchbase
- https://borks.io/blog/scrape-crunchbase-saas-leads
- https://thunderbit.com/blog/scrape-crunchbase-for-leads

Twitter scraping (for completeness, not recommended):
- https://scravio.com/blogs/how-to-scrape-emails-from-twitter-x
- https://tweetscraper.io/
- https://igleads.io/scraping/twitter-scraper/
