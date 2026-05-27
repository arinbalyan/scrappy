# Teamtailor

## Current integration
- API/URL pattern: JSON API at `https://career.teamtailor.com/widget/jobs/{slug}`
- Seed source: `SCRAPPY_TEAMTAILOR_SEEDS` env var, or `config/company_slugs.yaml` entry, or `--search` term
- Parser order:
  1. JSON API response (`ttResponse` / `data` array) — JSON:API format
- Extracted fields: title, company_name (slug), location (city/region/country), description (body), department, remote flag, employment_type, date_posted, job_url (careersite-url / apply-url / external-url)

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- Seed inputs: `--teamtailor-seeds` / `SCRAPPY_TEAMTAILOR_SEEDS`

## Constraints and breakpoints
- Slug must match the Teamtailor tenant subdomain (e.g. `acme` for `career.teamtailor.com/acme/jobs`)
- Uses JSON:API format with `data` array and `attributes`/`relationships` inside
- No pagination — all jobs returned in one response
- Job URL resolution priority: `links.careersite-url` > `attributes.apply-url` > `attributes.external-url` > constructed fallback
- Department is extracted from `relationships.department.data.id` — not a name label
- Description is HTML — stripped via `util.StripHTML`

## Debug/update playbook
1. Verify the Teamtailor API URL resolves (e.g. `https://career.teamtailor.com/widget/jobs/{slug}`)
2. Check JSON:API response structure matches `ttResponse` types
3. Validate relationship data parsing for department
4. Re-run `go test ./internal/scraper/ats-teamtailor` and full suite
