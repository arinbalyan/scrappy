# SuccessFactors

## Current integration
- API/URL pattern: OData v2 API at `https://{instance}.successfactors.com/odata/v2/JobRequisitionPosting`
- Seed source: `SCRAPPY_SUCCESSFACTORS_SEEDS` env var, or `config/company_slugs.yaml` entry, or `--search` term
- Seed format: `{instance}` or `{instance}:{companyId}`
- Parser order:
  1. OData v2 JSON response (`sfODataResponse` / `d.results`)
  2. Paginated — `$top=20`, `$skip`-based continuation
- Extracted fields: title (jobTitle/formattedJobTitle), company_name (companyName), location (locationObj: city/state/country), department, employment_type, date_posted (postingStartDate), job_url (externalJobUrl), description

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- Seed inputs: `--successfactors-seeds` / `SCRAPPY_SUCCESSFACTORS_SEEDS`

## Constraints and breakpoints
- Seed format: `{instance}` uses instance as both SAP instance subdomain and company ID; `{instance}:{companyId}` allows separate company ID
- Uses OData v2 with `$select`, `$top`, `$skip`, `$orderby`, `$format=json` parameters
- Job description is not extracted (jobDescription column is read but not included in output)
- External job URL falls back to constructed URL if not provided
- Remote detection is naive — checks if location City contains "remote"
- Page size is 20 — smaller than most ATS scrapers

## Debug/update playbook
1. Verify the OData endpoint resolves (e.g. `https://{instance}.successfactors.com/odata/v2/JobRequisitionPosting`)
2. Check OData response structure matches `sfODataResponse` types
3. Validate URL construction for instances without external URL
4. Re-run `go test ./internal/scraper/ats-successfactors` and full suite
