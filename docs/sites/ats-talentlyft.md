# TalentLyft

## Current integration
- API/URL pattern: JSON API at `https://api.talentlyft.com/v2/public/{slug}/jobs?page=1&perPage={n}`
- Seed source: `SCRAPPY_TALENTLYFT_SEEDS` env var, or `config/company_slugs.yaml` entry, or `--search` term
- Parser order:
  1. JSON API response (`tlResponse` / `Results` array)
  2. No pagination — single fetch per seed
- Extracted fields: title, company_name (slug), location, description, department, date_posted, job_url

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- Seed inputs: `--talentlyft-seeds` / `SCRAPPY_TALENTLYFT_SEEDS`

## Constraints and breakpoints
- Slug must match the TalentLyft tenant identifier
- No pagination — single request with `perPage` set to `results_wanted`
- Company name is set to slug, not an actual company name from the response
- Job URL is taken directly from the `Url` field in the response; may be empty
- Description is HTML — stripped via `util.StripHTML`
- Field names use PascalCase (e.g., `Id`, `Title`, `Description`) which is unusual for JSON APIs

## Debug/update playbook
1. Verify the TalentLyft API URL resolves (e.g. `https://api.talentlyft.com/v2/public/{slug}/jobs`)
2. Check response structure matches `tlResponse` types
3. Confirm PascalCase field mapping matches actual API response
4. Re-run `go test ./internal/scraper/ats-talentlyft` and full suite
