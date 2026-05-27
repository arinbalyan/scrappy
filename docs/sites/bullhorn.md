# Bullhorn (ATS)

## Current integration
- Parser order:
  1. JSON API at `https://public-rest{cls}.bullhornstaffing.com/rest-services/{corpToken}/search/JobOrder`
  2. Company seeds from env, config file, or search term
- Extracted fields: id, title, publicDescription, address, dateAdded, salary, salaryUnit, employmentType, categories

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)

## Seed configuration
- **Env:** `SCRAPPY_BULLHORN_SEEDS` — comma-separated seeds in format `cls:corpToken`
- **Config:** `config/company_slugs.yaml` under `bullhorn` key
- **Search fallback:** `--search` passed as slug (must include `:` separator)
- Seeds are capped at `SCRAPPY_ATS_MAX_SEEDS` (default 20)

## Constraints and breakpoints
- Slug format is `{cls}:{corpToken}` where `cls` is the cluster identifier and `corpToken` is the company's corporate token.
- API queries for open job orders via `(isOpen:1)`.
- Location comes from `Address` object (city, state, country).
- Salary unit is mapped via `mapSalaryUnit`: "per hour" → hourly, "per month" → monthly, etc.
- Department extracted from `Categories` array (first category name).
- Date is in epoch milliseconds (`dateAdded`).

## Debug/update playbook
1. Verify seed format `cls:corpToken` is correct.
2. Check the Bullhorn REST API responds for the specific corpToken.
3. Validate salary unit mapping for new unit strings.
4. Re-run `go test ./tests/scraper/bullhorn` and full suite.
