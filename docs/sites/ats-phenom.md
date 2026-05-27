# Phenom

## Current integration
- API/URL pattern: JSON API at `https://jobs.{slug}.com/api/jobs?offset=0&limit=25`
- Seed source: `SCRAPPY_PHENOM_SEEDS` env var, or `config/company_slugs.yaml` entry, or `--search` term
- Parser order:
  1. JSON API response (`phenomResponse`)
  2. No pagination — single fetch per seed
- Extracted fields: title, company_name, location (city/state/country), description, short_description, department, category, employment_type, date_posted, job_url, apply_url, remote flag

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- Seed inputs: `--phenom-seeds` / `SCRAPPY_PHENOM_SEEDS`

## Constraints and breakpoints
- URL pattern `jobs.{slug}.com` means the slug must match the company's Phenom careers subdomain exactly
- Many Phenom instances block non-browser traffic — the error message recommends using residential proxies
- No pagination — only first 25 results per seed
- `postedDate` is a float that may be epoch seconds or milliseconds — auto-detected
- Remote detection checks multiple fields: `isRemote` boolean, `workplaceType`, `locationText`, `type`, and title
- Company name falls back to slug if not provided in response

## Debug/update playbook
1. Verify the Phenom API URL resolves (e.g. `https://jobs.{slug}.com/api/jobs?offset=0&limit=25`)
2. Check response structure matches `phenomResponse` types
3. If blocked, test with a residential proxy
4. Re-run `go test ./internal/scraper/ats-phenom` and full suite
