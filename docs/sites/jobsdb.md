# jobsdb

## Current integration
- Parser order:
  1. API (SEEK): https://www.seek.com.au/api/jobsearch/v5/search
- Extracted fields: title, company, description, location, salary, classification, sub-classification, date

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- `search_term`, `location`, `country`
- none

## Constraints and breakpoints
- SEEK-powered API for JobsDB (Singapore/SE Asia). Uses SEEK API with siteKey=SG-Main. Supports rich search including classification filters.

## Debug/update playbook
1. Test the API endpoint directly with expected parameters.
2. Verify JSON response structure matches expected fields.
3. Check rate limits and pagination parameters.
4. Re-run `go test ./tests/scraper/jobsdb` and full suite.
