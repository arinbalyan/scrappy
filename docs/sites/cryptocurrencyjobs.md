# CryptocurrencyJobs

## Current integration
- Parser order:
  1. RSS feed at `https://cryptocurrencyjobs.co/index.xml`
- Extracted fields: title, link, guid, description, pubDate, category

## Supported knobs
- `results_wanted`
- `search_term` — client-side filter on title + description + category (OR semantics)
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)

## Constraints and breakpoints
- Single RSS feed fetch — no pagination.
- Search term supports OR semantics.
- Company name extracted from title using regex `at (.+)$` (e.g., "Software Engineer at Coinbase").
- Description is HTML — stripped to plain text.

## Debug/update playbook
1. Verify the RSS feed URL at `cryptocurrencyjobs.co/index.xml`.
2. Validate `<item>` structure (title, link, guid, description, pubDate).
3. Check company extraction regex for new title formats.
4. Re-run `go test ./tests/scraper/cryptocurrencyjobs` and full suite.
