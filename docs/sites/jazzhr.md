# jazzhr

## Current integration
- Parser order:
  1. ATS API: https://api.jazzhr.com/v1/{slug}/jobs
- Extracted fields: title, description, location, department, employment type, compensation

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- Company slug seed via `SCRAPPY_JAZZHR_SEEDS` env var, `config/company_slugs.yaml`, or `--search` as fallback

## Seed resolution
- `ats.ResolveSeeds` with key `SCRAPPY_JAZZHR_SEEDS`
- Priority: env var > config file (yaml) > search term
- Max seeds capped by `SCRAPPY_ATS_MAX_SEEDS` (default 20)

## Constraints and breakpoints
- Requires company slug seed or config entry; empty results if no seeds available.
- ATS scraper. Requires JAZZHR_API_KEY env var. Uses ats.ResolveSeeds via SCRAPPY_JAZZHR_SEEDS or config/company_slugs.yaml. API key sent via header.

## Debug/update playbook
1. Verify API endpoint is reachable and returns expected JSON.
2. Check that `SCRAPPY_JAZZHR_SEEDS` or the config file has target company slugs.
3. Inspect job response structure for field name changes.
4. Re-run `go test ./tests/scraper/jazzhr` and full suite.
