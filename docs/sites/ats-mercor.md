# Mercor

## Current integration
- API/URL pattern: `GET https://aws.api.mercor.com/work/listings-explore-page`
- Seed source: `SCRAPPY_MERCOR_SEEDS` env var, or `config/company_slugs.yaml` entry, or `--search` term
- Parser order:
  1. JSON API response (`mercorListingsResponse`)
  2. Seeds are used as a client-side filter (company name contains slug string)
- Extracted fields: title, company_name, location, date_posted, compensation (rate_min/rate_max, frequency, currency USD), job_url

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- Seed inputs: `--mercor-seeds` / `SCRAPPY_MERCOR_SEEDS`

## Constraints and breakpoints
- Slug filtering is a client-side substring match against `CompanyName` — not an API filter parameter
- Uses a fixed API endpoint (not slug-based per-company)
- No pagination — returns all listings in one shot
- If no seeds provided, all listings are returned unfiltered

## Debug/update playbook
1. Verify the Mercor API endpoint is still responding with valid JSON
2. Check the response structure matches `mercorListingsResponse` types
3. Validate seed-based company name filtering works as expected
4. Re-run `go test ./internal/scraper/ats-mercor` and full suite
