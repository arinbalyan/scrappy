# Quality Scoring

`internal/quality/` — deterministic 0-100 score computed per posting, no LLM needed.

## Formula

```
Salary mentioned                +20  (hasSalary: Compensation.MinAmount or MaxAmount > 0)
Direct apply link present       +15  (hasDirectApply: ApplyMethod in easy_apply|email|direct_url|external_url)
Email domain == company domain  +15  (emailMatchesCompanyDomain: any email @domain matches job.Domain)
Freshness (scaled)              +15  (< 24h=15, < 72h=10, < 7d=5, older=0)
At least one verified email     +10  (verifiedEmailScore: any Email with Verified=true)
Description length (scaled)     +10  (> 2000=10, > 500=7, > 200=5)
NOT a staffing/agency posting   +10  (!isAgency: domain not in blocklist AND company name has no agency tokens)
Two or more distinct emails     + 5  (multipleEmails: >= 2 unique addresses)
                               _____
                           Total 100
```

## Agency/staffing blocklist

Hardcoded in `internal/quality/score.go`. Domain-based detection:

```
aerotek.com
adecco.com
ciber.com
collateraledge.com
experis.com
hays.com
insightglobal.com
kellyservices.com
kforce.com
manpower.com
michaelpage.com
modis.com
randstad.com
roberthalf.com
robertwalters.com
spencerogden.com
teksystems.com
```

Additionally, a company name containing any of the tokens `staffing`, `recruiting`, `recruitment`, `agency`, `talent`, `workforce`, or `placement` is flagged as an agency.

## Race safety

The quality scoring implementation uses concurrent maps with atomic counters to ensure thread safety when scores are computed across multiple goroutines. Always validate with `go test -race ./...` when modifying scoring logic.

## Domain match logic

The `emailMatchesCompanyDomain` factor relies on domain population from the email pipeline. After email extraction, the email domain is stored on `JobPost.Domain`. The quality scorer compares each email's domain against `JobPost.Domain`. A match yields +15 points.

See [010-Email.md](010-Email.md) for the email pipeline details.

## Usage

```go
score := quality.Score(job)
if score < 60 {
    continue // skip, too low quality
}
```

## CLI flag

```
--min-score 60     # Drop postings below this threshold before export
```

After filtering by `--min-score`, remaining jobs are passed to the export pipeline. In library mode, call `quality.Score()` per job or rely on the engine (which computes scores automatically).

```go
// The engine sets JobPost.QualityScore automatically during Scrape().
// You only need to call quality.Score() directly if using the scraper
// package without the engine.
```

## Developer notes

- Scores are computed after dedup, before filtering
- The engine applies `--min-score` after cross-site aggregation, so the input to the score filter includes deduplicated jobs from all sites
- There is no caching — scores are computed once per job per scrape run
- The 8 factors are independent; a job can score 100 if all criteria are met
- `Score(nil)` returns 0 safely
