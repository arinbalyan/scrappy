# ats-oracle

Source id: ats-oracle
Family: ats
Health: requires_slug
Endpoint: GET https://{slug}.fa.oraclecloud.com/hcmRestApi/resources/latest/recruitingCEJobRequisitions
Parser: JSON with items.0.requisitionList envelope
Fetcher: plain
Rate: 30 requests per minute
Concurrency: 1

## Behavior

- The seed supplies the tenant subdomain before the .fa.oraclecloud.com region host. The default site number, facets list, and finder parameters are fixed in the manifest.
- The first items entry holds requisitionList. Fixtures carry explicit URLs and company values because the historical code composed them from ExternalUrlSeo and the tenant. ExternalUrlSeo and Id are not URLs, so they are excluded from URL aliases.
- Company prefers EmployerName and falls back to the slug. Description is absent from the historical feed.

## Quirks

- A historical slug such as tenant-us was split into tenant.fa.us.oraclecloud.com; the manifest expects the tenant subdomain directly.
- Offset pagination, remote inference, and posting dates stay outside the adapter.

## Break and fix log
- 2026-11-27: Endpoint research found no verified replacement for the HCM recruiting requisitions route. The failure came from the example-co placeholder subdomain, so the slug placeholder and requires_slug stay unchanged. Re-probe with an active tenant subdomain before changing the URL.
- 2026-11-27: Live validation could not reach the example tenant host; the source stays requires_slug.

- 2026-11-26: Ported the recruiting requisitions feed with a slug manifest and synthetic fixture.
- 2026-11-26: Live probe of a historical tenant subdomain timed out; health left at requires_slug because no active public tenant was confirmed.


## Live seed verification

- 2026-11-27: verified live seed `ejwl.fa.us2.oraclecloud.com` through the manifest adapter, returning 100 records.
- The manifest now ships this verified seed and is declared working. Re-probe when the tenant board moves.
