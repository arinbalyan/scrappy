# ats-gem

## Current integration
- Parser order:
  1. ATS API: https://jobs.gem.com/api/public/graphql/batch
- Extracted fields: title, company, location, description, site identifier

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- Company slug seed via `SCRAPPY_GEM_SEEDS` env var, `config/company_slugs.yaml`, or `--search` as fallback

## Seed resolution
- `ats.ResolveSeeds` with key `SCRAPPY_GEM_SEEDS`
- Priority: env var > config file (yaml) > search term
- Max seeds capped by `SCRAPPY_ATS_MAX_SEEDS` (default 20)

## Constraints and breakpoints
- Requires company slug seed or config entry; empty results if no seeds available.
- Gem GraphQL API. Two-step: board theme query then job listing query. No auth required. Returns jobs with location, remote status, salary, and department.

## Debug/update playbook
1. Verify API endpoint is reachable and returns expected JSON.
2. Check that `SCRAPPY_GEM_SEEDS` or the config file has target company slugs.
3. Inspect job response structure for field name changes.
4. Re-run `go test ./tests/scraper/gem` and full suite.
