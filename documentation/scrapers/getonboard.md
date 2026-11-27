# getonboard

Source id: getonboard
Family: feeds_api
Health: working
Endpoint: GET https://www.getonbrd.com/api/v0/search/jobs?per_page={query:per_page}&page=1&query={query:term}
Parser: json
Fetcher: plain
Credentials: none
Rate: 20 requests per minute
Concurrency: 2
Robots: allow
Attribution: GetOnBoard

## Behavior

- Single GET with query placeholders for per-page count and search term; the page is fixed at 1.
- The JSON:API payload has a top-level data array of resource objects.
- Field aliases read attributes.title, attributes.company, attributes.description, attributes.location_cities, and links.public_url.
- Alias traversal is dot-separated, so nested JSON:API attributes are read without a custom parser.
- The frozen fixture exercises both an array and an object form of the mixed-type location field.

## Known quirks

- Seniority, tags, and salary fields from the prior scraper are not part of the normalized record contract.
- A location field that is an object of references yields no normalized location.

## Break and fix log
- 2026-11-27: Live validation returned HTTP 200 but no records with title, company, and URL. The public API reference still documents `GET /api/v0/search/jobs` with a JSON:API `data[]` array, `links.public_url`, and `attributes.*`, matching the manifest. The fixture and manifest were left unchanged.

- 2026-11-26: Manifest, generic fixtures, and offline replay recorded during the Tier 1 port.

## Hardened live probe 2026-11-27

- Live probe returned 20 records after the challenge-classification and pagination fixes.
