# ats-recruiterflow

Source id: ats-recruiterflow
Family: ats
Health: requires_slug
Endpoint: GET https://api.recruiterflow.com/api/external/job/list
Parser: JSON with data envelope
Fetcher: plain
Rate: 30 requests per minute
Concurrency: 1

## Behavior

- The seed slug is sent as the RF-Api-Key header; the endpoint URL does not contain the slug.
- The data envelope holds postings. Fixtures carry explicit URLs and company values because the historical code synthesized them from the slug and numeric ID.
- Location and description are read from flat fields.

## Quirks

- The seed doubles as a credential, which differs from the named credential model used by keyed sources.
- Remote inference and creation dates stay outside the normalized record contract.

## Break and fix log
- 2026-11-27: Live validation returned HTTP 401 with the example slug. The source remains requires_slug and needs an active board slug for a meaningful probe.

- 2026-11-26: Ported the external job list with a slug header template and synthetic fixture.
- 2026-11-26: Live probe returned HTTP 400 with an API key requirement; health left at requires_slug because the manifest declares no credential option yet.


## Viability classification 2026-11-27

- Class: requires_slug. The seed is the API key itself, so the source is credential-only.
