# Email Pipeline

`internal/email/` — extract, normalize, validate, enrich.

## Pipeline stages

```
extract_emails()          — regex from description + mailto links
normalize_emails()        — [at] → @, strip whitespace, lowercase domain
validate_mx_async()       — net.LookupMX(domain), semaphore-bounded fan-out
enrich_company_pages()    — fetch careers@/contact@ pages for additional addresses
```

## Extracted-emails coverage

| Source | Email presence |
|---|---|
| Job description body | ~10–25% |
| Company career/contact pages | **~60–80%** |
| `mailto:` links | Embedded in some pages |
| LinkedIn Easy Apply / Indeed | **Zero** — platforms hide emails |

## CLI flags

```
--verify-email           Enable MX lookup (default: true)
--verify-concurrency 50  Concurrent MX DNS lookups per batch
--exclude-roles          Skip info@, admin@, support@ (default: true)
--email-max-per-job 3    Cap extracted emails per posting
--email-enrich           Company-page follow-up (default: true)
--email-enrich-domains  careers,contact,about,team  # Pages to probe
```

## Email type

```go
type Email struct {
    Addr     string `json:"addr"`
    Verified bool   `json:"verified"`    // MX lookup passed
    Source   string `json:"source"`      // description | company_page | mailto | direct
    Role     bool   `json:"role,omitempty"` // info@, admin@, support@, etc.
}
```

## Pre-MX exclusions

- Role-only addresses: `info@`, `support@`, `admin@`, `noreply@`, `no-reply@`
- Platform routing: `*@indeed.com`, `*@glassdoor.com`, `*@linkedin.com`
- Disposable/malicious TLDs: `*@10minutemail.com`, `*@guerrillamail.com` (configurable blocklist in `internal/email/blocklist.txt`)

## Output

- Parquet / JSONL: `emails` + `emails_verified` as parallel arrays
- CSV: semicolon-joined `emails` + `emails_verified` columns
