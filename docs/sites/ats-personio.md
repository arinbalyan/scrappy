# Personio

## Current integration
- API/URL pattern: XML feed at `https://{slug}.jobs.personio.de/xml?language=en` (with `.com` fallback)
- Seed source: `SCRAPPY_PERSONIO_SEEDS` env var, or `config/company_slugs.yaml` entry, or `--search` term
- Parser order:
  1. XML feed (`workzag-jobs` / `position` elements)
  2. Tries `.personio.de` first, falls back to `.personio.com`
  3. Max 8 seeds
- Extracted fields: title, company_name (slug), location (office), date_posted, description (concatenated jobDescriptions), department, skills (keywords), job_url

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- Seed inputs: `--personio-seeds` / `SCRAPPY_PERSONIO_SEEDS`

## Constraints and breakpoints
- XML parsing is brittle — uses `xml.Unmarshal` against a specific `workzag-jobs` schema
- Company slug must match the Personio tenant subdomain exactly (e.g. `acme` for `acme.jobs.personio.de`)
- 700ms sleep between seed fetches to avoid rate limiting
- Max 8 seeds (hardcoded cap)
- Company name is set to the slug, not an actual company name from the feed
- Description is HTML — stripped via `util.StripHTML`

## Debug/update playbook
1. Verify the XML feed URL resolves for a given slug (try `https://{slug}.jobs.personio.de/xml?language=en`)
2. Check XML structure matches `personioXML` types
3. Validate that job descriptions are concatenated correctly from `jobDescriptions`
4. Re-run `go test ./internal/scraper/ats-personio` and full suite
