# Rippling

## Current integration
- URL pattern: HTML page at `https://ats.rippling.com/{slug}/jobs`
- Seed source: `SCRAPPY_RIPPLING_SEEDS` env var, or `config/company_slugs.yaml` entry, or `--search` term
- Parser order:
  1. HTML page parsing — extracts `__NEXT_DATA__` JSON script tag
  2. Navigates deeply into `props.pageProps.dehydratedState.queries[*].state.data.items`
- Extracted fields: title, company_name, location (city/state/country, workplaceType), description (role + company fields), department, employment_type, date_posted, job_url, compensation (pay_range min/max/currency/interval), remote flag

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- Seed inputs: `--rippling-seeds` / `SCRAPPY_RIPPLING_SEEDS`

## Constraints and breakpoints
- Depends on `__NEXT_DATA__` JSON script tag — any Next.js build change can break parsing
- The `dehydratedState` structure is deeply nested and fragile across Rippling deployments
- Multiple queries are searched; first one with items wins
- Description is a map with `role`, `company` keys — concatenated with double newline
- Compensation interval auto-detected from `payRangeDetails[0].interval`
- Remote detection checks `workplaceType` from locations and `workLocations` array
- No pagination — all jobs extracted from the embedded state

## Debug/update playbook
1. Fetch the Rippling career page HTML for a slug
2. Search for `__NEXT_DATA__` script tag and validate JSON structure
3. Trace `props.pageProps.dehydratedState.queries[*].state.data.items` path
4. Re-run `go test ./internal/scraper/ats-rippling` and full suite
