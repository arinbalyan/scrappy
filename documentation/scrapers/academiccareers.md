# academiccareers

Source id: academiccareers
Family: feeds_api
Health: working
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
- The 2026-11-26 probe saw an nginx 403. Later probes returned 50 RSS records, so the earlier denial was transient or environment-specific.

## Break and fix log

- 2026-11-26: Ported from historical behavior into the manifest adapter with synthetic fixtures.
- 2026-11-26: Live probe returned HTTP 403; health changed to blocked.
- 2026-11-27: Live validation returned 50 records from the RSS endpoint. Updated health and replay contract to working; the earlier 403 is retained as a known transient failure.
- 2026-11-27: Endpoint research found no replacement path, and the recorded /rss URL still returned records.
