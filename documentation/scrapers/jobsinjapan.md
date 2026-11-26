# jobsinjapan

Source id: jobsinjapan
Family: feeds_api
Health: working
Endpoint: GET https://jobsinjapan.com/feed/
Parser: RSS with title pattern splitting
Fetcher: plain
Credentials: none
Rate: 20 requests per minute
Concurrency: 2
Robots: allow
Attribution: Jobs in Japan

## Quirks

- Historical source parsed RSS items with custom namespaced fields; this port splits the title into title and company.
- The live feed is a blog feed rather than a job feed, and its items omit the company and job type fields that the historical filter required.
- Client-side search filtering, date parsing, salary, and category are outside this adapter.

## Break and fix log

- 2026-11-26: Ported from historical behavior into the manifest adapter with synthetic fixtures.
- 2026-11-26: Live probe returned 200 with twenty blog items and no job company fields; this source needs a real job feed endpoint before it produces jobs.
