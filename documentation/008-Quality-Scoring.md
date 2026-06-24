# Quality Scoring

The quality score is a **deterministic 0-100** metric for every job posting. It runs entirely in Go — no LLM calls, no external APIs. The score is computed in the engine's post-processing pipeline and stored as `JobPost.QualityScore`.

## Formula

| Points | Factor | Description |
|--------|--------|-------------|
| 20 | **Salary** | Non-zero minimum or maximum amount listed |
| 15 | **Direct apply** | Apply method is `easy_apply`, `email`, `direct_url`, or `external_url` |
| 15 | **Email matches domain** | At least one email's domain matches the company domain or is a subdomain |
| 15 | **Freshness** | Scaled: < 24h = 15, < 72h = 10, < 7d = 5, older = 0 |
| 10 | **Verified email** | At least one MX-verified email address |
| 10 | **Description length** | Scaled: > 2000 chars = 10, > 500 = 7, > 200 = 5 |
| 10 | **Not an agency** | Domain and company name checked against known staffing/recruiting firms |
| 5 | **Multiple emails** | Two or more distinct email addresses |
| **100** | **Total** | |

---

## Component breakdown

### Salary (20 pts)

```go
func hasSalary(job *model.JobPost) bool {
    if job.Compensation == nil {
        return false
    }
    if job.Compensation.MinAmount != nil && *job.Compensation.MinAmount > 0 {
        return true
    }
    return job.Compensation.MaxAmount != nil && *job.Compensation.MaxAmount > 0
}
```

Requires a non-zero amount in either `min_amount` or `max_amount`. Zero or nil compensation → no points. The interval (yearly/hourly/daily) does not affect scoring.

### Direct apply (15 pts)

Recognized apply methods:
- `easy_apply` — LinkedIn/Indeed one-click apply
- `email` — apply by sending an email
- `direct_url` — external application URL
- `external_url` — alternate external URL

### Email-domain match (15 pts)

Compares the domain portion of each email against `JobPost.Domain`. Also accepts subdomains (e.g. `engineering@sub.acme.com` matches `acme.com`). Requires at least one email address AND a non-empty domain.

### Freshness (15 pts)

```go
func freshnessScore(job *model.JobPost) int {
    diff := time.Since(*job.DatePosted)
    switch {
    case diff < 0:           return 0  // future date
    case diff <= 24*time.Hour:  return 15
    case diff <= 72*time.Hour:  return 10
    case diff <= 7*24*time.Hour: return 5
    default:                 return 0
    }
}
```

Future dates (clock skew, timezone mismatch) and nil dates score 0.

### Verified email (10 pts)

Grants points when at least one email in the job has `Verified: true`, meaning it passed DNS MX record lookup. Emails that fail MX verification, or jobs with no emails at all, score 0.

### Description length (10 pts)

- `> 2000` characters → 10 points
- `> 500` characters → 7 points
- `> 200` characters → 5 points
- `<= 200` characters → 0 points

### Not an agency (10 pts)

scrappy maintains a list of known staffing/recruiting agencies:

```
aerotek.com, adecco.com, hays.com, kellyservices.com,
kforce.com, manpower.com, randstad.com, roberthalf.com,
robertwalters.com, teksystems.com, ...
```

In addition, the company name is checked for tokens: `staffing`, `recruiting`, `recruitment`, `agency`, `talent`, `workforce`, `placement`.

Jobs from these firms lose 10 points. The check is case-insensitive and token-aware (whole-word match, not substring).

### Multiple emails (5 pts)

Bonus points for jobs with two or more distinct email addresses. Deduplicated by address string (case-insensitive).

---

## Usage

Filter results by minimum score:

```bash
scrappy --sites remoteok --search "golang" --min-score 50
```

Only jobs scoring 50+ pass through. The score is also written to every export format (CSV, JSONL, XLSX, Parquet) under the `quality_score` column, so post-processing tools can filter independently.

The score always falls within 0-100. If the sum exceeds 100 (not possible with the current weights), it is clamped. Negative scores are clamped to 0.
