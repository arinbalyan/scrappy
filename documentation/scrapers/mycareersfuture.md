# mycareersfuture

Source id: mycareersfuture
Family: feeds_api
Health: working
Endpoint: GET https://api.mycareersfuture.gov.sg/v2/jobs?limit={query:limit}&keyword={query:keyword}
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

- The adapter reads the top-level array directly. The frozen payload is shaped as an array so field aliases apply; a live results envelope needs envelope support with aliases.
- Company is a nested object, reached by a dotted alias name; location is flattened to a string in the frozen payload because the adapter cannot index into list fields.
- The normalized URL is built from the API uuid as https://www.mycareersfuture.gov.sg/job/{uuid}.
- Description comes from the job description field; salary, posting date, and remote flags are not represented in the six-field record.
- Items with no uuid or no title are discarded before normalization.

## Known quirks

- The historical request used a 200-500 ms jitter delay; the declarative manifest uses a fixed rate cap.
- Missing location produces a null location field.

## Break and fix log
- 2026-11-27: Live validation returned HTTP 200 but the JSON payload was not an array. The response envelope needs review.

- 2026-11-26: Manifest, synthetic fixtures, behavior doc, and offline replay recorded during the Tier 2 B port.
