# adp

Source id: adp
Family: ats
Health: blocked
Endpoint: GET https://workforcenow.adp.com/mascsr/default/careercenter/public/events/staffing/v1/job-requisitions?cid={slug}
Parser: JSON with jobRequisitions envelope
Fetcher: plain
Credentials: none
Slugs: example-co (seed placeholder)

## Behavior

- The tenant slug fills the cid query parameter on the public requisition feed.
- The jobRequisitions envelope holds requisitions. Titles come from jobTitle and URLs from externalUrl.
- Company prefers companyName and falls back to the slug. Description prefers jobDescription, then shortDescription.
- Location aliases cover the singular location object first, then the first entry of locations.
- Fixtures carry explicit URLs because the historical detail URL was composed from the tenant and requisition id.
- Health is blocked because the historical status report lists ADP as timing out with populated slugs.

## Quirks

- Requisition ids, requisition numbers, departments, worker types, employment types, posting dates, compensation objects, remote inference from formatted addresses, HTML stripping, deduplication, and concurrent seed processing are outside the normalized record contract.
- The adapter makes one request per seed; $top-style result limits, offset paging, and the 25-job default are not represented.
- Boards without a configured seed cannot build a cid and are skipped.

## Break and fix log
- 2026-11-27: Live validation returned HTTP 404 for the seeded tenant path. Keep the tenant seed requirement and re-check the endpoint before enabling this source.

- 2026-11-26: Ported the tenant requisition feed with a slug manifest and synthetic fixture.
- 2026-11-26: Historical timeout report mapped to blocked; no live probe was added.
