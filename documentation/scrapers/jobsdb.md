# jobsdb

Source id: jobsdb
Family: feeds_api
Health: working
Endpoint: GET https://www.seek.com.au/api/jobsearch/v5/search?keywords={query:keywords}&page={query:page}&pagesize={query:pagesize}&siteKey=SG-Main&where={query:where}
Parser: json
Fetcher: plain
Pagination: numbered pages, up to 10, stopping on an empty page or fewer than 30 results
Rate: 20 requests per minute
Concurrency: 1
Robots: allow public job search
Attribution: JobsDB
Credentials: none
Slugs: none

## Behavior

- The SEEK v5 search API is shared with other regional boards; the site key selects the SG-Main index.
- The adapter reads the `data` array from the SEEK v5 response. The frozen payload carries `data` and `totalCount` alongside the job rows.
- Location is flattened into a scalar string in the frozen payload because the adapter cannot index into list fields.
- Company comes from companyName, with advertiser.description as the fallback.
- The live response omits jobUrl. The canonical URL is derived from the `id` field with the `https://www.jobsdb.com/job/` prefix. The opaque `solMetadata.token` is never used as a link.
- Location comes from the flattened location string; description is the teaser.
- Salary parsing, date parsing, remote detection, classification fields, and bullet-point merging are not represented in the six-field record.

## Known quirks

- The historical request used a 200-500 ms jitter between pages; the declarative manifest uses a fixed rate cap.
- Items with no id, no title, or no company are discarded before normalization.
- Missing location produces a null location field.

## Break and fix log
- 2026-11-27: Live validation returned HTTP 200 but the JSON payload was not an array. SEEK v5 still serves `/api/jobsearch/v5/search` with `siteKey=SG-Main` and returns rows under `data` with a `totalCount`. Added the `data` envelope to the manifest and wrapped the fixture; golden records are unchanged.

- 2026-11-27: Replay showed SEEK rows now carry location under `locations.0.label`. Added that alias ahead of `location`. The opaque `solMetadata.token` was rejected as a URL because it is not an http link.

- 2026-11-27: Live validation returned HTTP 200 but zero records because the API stopped emitting `jobUrl`. Added `url_from` and `url_prefix` parser options so the canonical URL is derived from the numeric `id`. Updated fixtures and golden records to match the live payload shape.

- 2026-11-26: Manifest, synthetic fixtures, behavior doc, and offline replay recorded during the Tier 2 B port.
