# 4dayweek

Source id: 4dayweek
Family: feeds_api
Health: working
Endpoint: GET https://4dayweek.io/api/jobs
Parser: json
Fetcher: plain
Credentials: none
Rate: 20 requests per minute
Concurrency: 2
Robots: allow
Attribution: 4 Day Week

## Behavior

- Base endpoint returns a jobs envelope with a has-more flag.
- Field aliases read the nested company name, the first location city, and the explicit job URL.
- The manifest declares pagination none; has-more paging and the five-page cap are not part of the adapter contract.
- The frozen fixture shows the first envelope shape, with has_more false.

## Known quirks

- Salary, schedule type, and posted timestamp from the prior scraper are not part of the normalized record contract.
- Location aliases use the first array element only.

## Break and fix log
- 2026-11-27: Live validation returned HTTP 200 but no records with title, company, and URL. The JSON envelope or aliases need review against the live payload.

- 2026-11-26: Manifest, generic fixtures, and offline replay recorded during the Tier 1 port.
