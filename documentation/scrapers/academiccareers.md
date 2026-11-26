# academiccareers

Source id: academiccareers
Family: feeds_api
Health: blocked
Endpoint: GET https://www.academiccareers.com/rss
Parser: RSS with title pattern splitting
Fetcher: plain
Credentials: none
Rate: 20 requests per minute
Concurrency: 2
Robots: allow
Attribution: Academic Careers

## Quirks

- Historical source parsed RSS items with custom namespaced fields; this port splits the title into title and company.
- Client-side search filtering, date parsing, salary, and category are outside this adapter.
- The plain fetcher received HTTP 403 from nginx during live validation; browser or proxy access is needed before this source can serve records.

## Break and fix log

- 2026-11-26: Ported from historical behavior into the manifest adapter with synthetic fixtures.
- 2026-11-26: Live probe returned HTTP 403; health changed to blocked.
