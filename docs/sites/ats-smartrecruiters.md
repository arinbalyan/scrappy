# SmartRecruiters

## Current integration
- API/URL pattern: REST API at `https://api.smartrecruiters.com/v1/companies/{slug}/postings?offset={n}&limit=100`
- Seed source: `SCRAPPY_SMARTRECRUITERS_SEEDS` env var, or `config/company_slugs.yaml` entry, or `--search` term
- Parser order:
  1. JSON API response (`srResponse` / `content` array)
  2. Paginated — `pageSize=100`, offset-based continuation
- Extracted fields: title, company_name, location (city/region/country, remote boolean), description (jobDescription + qualifications + additionalInformation sections), department, employment_type, date_posted, job_url (via ref field)

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- Seed inputs: `--smartrecruiters-seeds` / `SCRAPPY_SMARTRECRUITERS_SEEDS`

## Constraints and breakpoints
- Slug must match the SmartRecruiters company identifier (case-sensitive in URL)
- Paginated with 100-per-page limit — loops until fewer items returned or results_wanted reached
- Description is assembled from up to 3 sections: `jobDescription`, `qualifications`, `additionalInformation`
- Each section has a `title` and `text` field; only `text` is used
- Description text is HTML — stripped via `util.StripHTML`
- Job URL defaults to `https://jobs.smartrecruiters.com/{slug}/{id}` if `ref` field is empty

## Debug/update playbook
1. Verify the SmartRecruiters API URL resolves (e.g. `https://api.smartrecruiters.com/v1/companies/{slug}/postings`)
2. Check response structure matches `srResponse` types
3. Validate description assembly from jobAd sections
4. Re-run `go test ./internal/scraper/ats-smartrecruiters` and full suite
