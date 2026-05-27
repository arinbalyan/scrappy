# Pinpoint

## Current integration
- API/URL pattern: JSON API at `https://{slug}.pinpointhq.com/postings.json`
- Seed source: `SCRAPPY_PINPOINT_SEEDS` env var, or `config/company_slugs.yaml` entry, or `--search` term
- Parser order:
  1. JSON API response — accepts both `{"data": [...]}` wrapper and bare array
- Extracted fields: title, company_name, location, description, department, remote flag, date_posted, job_url

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- Seed inputs: `--pinpoint-seeds` / `SCRAPPY_PINPOINT_SEEDS`

## Constraints and breakpoints
- Pinpoint can return `{"data": [...]}` or a bare JSON array — both formats are parsed
- Seeds are iterated sequentially, one slug per company
- No pagination in the current implementation
- Date posts from `published_at` or fallback to `created_at`
- Company name falls back to slug if not provided
- Description is HTML — stripped via `util.StripHTML`

## Debug/update playbook
1. Verify the Pinpoint API URL resolves (e.g. `https://{slug}.pinpointhq.com/postings.json`)
2. Check response format (wrapped or bare array) and validate against `pinpointResponse`/array types
3. Confirm date parsing works (ISO 8601 format expected)
4. Re-run `go test ./internal/scraper/ats-pinpoint` and full suite
