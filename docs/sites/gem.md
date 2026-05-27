# Gem (ATS)

## Current integration
- Parser order:
  1. GraphQL batch API at `https://jobs.gem.com/api/public/graphql/batch`
  2. Company seeds from env, config file, or search term
- Extracted fields: id, extId, title, locations, job (department, locationType, employmentType)

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)

## Seed configuration
- **Env:** `SCRAPPY_GEM_SEEDS` — comma-separated company slugs (board IDs)
- **Config:** `config/company_slugs.yaml` under `gem` key
- **Search fallback:** `--search` passed as slug if no env/config entry exists
- Seeds are capped at `SCRAPPY_ATS_MAX_SEEDS` (default 20)

## Constraints and breakpoints
- Uses a GraphQL batch query with two operations: `JobBoardTheme` and `JobBoardList`.
- Company slug is the `boardId` parameter passed in GraphQL variables.
- Requests are sent with `batch: true` header.
- Response is a batch envelope array — the `JobBoardList` response is picked by checking for `oatsExternalJobPostings` data.
- Company name comes from `jobBoardExternal.teamDisplayName` (preferred) or falls back to slug.
- Location first location in the `Locations` array.
- Remote detection from `Location.isRemote` or `Job.locationType` containing "remote".
- Department from `Job.Department.Name`.
- No description field is returned by the API.

## Debug/update playbook
1. Verify company slug (boardId) is valid.
2. Check the GraphQL batch endpoint at `jobs.gem.com/api/public/graphql/batch`.
3. Validate GraphQL response structure for any schema changes.
4. Re-run `go test ./tests/scraper/gem` and full suite.
