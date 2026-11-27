# ats-phenom

Source id: ats-phenom
Family: ats
Health: stale
Endpoint: GET https://jobs.{slug}.com/api/jobs?offset=0&limit=25
Parser: JSON with jobs envelope
Fetcher: plain
Credentials: none
Slugs: example-co (seed placeholder)

## Behavior

- The tenant slug completes the jobs host, with offset 0 and a 25-item limit fixed in the manifest.
- The jobs envelope holds postings. Titles come from title and URLs from url, with applyUrl as the second choice.
- Company prefers companyName and falls back to the slug. Description prefers description, then shortDescription.
- Location aliases cover the city, state, and country of the location object, then the flat locationText value.
- Fixtures carry explicit URLs because the historical job detail URL was synthesized from the tenant and numeric id.
- Health is stale because the historical status report marks the seeded slugs as stale.

## Quirks

- Job ids and requisition ids, departments, categories, employment and workplace types, remote inference, posted dates, and meaningful-job checks are outside the normalized record contract.
- Offset paging over multiple seed tenants and the 100-job default are outside the adapter; the manifest issues one request for the first page.
- Boards without a configured seed cannot build a tenant host and are skipped.

## Break and fix log
- 2026-11-27: The latest live sweep kept ats-phenom in the stale section. The
  example tenant host is a placeholder, so the host failure does not confirm a
  source-side change.
- 2026-11-27: Endpoint research found no verified replacement for the jobs tenant API route. The failure came from the placeholder tenant host, so the slug template and stale health stay unchanged. Confirm the route with an active tenant before changing the URL.
- 2026-11-27: Live validation could not reach the example tenant host. The source remains stale and needs an active tenant seed for a meaningful probe.

- 2026-11-26: Ported the first page of the tenant job API with a slug manifest and synthetic fixture.
- 2026-11-26: Historical stale-slug report mapped to stale; no live probe was added.
