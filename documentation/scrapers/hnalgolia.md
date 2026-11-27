# hnalgolia

Source id: hnalgolia
Family: feeds_api
Health: working
Endpoint: GET https://hn.algolia.com/api/v1/search_by_date?tags=job&hitsPerPage=50
Parser: json
Fetcher: plain
Pagination: none; the page size is fixed at 50 recent job stories
Rate: 20 requests per minute
Concurrency: 1
Robots: allow public search API
Attribution: Hacker News Algolia
Credentials: none
Slugs: none

## Behavior

- The hits envelope contains job stories with a title, author, and optional external URL.
- Canonical URLs come from the Hacker News item id when an external URL is absent.
- The job_text field supplies the description; no structured location or company field is published.
- The author handle is used as the company label because the API has no company field.
- The fixture is synthetic and uses example.test URLs.

## Quirks

- External URLs are ignored in favor of the Hacker News discussion URL so records are stable over time.
- Some hits contain HTML entities in job_text, which are preserved as text.
- The existing hackernews source uses a two-stage fan-out and reports zero records in single-request live validation; this source covers the same jobs in one request.

## Break and fix log
- 2026-11-27: The latest live sweep recorded hnalgolia in the working section
  with 50 records from the fixed `hitsPerPage=50` page. No new substitution or
  credential was needed.
- 2026-11-27: Added the Algolia jobs endpoint after live validation showed the Firebase jobstories endpoint needs a second-stage fan-out that the single-request live validator does not perform.

