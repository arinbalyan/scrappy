# nofluffjobs

Source id: nofluffjobs
Family: feeds_api
Health: working
Endpoint: GET https://nofluffjobs.com/api/posting
Parser: json with field aliases
Fetcher: plain
Credentials: none
Rate: 20 requests per minute
Concurrency: 2
Robots: allow
Attribution: NoFluffJobs

## Quirks

- Historical source read a bare array of postings from the public posting API.
- Company comes from the posting name, and location comes from the first place entry.
- The URL alias falls back to the posting id, which the consumer resolves against the job path.
- Salary, seniority, posted time, regions, and client-side search filtering are outside this adapter.
- The bare posting API returned more than 8 MiB on the public tenant. The public search route is POST https://nofluffjobs.com/api/search/posting with limit, offset, and region parameters plus a JSON criteria body. The manifest only issues GET, so it cannot adopt that route without a request_options change and a fixture body.

## Break and fix log
- 2026-11-27: Research identified POST https://nofluffjobs.com/api/search/posting as the bounded public search route. The manifest and fixtures stay on the GET posting route because the current request template is GET-only; the oversized response remains the known break until a POST request with criteria is introduced and verified.
- 2026-11-27: Live validation returned a response above the 8 MiB fetch limit. The manifest needs a narrower query or a raised bounded limit.

- 2026-11-26: Ported from historical behavior into the manifest adapter with synthetic fixtures.
- 2026-11-26: Corrected endpoint from the RSS path to the posting API and aligned fixture fields.



## Oversized payload 2026-11-27

- The public `/api/posting` response exceeded 63 MB in a 30-second probe and was still streaming.
- Buffering that response is not safe for the default memory ceiling. The source is marked blocked until a filtered or paginated endpoint is documented.
