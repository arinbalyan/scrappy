# Quality Scoring

`internal/quality/` — deterministic 0–100 score computed per posting, no LLM needed.

## Formula

```
Salary mentioned                +30    (hasSalary: Compensation.MinAmount or MaxAmount > 0)
Direct apply link present       +20    (hasDirectApply: ApplyMethod in easy_apply|email|direct_url|external_url)
Email domain == company domain  +15    (emailMatchesCompanyDomain: any email @domain matches job.Domain)
Posted within last 24h          +15    (isFresh: DatePosted within 24h of now)
Description length > 200 chars  +10    (len(trim(Description)) > 200)
NOT a staffing/agency posting   +10    (!isAgency: domain not in blocklist AND company name has no staffing/recruiting/recruitment tokens)
                                   ────
                             Total 100
```

## Agency/staffing blocklist

Hardcoded in `internal/quality/score.go`. Domain-based detection:

```
randstad.com
manpower.com
adecco.com
kellyservices.com
hays.com
michaelpage.com
robertwalters.com
spencerogden.com
```

Additionally, a company name containing any of the tokens `staffing`, `recruiting`, or `recruitment` is flagged as an agency.

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
- The 6 factors are independent; a job can score 100 if all criteria are met
