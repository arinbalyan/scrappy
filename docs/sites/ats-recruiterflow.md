# RecruiterFlow

## Current integration
- API/URL pattern: POST-style REST API at `https://api.recruiterflow.com/api/external/job/list`
- Seed source: `SCRAPPY_RECRUITERFLOW_SEEDS` env var, or `config/company_slugs.yaml` entry, or `--search` term
- Parser order:
  1. JSON API response (`rfAPIResponse` / `data` array)
  2. The seed slug is sent as the `RF-Api-Key` HTTP header
- Extracted fields: title, company_name (slug), location, description, date_posted, job_url

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- Seed inputs: `--recruiterflow-seeds` / `SCRAPPY_RECRUITERFLOW_SEEDS`

## Constraints and breakpoints
- The seed value is NOT a URL slug but an API key sent in the `RF-Api-Key` header
- Uses a single fixed API endpoint — not a per-company URL pattern
- No pagination — returns all jobs in one response
- Job URL is constructed as `https://recruiterflow.com/jobs/{slug}/{id}` — may not reflect actual posting URL
- Company name is set to the slug (API key), not a human-readable company name
- Description is HTML — stripped via `util.StripHTML`

## Debug/update playbook
1. Verify the RecruiterFlow API endpoint is alive (`https://api.recruiterflow.com/api/external/job/list`)
2. Test with a valid API key as the seed value
3. Check response structure matches `rfAPIResponse` types
4. Re-run `go test ./internal/scraper/ats-recruiterflow` and full suite
