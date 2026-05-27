# ADP (ATS)

## Current integration
- Parser order:
  1. JSON API at `https://workforcenow.adp.com/mascsr/default/careercenter/public/events/staffing/v1/job-requisitions`
  2. Company seeds from env, config file, or search term
- Extracted fields: jobRequisitionId, jobTitle, jobDescription, departmentName, location, compensation, postedDate, employmentType, companyName

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)

## Seed configuration
- **Env:** `SCRAPPY_ADP_SEEDS` — comma-separated company slugs (ADP cid parameters)
- **Config:** `config/company_slugs.yaml` under `adp` key
- **Search fallback:** `--search` passed as slug if no env/config entry exists
- Seeds are capped at `SCRAPPY_ATS_MAX_SEEDS` (default 20)

## Constraints and breakpoints
- ADP uses a tenant-specific `cid` query parameter derived from the company slug.
- Location comes from `Locations` array (preferred) or single `Location` object.
- Remote detection is heuristic: looks for "remote" in formatted address or city.
- Job URL construction falls back to ADP's recruitment page if `externalUrl` is empty.
- Employment type is normalized from multiple field sources (`employmentType`, `workerTypeCode`).

## Debug/update playbook
1. Verify company slug exists in env var, config file, or valid search term.
2. Check the API endpoint responds for the specific tenant's cid.
3. Validate location parsing for new location formats.
4. Re-run `go test ./tests/scraper/adp` and full suite.
