# Exa

## Current integration
- Parser order:
  1. POST API at `https://api.exa.ai/search`
- Extracted fields: url, title, author, text, summary, publishedDate

## Supported knobs
- `results_wanted`
- `search_term` — combined with location and remote flag in the query
- `location` — appended to the search query
- `is_remote` — adds "remote" to the query
- `hours_old` — passed as `startPublishedDate` filter
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)

## Constraints and breakpoints
- **Requires `EXA_API_KEY`** environment variable (set via `NewWithAPIKey` or env var).
- API is a search engine, not a job board — results may include non-job content filtered by `IncludeDomains`.
- Uses a fixed list of job domains (linkedin.com, indeed.com, glassdoor.com, etc.) for filtering.
- Company name extracted from `author` field or URL pattern matching (greenhouse, ashby, lever, workable).
- Title falls back to URL path extraction if empty.
- Remote detection is heuristic: checks for "remote", "work from home", "wfh" in title or description.

## Debug/update playbook
1. Verify `EXA_API_KEY` is set.
2. Check the API at `api.exa.ai/search` responds.
3. Validate domain filter list for new sources.
4. Test company name extraction from various ATS URL patterns.
5. Re-run `go test ./tests/scraper/exa` and full suite.
