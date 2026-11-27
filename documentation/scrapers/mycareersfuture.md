# mycareersfuture

Source id: mycareersfuture
Family: feeds_api
Health: working
Endpoint: POST https://api.mycareersfuture.gov.sg/v2/search with JSON body {"limit":{query:limit},"keyword":"{query:keyword}"}
Parser: json
Fetcher: plain
Pagination: none, one request with a limit parameter
Rate: 20 requests per minute
Concurrency: 1
Robots: allow public job search
Attribution: MyCareersFuture
Credentials: none
Slugs: none

## Behavior

- The adapter posts a JSON search body to `/v2/search` and reads the `results` array from the response alongside `total`.
- Company is a nested object, reached by a dotted alias name; location is flattened to a string in the frozen payload because the adapter cannot index into list fields.
- The normalized URL comes from metadata.jobDetailsUrl, with jobUrl as the fallback.
- Description comes from the job description field; salary, posting date, and remote flags are not represented in the six-field record.
- Items with no jobUrl or no title are discarded before normalization.

## Known quirks

- The historical request used a 200-500 ms jitter delay; the declarative manifest uses a fixed rate cap.
- Missing location produces a null location field.

## Break and fix log
- 2026-11-27: Live validation returned HTTP 200 but the JSON payload was not an array. Public search now uses zero-auth `POST /v2/search`, which returns `total` and `results`. Switched the manifest and fixture to that request with a `keyword`/`limit` JSON body and added the `results` envelope; golden records are unchanged.

- 2026-11-27: Replay showed current postings put the company under `postedCompany.name` and the public URL under `metadata.jobDetailsUrl`. Added those aliases ahead of `company.name` and `jobUrl`.

- 2026-11-26: Manifest, synthetic fixtures, behavior doc, and offline replay recorded during the Tier 2 B port.
