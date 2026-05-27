# CareerJet

## Current integration
- Parser order:
  1. JSON API at `https://public.api.careerjet.net/search`
- Extracted fields: title, company, date, description, locations, url, salary_min, salary_max, salary_type, salary_currency_code

## Supported knobs
- `results_wanted`
- `search_term` — required, server-side via `keywords` param
- `location` — server-side via `location` param
- `country` — mapped to locale codes (en_US, en_CA, en_GB, de_DE, fr_FR, en_IN, en_AU)
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)

## Constraints and breakpoints
- **Requires `CAREERJET_AFFID`** environment variable (affiliate ID).
- **Search term is required** — empty search returns an error.
- Rate-limited to ~3 requests/second (350ms minimum interval via ticker).
- Pagination: page-based, up to 10 pages, with page size of 50.
- Salary interval codes: Y=yearly, M=monthly, W=weekly, D=daily, H=hourly.
- Location is parsed as "City, State" format.

## Debug/update playbook
1. Verify `CAREERJET_AFFID` is set.
2. Confirm the API at `public.api.careerjet.net/search` responds.
3. Check salary type code mapping for any new codes.
4. Validate locale mapping for new country codes.
5. Re-run `go test ./tests/scraper/careerjet` and full suite.
