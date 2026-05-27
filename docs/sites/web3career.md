# web3career

## Current integration
- Parser order:
  1. API: https://web3.career/api/v1
- Extracted fields: title, company, description, location, salary, remote

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- `search_term`, `location`, `country`
- no auth required

## Constraints and breakpoints
- Web3/Cryptocurrency job board API. No auth required. Crypto/web3 focused positions.

## Debug/update playbook
1. Test the API endpoint directly with expected parameters.
2. Verify JSON response structure matches expected fields.
3. Check rate limits and pagination parameters.
4. Re-run `go test ./tests/scraper/web3career` and full suite.
