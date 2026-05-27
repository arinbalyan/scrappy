# Taleo

## Current integration
- API/URL pattern: POST-based REST API at `https://{company}.taleo.net/careersection/rest/jobboard/searchjobs`
- Seed source: `SCRAPPY_TALEO_SEEDS` env var, or `config/company_slugs.yaml` entry, or `--search` term
- Seed format: `{company}` or `{company}:{careerSection}`
- Parser order:
  1. JSON POST response (`taleoSearchResponse` / `requisitionList`)
  2. Paginated — `pageNo`-based, `pageSize=25`
- Extracted fields: title, company_name (organization), location (primaryLocation), department (jobField), date_posted (postingDate/openingDate), job_url

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- Seed inputs: `--taleo-seeds` / `SCRAPPY_TALEO_SEEDS`

## Constraints and breakpoints
- Uses POST with JSON payload — not a standard REST GET API
- Seed format: `{company}` uses default career section `ExternalCareerSite`; `{company}:{careerSection}` allows override
- Sorting is hardcoded to `postedDate` descending
- Search keyword and location from `ScraperInput` are sent in the POST body
- Job URL is constructed with `jobdetail.ftl?job={contestNo}` — may need updating if Taleo changes URL scheme
- Company name falls back to company seed if `organization` field is empty
- Remote detection is naive — checks if location string contains "remote"

## Debug/update playbook
1. Verify the Taleo search POST endpoint resolves (e.g. `https://{company}.taleo.net/careersection/rest/jobboard/searchjobs`)
2. Confirm POST payload structure matches `taleoSearchPayload`
3. Validate response `requisitionList` structure
4. Re-run `go test ./internal/scraper/ats-taleo` and full suite
