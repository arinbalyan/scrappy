# Email Pipeline

`internal/email/` — extract, normalize, validate, enrich.

## Pipeline stages

```
extract_emails()          -- regex from description + mailto links
normalize_emails()        -- [at] -> @, strip whitespace, lowercase domain
validate_mx()             -- net.LookupMX(domain) with context-aware timeout
enrich_company_pages()    -- fetch careers@/contact@ pages for additional addresses
populate_domain()         -- set JobPost.Domain from first email address
set_verified()            -- set Email.Verified from MX lookup result
```

## Email type

```go
type Email struct {
    Addr     string `json:"addr"`
    Verified bool   `json:"verified"`       // MX lookup passed
    Source   string `json:"source"`         // description | company_page | mailto | direct
    Role     bool   `json:"role,omitempty"` // info@, admin@, support@, etc.
}
```

## Source tags

| Tag | Description |
|-----|-------------|
| `description` | Found verbatim in the job description body |
| `company_page` | Scraped from `company.com/careers` or `company.com/contact` |
| `mailto` | Extracted from an `href="mailto:..."` link |
| `direct` | From a `mailto:` link on the job listing page itself |

## Extraction

The regex `[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}` matches standard email addresses in text. It does **not** handle `[at]` obfuscation patterns. Each candidate is validated via `net/mail.ParseAddress` and checked against the disposable domain blocklist before being returned.

## Pre-MX exclusions

Candidates are discarded before DNS lookup:

1. **Role-only addresses**: `info@`, `admin@`, `support@`, `contact@`, `sales@`, `hello@`, `careers@`, `press@`, `marketing@`, `jobs@`, `hr@`, `recruiting@`, `noreply@`, `no-reply@`, `help@`, `enquiries@`, `enquiry@`, `billing@` -- tagged with `Role: true`
2. **Platform routing addresses**: Domains matching `*@indeed.com`, `*@glassdoor.com`, `*@linkedin.com` -- always discarded
3. **Disposable TLDs**: `guerrillamail.com`, `mailinator.com`, `trashmail.com`, `tempmail.com`, `10minutemail.com`, `yopmail.com`, `sharklasers.com`, `throwam.com`, `fakeinbox.com`, `maildrop.cc`, `getnada.com`, `burnermail.io`, `emailondeck.com`, `mohmal.com`, `temp-mail.org` -- hardcoded blocklist

## MX verification

```go
type MXVerifier struct {
    Resolver *net.Resolver          // DNS resolver (defaults to net.DefaultResolver)
    Timeout  time.Duration          // per-lookup timeout (defaults to 10s)
    LookupMX func(domain string) (hosts []string, ok bool) // optional test stub
}

func NewMXVerifier() *MXVerifier
func (v *MXVerifier) Verify(ctx context.Context, addr string) bool
```

`net.LookupMX(domain)` checks for MX records. The verifier is created once per scrape run and shared across all jobs. A `LookupMX` function field allows tests to inject a stub without touching the network.

**Performance**: ~100 ms per domain. The verifier is shared across all jobs in a batch, so DNS-level OS caching reduces repeated lookups for the same domain. The `Verify` method accepts a `context.Context` so lookups are cancelled when the parent scrape is cancelled.

## Company-page enrichment

`CompanyPageEnricher` fetches company career/contact pages and extracts emails with bounded concurrency:

```go
enricher := email.NewCompanyPageEnricher(httpClient, 10, 500) // 10 concurrent, 500ms pause
```

Domains to probe are configured via `--email-enrich-domains careers,contact,about,team`.

## Domain population

After email extraction, the domain from the first email address is stored on `JobPost.Domain`. This domain is used by the quality scoring pipeline to compute the +15 point bonus for email-domain/company-domain match. See [011-Quality.md](011-Quality.md).

## Extracted-emails coverage

| Source | Email presence |
|--------|---------------|
| Job description body | ~10-25% -- recruiter HR emails, obfuscated patterns |
| Company career/contact pages (enrichment) | **~60-80%** -- careers@, hr@, recruiting@ |
| `mailto:` links | Embedded in some listing pages |
| LinkedIn Easy Apply / Indeed | **Zero** -- platforms hide contact emails by design |

## CLI flags

| Flag | Default | Description |
|------|---------|-------------|
| `--verify-email` | `true` | Enable MX DNS lookup (runs synchronously per job) |
| `--exclude-roles` | `true` | Skip info@, admin@, support@, etc. |
| `--email` | `false` | Only include jobs with at least 1 email |

> **Note:** MX verification runs synchronously per job. Flags `--verify-concurrency`, `--email-max-per-job`, `--email-enrich`, and `--email-enrich-domains` are not currently wired in the CLI.

## Output

- **Parquet / JSONL**: `emails` is a structured array of `Email` objects with Addr, Verified, Source, Role
- **CSV**: semicolon-joined columns: `emails`, `emails_verified`, `email_source` (parallel arrays)
- **Emails-only filter**: `--email` drops all jobs with zero email addresses before export
