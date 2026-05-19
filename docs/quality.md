# Quality Scoring

`internal/quality/` — deterministic 0–100 score computed per posting, no LLM needed.

## Formula

```
Salary mentioned                +30
Direct apply link present       +20
Email domain == company domain  +15
Posted within last 24h          +15
Description length > 200 chars  +10
NOT a staffing/agency posting   +10
                                  ────
                            Total 100
```

## Staffing/agency blocklist

`internal/quality/agency_domains.txt` — flat file, one domain per line. Updated at build time via `go:embed`. Domains matched by suffix:

```
roberthalf.com
kellyservices.com
randstadusa.com
...
```

## Usage

```go
score := quality.ComputeScore(job)
if score < 60 {
    continue // skip, too low quality
}
```

## CLI flag

```
--min-score 60     # Drop postings below this threshold before export
```

After filtering by `--min-score`, remaining jobs are passed to the export pipeline and, if JobHunter is orchestrating, to the LLM reranker.
