# Workday

## Current integration
- API/URL pattern: POST-based REST API at `https://{company}.wd{wdNumber}.myworkdayjobs.com/wday/cxs/{company}/{site}/jobs`
- Seed source: `SCRAPPY_WORKDAY_SEEDS` env var, or `config/company_slugs.yaml` entry, or `--search` term
- Seed format: `{company}` or `{company}:{wdNumber}` or `{company}:{wdNumber}:{site}`
- Parser order:
  1. JSON POST response (`wdSearchResponse` / `jobPostings` array)
  2. Paginated — offset-based, `pageSize=20`
- Extracted fields: title, company_name, location (locationsText), department (from subtitles), date_posted, job_url (from externalPath)

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- Seed inputs: `--workday-seeds` / `SCRAPPY_WORKDAY_SEEDS`

## Constraints and breakpoints
- Uses POST with JSON payload (`wdSearchPayload`) — not a standard REST GET API
- Seed format: `{company}` defaults to `wdNumber=5`, `site=External`; `{company}:{wdNumber}:{site}` overrides all
- If seed comes from `--search` term and looks like a multi-word search phrase (contains spaces or "OR"), it returns an error — Workday needs company subdomains, not search text
- Job ID is extracted from `externalPath` via regex (`/(\d+)`) — if not found, uses the full path
- Department comes from first non-empty `subtitles[i].instances[j].text`
- Remote detection is naive — checks if `locationsText` contains "remote"
- No description is extracted (not available in list response)
- Page size is 20

## Debug/update playbook
1. Verify the Workday POST endpoint resolves (e.g. `https://{company}.wd5.myworkdayjobs.com/wday/cxs/{company}/External/jobs`)
2. Confirm POST payload structure matches `wdSearchPayload`
3. Validate URL construction from `externalPath` (with or without leading `/`)
4. Re-run `go test ./internal/scraper/ats-workday` and full suite
