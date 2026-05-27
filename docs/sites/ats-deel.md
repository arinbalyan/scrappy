# ats-deel

## Current integration
- Parser order:
  1. ATS API: https://api.letsdeel.com/rest/v2/ats/job-postings
- Extracted fields: title, company, location, description, site identifier

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- Company slug seed via `SCRAPPY_DEEL_SEEDS` env var, `config/company_slugs.yaml`, or `--search` as fallback

## Seed resolution
- `ats.ResolveSeeds` with key `SCRAPPY_DEEL_SEEDS`
- Priority: env var > config file (yaml) > search term
- Max seeds capped by `SCRAPPY_ATS_MAX_SEEDS` (default 20)

## Constraints and breakpoints
- Requires company slug seed or config entry; empty results if no seeds available.
- Deel ATS API. Requires DEEL_API_TOKEN env var as Bearer token. Returns job postings with location, compensation, department, and remote status.

## Debug/update playbook
1. Verify API endpoint is reachable and returns expected JSON.
2. Check that `SCRAPPY_DEEL_SEEDS` or the config file has target company slugs.
3. Inspect job response structure for field name changes.
4. Re-run `go test ./tests/scraper/deel` and full suite.
