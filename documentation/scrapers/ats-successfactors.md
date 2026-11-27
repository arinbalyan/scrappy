# ats-successfactors

Source id: ats-successfactors
Family: ats
Health: stale
Endpoint: GET https://example-co.successfactors.com/odata/v2/JobRequisitionPosting?$select=...&$top=20&$skip=0&$format=json
Parser: JSON with d.results envelope
Fetcher: plain
Credentials: none
Slugs: example-co (seed placeholder)

## Behavior

- The OData query selects the historical posting fields with a 20-item top and zero skip. Dollar signs are literal in the manifest TOML string.
- The d.results envelope holds postings. Titles prefer jobTitle, then formattedJobTitle. URLs come from externalJobUrl.
- Company prefers companyName and falls back to the seed name. Description comes from jobDescription.
- Location aliases read the city, state, and country of locationObj.
- The instance is fixed to example-co in the manifest and fixture because the adapter has no {instance} placeholder. The slug seed is declared for inventory only.
- Fixtures carry explicit URLs because the historical career URL was composed from the instance, company id, and requisition id.
- Health is stale because the historical status report marks the seeded slugs as stale.

## Quirks

- Requisition ids, departments, posting start dates, employment types, result counts, remote inference from the city, and seed splitting into instance plus company id are outside the normalized record contract.
- $skip paging, the instance-plus-company-id seed format, and the 100-job default are outside the adapter; the manifest issues one fixed-tenant request.
- A different tenant requires editing the manifest URL; no runtime substitution changes the instance.

## Break and fix log

- 2026-11-26: Ported the first OData page with a fixed tenant and synthetic fixture.
- 2026-11-26: Historical stale-slug report mapped to stale; no live probe was added.
