# Email Pipeline

`internal/email/` -- extract, normalize, validate, enrich.

## Pipeline stages

```
extract_emails()          -- regex from description + mailto links
normalize_emails()        -- [at] -> @, strip whitespace, lowercase domain
validate_mx_async()       -- net.LookupMX(domain), semaphore-bounded fan-out
enrich_company_pages()    -- fetch careers@/contact@ pages for additional addresses
populate_domain()         -- set JobPost.Domain from email domain
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

The regex `[a-zA-Z0-9._%+\-]+(?:---[a-zA-Z0-9._%+\-]+)*@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}` handles plain addresses and dot-obfuscated `[at]` patterns (`user---example---com` becomes `user@example.com`). Each candidate is validated via `net/mail.ParseAddress` and checked against the disposable domain blocklist before being returned.

## Pre-MX exclusions

Candidates are discarded before DNS lookup:

1. **Role-only addresses**: `info@`, `admin@`, `support@`, `contact@`, `sales@`, `hello@`, `careers@`, `press@`, `marketing@`, `jobs@`, `hr@`, `recruiting@`, `noreply@`, `no-reply@`, `help@`, `enquiries@`, `enquiry@`, `billing@` -- tagged with `Role: true`
2. **Platform routing addresses**: Domains matching `*@indeed.com`, `*@glassdoor.com`, `*@linkedin.com` -- always discarded
3. **Disposable TLDs**: `guerrillamail.com`, `mailinator.com`, `trashmail.com`, `tempmail.com`, `10minutemail.com`, `yopmail.com`, `sharklasers.com`, `throwam.com`, `fakeinbox.com` -- hardcoded blocklist

## MX verification

```go
type EmailMXVerifier struct {
    LookupMX func(domain string) (mxEntries []string, gotMX bool)
}
```

`net.LookupMX(domain)` checks for MX records. When a verifier is wired via `EnrichEmailStage()`, company-page enrichment results are validated before being added to the job.

**Performance**: ~100 ms per domain. 50 concurrent lookups per batch (`--verify-concurrency 50`).

## Company-page enrichment

`CompanyPageEnricher` fetches company career/contact pages and extracts emails with bounded concurrency:

```go
enricher := email.NewCompanyPageEnricher(httpClient, 10, 500) // 10 concurrent, 500ms pause
```

Domains to probe are configured via `--email-enrich-domains careers,contact,about,team`.

## Domain population

After email extraction and verification, the email domain is stored on `JobPost.Domain`. This domain is used by the quality scoring pipeline to compute the +15 point bonus for email-domain/company-domain match. See [011-Quality.md](011-Quality.md).

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
| `--verify-email` | `true` | Enable MX DNS lookup |
| `--verify-concurrency` | `50` | Concurrent MX lookups per batch |
| `--exclude-roles` | `true` | Skip info@, admin@, support@, etc. |
| `--email-max-per-job` | `3` | Cap extracted emails per posting |
| `--email-enrich` | `true` | Enable company-page follow-up |
| `--email-enrich-domains` | `careers,contact,about,team` | Pages to probe |
| `--email` | `false` | Only include jobs with at least 1 email |

## Output

- **Parquet / JSONL**: `emails` is a structured array of `Email` objects with Addr, Verified, Source, Role
- **CSV**: semicolon-joined columns: `emails`, `emails_verified`, `email_source` (parallel arrays)
- **Emails-only filter**: `--email` drops all jobs with zero email addresses before export
