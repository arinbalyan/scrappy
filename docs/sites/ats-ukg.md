# UKG (UltiPro)

## Current integration
- API/URL pattern: JSON API at `https://recruiting.ultipro.com/{slug}/OpportunitySearch`
- Seed source: `SCRAPPY_UKG_SEEDS` env var, or `config/company_slugs.yaml` entry, or `--search` term
- Parser order:
  1. JSON API response (`ukgResponse` / `opportunities` array)
- Extracted fields: title, company_name, location (city/state/country), description/short_description, department, category, job_type, date_posted, job_url, remote flag

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- Seed inputs: `--ukg-seeds` / `SCRAPPY_UKG_SEEDS`

## Constraints and breakpoints
- Slug is the UKG tenant ID — a UUID or alphanumeric identifier, not a company name
- Response uses `opportunities` array — not `jobs` or `results`
- Location is extracted from `locations[0]` or `location` field; checks `formattedAddress` and `city` for remote detection
- Company name falls back to slug if `companyName` field is empty
- Department falls back to `category` if `department` is empty
- Job ID falls back to `requisitionNumber` if `id` is empty
- Description is HTML — stripped via `util.StripHTML`
- No pagination — all opportunities returned in one response

## Debug/update playbook
1. Verify the UKG API URL resolves (e.g. `https://recruiting.ultipro.com/{slug}/OpportunitySearch`)
2. Check response structure matches `ukgResponse` types
3. Validate location extraction from both `locations` array and `location` object
4. Re-run `go test ./internal/scraper/ats-ukg` and full suite
