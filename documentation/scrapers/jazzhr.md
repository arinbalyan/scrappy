# jazzhr

Source id: jazzhr
Family: ats
Health: keyed
Endpoint: GET https://api.resumatorapi.com/v1/jobs/status/open?apikey={key:JAZZHR_API_KEY}
Parser: JSON root array
Fetcher: plain
Credentials: JAZZHR_API_KEY

## Behavior

- The API key is substituted into the query apikey parameter; replay uses a synthetic value.
- The payload is a top-level JSON array of open jobs read through field aliases.
- Titles come from title, and location prefers city then state with a flat location fallback.
- The historical scraper derived company and the applytojob.com URL from the company seed, while stripping HTML from descriptions. The derived company and URL are outside the adapter, so fixtures carry explicit company, URL, and plain description fields.

## Quirks

- Seed resolution, per-seed deduplication, ZIP codes, departments, employment types, open dates, and board codes are outside the normalized record contract and the single-request adapter.
- Without a configured credential the manifest cannot build a request.

## Break and fix log

- 2026-11-26: Ported from historical behavior with a keyed manifest and synthetic credential fixture.
- 2026-11-26: Historical status docs list the JAZZHR_API_KEY requirement and 21 populated seeds; health stays keyed because the credential is mandatory.
