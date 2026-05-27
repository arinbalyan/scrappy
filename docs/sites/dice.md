# Dice

## Current integration
- Parser order:
  1. REST API at `https://job-search-api.svc.dhigroupinc.com/v1/dice/jobs/search` (primary)
  2. HTML scraping of `www.dice.com/jobs` (fallback)
- Extracted fields: id, jobId, title, companyName, summary, detailsPageUrl, formattedLocation, postedDate, salary, payRateRange, employmentType, isRemote

## Supported knobs
- `results_wanted`
- `search_term` — required, server-side
- `location` — server-side
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)

## Constraints and breakpoints
- **Search term is required** — empty search returns an error.
- API uses a publicly known `x-api-key` (`1YAt0R9wBg4WfsF9VB2778F5CHLAPMVW3WAZcKd8`) — if this key is revoked, the API path will fail and fall back to HTML scraping.
- Rate-limited to ~3 requests/second (333ms interval via ticker).
- Pagination: page-based (20 per page), up to 50 pages.
- HTML fallback uses regex patterns for `data-cy` attributes — fragile if DOM structure changes.
- Salary parsing from both structured `payRateRange` and raw string `salary` field.
- Date parsing for both ISO 8601 (`postedDate`) and relative text (`"Posted 2 days ago"`).

## Debug/update playbook
1. Check if the `x-api-key` is still valid for the Dice API.
2. If API fails, verify HTML fallback `data-cy` attribute patterns still match.
3. Validate salary range parsing for new formats.
4. Re-run `go test ./tests/scraper/dice` and full suite.
