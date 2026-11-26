# cryptojobslist

Source id: cryptojobslist
Family: feeds_api
Health: working
Endpoint: GET https://api.cryptojobslist.com/jobs.rss
Parser: RSS with title pattern splitting
Fetcher: plain
Credentials: none
Rate: 20 requests per minute
Concurrency: 2
Robots: allow
Attribution: Crypto Jobs List

## Quirks

- Historical source parsed RSS items with custom namespaced fields; this port splits the title into title and company.
- Client-side search filtering, date parsing, salary, and category are outside this adapter.

## Break and fix log

- 2026-11-26: Ported from historical behavior into the manifest adapter with synthetic fixtures.
- 2026-11-26: Live probe returned 200 with one hundred valid RSS items.
