# RemoteOK

## Current integration
- Endpoint: `https://remoteok.com/api`
- Response shape: array where index 0 is metadata, remaining rows are jobs.
- Extracted fields: id, position, company, url, epoch/date.

## Supported knobs
- `results_wanted`
- global resilience knobs: `retries`, `max_rps`, `site_rps`, proxy config

## Constraints and breakpoints
- Metadata-row behavior is a schema contract assumption.
- Fields can arrive as mixed/null types.
- API can return non-job rows during incidents.

## Debug/update playbook
1. Confirm index-0 metadata behavior still holds.
2. Validate required fields before mapping each row.
3. Guard mixed/null numeric values (`id`, `epoch`) and keep tolerant casts.
4. Re-run `go test ./tests/scraper/remoteok` and then full suite.
