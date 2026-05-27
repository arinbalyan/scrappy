# jobscore

## Current integration
- Parser order:
  1. ATS API: https://careers.jobscore.com/jobs
- Extracted fields: title, description, location, department, employment type

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- Company slug seed via `SCRAPPY_JOBSCORE_SEEDS` env var, `config/company_slugs.yaml`, or `--search` as fallback

## Seed resolution
- `ats.ResolveSeeds` with key `SCRAPPY_JOBSCORE_SEEDS`
- Priority: env var > config file (yaml) > search term
- Max seeds capped by `SCRAPPY_ATS_MAX_SEEDS` (default 20)

## Constraints and breakpoints
- Requires company slug seed or config entry; empty results if no seeds available.
- ATS scraper. No auth required for public API. Uses ats.ResolveSeeds via SCRAPPY_JOBSCORE_SEEDS or config/company_slugs.yaml.

## Debug/update playbook
1. Verify API endpoint is reachable and returns expected JSON.
2. Check that `SCRAPPY_JOBSCORE_SEEDS` or the config file has target company slugs.
3. Inspect job response structure for field name changes.
4. Re-run `go test ./tests/scraper/jobscore` and full suite.
