# Ashby (ATS)

## Current integration
- Parser order:
  1. JSON API at `https://api.ashbyhq.com/posting-api/job-board/{slug}`
  2. Company seeds from env, config file, or search term
- Extracted fields: id, title, departmentName, teamName, employmentType, location, address, isRemote, publishedDate, descriptionHtml, descriptionPlain, jobUrl, applyUrl, compensation

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)

## Seed configuration
- **Env:** `SCRAPPY_ASHBY_SEEDS` — comma-separated company slugs
- **Config:** `config/company_slugs.yaml` under `ashby` key
- **Search fallback:** `--search` passed as slug if no env/config entry exists
- Seeds are capped at `SCRAPPY_ATS_MAX_SEEDS` (default 20)

## Constraints and breakpoints
- API endpoint follows the pattern `api.ashbyhq.com/posting-api/job-board/{slug}`.
- Jobs with `isListed=false` are skipped.
- Location comes from structured `postalAddress` object (locality, region, country) with fallback to flat `location` string.
- Compensation is extracted from `compensationComponents` (preferring salary/base tier) or `summaryComponents`.
- Description uses `descriptionPlain` with fallback to stripped `descriptionHtml`.
- `PublishedDate` is parsed via `util.ParseDatePosted`.

## Debug/update playbook
1. Verify company slug exists in env var or config file.
2. Check the API endpoint responds for the specific tenant.
3. Validate compensation extraction for new component structures.
4. Re-run `go test ./tests/scraper/ashby` and full suite.
