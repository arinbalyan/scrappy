# Oracle

## Current integration
- API/URL pattern: REST API at `https://{subdomain}.fa.{region}.oraclecloud.com/hcmRestApi/resources/latest/recruitingCEJobRequisitions`
- Seed source: `SCRAPPY_ORACLE_SEEDS` env var, or `config/company_slugs.yaml` entry, or `--search` term
- Seed format: `{subdomain}-{region}` (e.g. `oracle-CX_45001`) or full URL
- Parser order:
  1. JSON API response (`oracleJobsResponse`) via OData-style REST
  2. Paginated — offset up to `oracleMaxPages=50` pages, `oracleRecordsPerPage=100`
- Extracted fields: title, company_name (EmployerName), location (PrimaryLocation), date_posted, job_url (ExternalUrl/ExternalUrlSeo)

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- Seed inputs: `--oracle-seeds` / `SCRAPPY_ORACLE_SEEDS`

## Constraints and breakpoints
- Seed parsing depends on the `{subdomain}-{region}` convention — last dash splits subdomain from region
- If the seed is a full URL, it parses the host directly
- Site number defaults to `CX_45001` — may need updating per tenant
- Sort defaults to `POSTING_DATES_DESC`
- Facets list is hardcoded — may affect filtering behaviour if Oracle changes API
- Company name extraction from domain is naive (takes host head, capitalizes)

## Debug/update playbook
1. Verify the tenant domain resolves (e.g. `https://oracle.fa.us6.oraclecloud.com`)
2. Check the REST endpoint returns valid JSON with `items[0].requisitionList`
3. If no results, confirm the site number in `oracleDefaultSiteNumber`
4. Re-run `go test ./internal/scraper/ats-oracle` and full suite
