# Email Extraction Pipeline

scrappy uses a multi-stage pipeline to extract, verify, and enrich email addresses from job postings. The pipeline runs in the engine's post-processing step and operates on every `JobPost` before export.

## Pipeline stages

```
Raw HTML description
  → ExtractFromHTML() (mailto: hrefs + standard emails)
  → Extract() (deobfuscated patterns + standard regex)
  → CompanyPageEnricher / MultiPageCompanyEnricher
  → MXVerifier (DNS MX record check)
  → SMTPVerifier (optional RCPT TO check)
  → Pattern permuter (first.last@domain inference)
```

---

## 1. Text Extraction — `internal/email/extract.go`

### `Extract(text string) []Email`

Scans plain text for email-like strings using a regex, then validates each candidate through multiple gates.

**Regex pattern:**

```go
mailRegex = regexp.MustCompile(
    `[a-zA-Z0-9._%+\-]+(?:---[a-zA-Z0-9._%+\-]+)*@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}\b`,
)
```

**Validation gates (in order):**

1. **Over-consumption guard** (`forOverconsumption`): rejects matches where the regex consumed adjacent text (e.g. `acme.com.jobs` where `.jobs` is an adjacent field, not part of the email). If the match was over-consumed, `tryShortenEmail` attempts to recover the real email by stripping the last domain segment.

2. **`strictlyValidEmail(addr)`**: RFC 5321-style validation via Go's `mail.ParseAddress`, plus:
   - Max 64-char local part, 255-char domain.
   - No consecutive/leading/trailing dots.
   - Domain must have at least one dot.
   - **TLD whitelist**: the last segment must be in `validTLDs` — a comprehensive map of 200+ real-world TLDs (gTLDs, ccTLDs, new-gTLDs). This rejects regex false-positives like `support@mercor.comps`.

3. **Blocked domain list**: disposes of throwaway mail providers (guerrillamail.com, mailinator.com, etc.), platform routing addresses (indeed.com, linkedin.com), and invalid suffixes (.local, .arpa, .test).

4. **Role address detection**: identifies `info@`, `admin@`, `support@`, `sales@`, etc. and marks them with `Role: true`.

**Deobfuscation** detects common anti-harvesting patterns:

```go
obfuscatedRegex = regexp.MustCompile(
    `(?i)([a-zA-Z0-9._%+\-]+)\s*(?:\[at\]|\(at\)|\bat\b)\s*` +
    `([a-zA-Z0-9.\-]+)\s*(?:\[dot\]|\(dot\)|\bdot\b)\s*([a-zA-Z]{2,})`,
)
```

Handles: `name [at] domain [dot] com`, `name(at)domain(dot)com`, `name AT domain DOT com`.

**HTML entity normalization** decodes `&#64;` → `@`, `&#46;` → `.` before regex matching.

### `ExtractFromHTML(html string) []Email`

For raw HTML (before HTML stripping), this function:

1. Extracts `mailto:href` attributes from `<a>` tags.
2. Strips query parameters from mailto URLs (`?subject=...`).
3. Also runs the standard `Extract()` on the HTML text.
4. Deduplicates results across both methods.

---

## 2. Company Page Enrichment — `internal/email/company_crawl.go`

### `CompanyPageEnricher` (single page)

Fetches the company's website URL (from `JobPost.CompanyURL`) and runs `Extract()` on its content. MX-verified results are added to the job's email list with `Source: "company_page"`.

### `MultiPageCompanyEnricher` (multi-page)

A superset that probes multiple pages on the company domain:

**Default page paths** (in hit-rate order):

```
/ → /about → /about-us → /about/team → /company/about
→ /team → /people → /leadership → /our-team
→ /contact → /contact-us → /get-in-touch
→ /careers → /careers/team → /jobs
```

**Subdomain probes** (only when the host is a bare domain):

```
about.<domain> → team.<domain> → careers.<domain>
→ contact.<domain> → people.<domain>
```

Each page is fetched, `Extract()` is run on the body, and candidates pass through MX verification. Non-fatal errors (404, timeout) are logged and do not prevent partial results from being returned. Concurrency is bounded by a channel-based semaphore (default 3).

---

## 3. MX Verification — `internal/email/smtp_verify.go`

### `MXVerifier`

Performs DNS MX record lookup to verify that a domain accepts mail:

```go
func (v *MXVerifier) Verify(ctx context.Context, addr string) bool {
    d := domainFrom(addr)
    mxs, err := v.Resolver.LookupMX(lookupCtx, d)
    return err == nil && len(mxs) > 0
}
```

- 10-second timeout per lookup.
- Supports `LookupMX` stub for testing.
- Nil resolver with no stub returns `true` (safe mode for offline/test).
- Returns `(verified bool, reason string)` via `VerifyEmail()` for diagnostics.

### `SMTPVerifier`

Deeper verification using the `AfterShip/email-verifier` library. Actually connects to the mail server and attempts an SMTP RCPT TO command (without sending a message):

```go
type SMTPResult struct {
    Deliverable bool   // RCPT TO returned 250
    CatchAll    bool   // server accepts any mailbox
    HasMX       bool
    HostExists  bool
    Reason      string
    Free        bool
    RoleAccount bool
    Disposable  bool
}
```

**Key caveat**: Gmail and Outlook return 250 for any well-formed address. The verifier reports `CatchAll: true` in that case.

Configured with:
- 3 concurrent workers (adjustable via `WithConcurrency`).
- 10-second connect/operation timeouts.
- Customizable EHLO/MAIL FROM identity.

---

## 4. GitHub Discovery — `internal/email/github_discover.go`

The `GitHubDiscoverer` extracts email addresses from commit author data on public GitHub repositories. Used by the `--github-scrape` CLI flag.

See [GitHub Discovery](013-GitHub-Discovery.md) for full details.

---

## 5. Pattern Permutation — `internal/email/pattern.go`

### `Permute(first, last, domain string, patterns []string) []string`

Generates candidate email addresses from a person's name and domain, using common corporate email patterns:

```
{first}.{last}     → john.doe@acme.com
{f}{last}          → jdoe@acme.com
{first}{last}      → johndoe@acme.com
{first}            → john@acme.com
{first}_{last}     → john_doe@acme.com
{f}.{last}         → j.doe@acme.com
{last}.{first}     → doe.john@acme.com
{first}-{last}     → john-doe@acme.com
{first}{l}         → john.d@acme.com
{f}{l}             → jd@acme.com
```

Patterns are ordered by statistical hit rate (based on real-world email provider data).

### `InferPattern(known map[string][2]string) string`

Given two or more known (email → firstName, lastName) samples, infers the most likely pattern for the domain:

```go
InferPattern(map[string][2]string{
    "john.doe@acme.com": {"john", "doe"},
    "jane.smith@acme.com": {"jane", "smith"},
})
// Returns: "{first}.{last}"
```

If multiple patterns match (possible with short names), the first match in `CommonPatterns()` order wins.

---

## How the pipeline fits together (in the engine)

The engine calls `enrichJobEmails()` for every job:

1. **Deduplicate**: remove any emails already set by the scraper.
2. **Text extraction**: run `Extract()` on the job description + company description.
3. **HTML extraction**: `ExtractFromHTML()` on the raw HTML before strip.
4. **Company page enrichment**: if `CompanyURL` is set, probe it for additional emails.
5. **MX verification**: for every email collected, do a DNS MX lookup (bounded to 5 concurrent lookups by default).
6. **Domain derivation**: set `JobPost.Domain` from the first email's domain or `CompanyURL`.
7. **Blocked domain filtering**: dispose of throwaway providers and platform addresses at every stage.

The result is that each `JobPost` carries a list of `model.Email` with `Addr`, `Verified`, `Source`, and `Role` fields, ready for export and quality scoring.
