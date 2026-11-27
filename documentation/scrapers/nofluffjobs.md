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

## Break and fix log
- 2026-11-27: Live validation returned a response above the 8 MiB fetch limit. The manifest needs a narrower query or a raised bounded limit.

- 2026-11-26: Ported from historical behavior into the manifest adapter with synthetic fixtures.
- 2026-11-26: Corrected endpoint from the RSS path to the posting API and aligned fixture fields.

