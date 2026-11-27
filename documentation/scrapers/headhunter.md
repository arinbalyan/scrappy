# headhunter

Source id: headhunter
Family: feeds_api
Health: blocked
Endpoint: GET https://api.hh.ru/vacancies?text={search_term}&per_page=25&page=0
Parser: JSON with items envelope
Fetcher: plain
Rate: 30 requests per minute

## Behavior

- The public vacancy API returns an items envelope with nested employer, area, and snippet objects.
- Field aliases read nested company, location, and description paths with flat fallbacks.
- Fixtures carry explicit URLs because the historical derived URL is outside the adapter.

## Quirks

- Salary, remote flags, and publication dates are outside the normalized record contract.
- The historical scraper jittered requests; pacing is handled by engine rate limits in the rewrite.

## Break and fix log
- 2026-11-27: The latest live sweep placed headhunter in the blocked section
  with HTTP 403. The declared health stays blocked; re-probe from an
  unrestricted network before treating the denial as a source-side change.
- 2026-11-27: Live validation returned HTTP 403, consistent with the blocked health. The run environment may restrict outbound hosts.

- 2026-11-26: Ported from historical behavior with a manifest and synthetic fixture.
- 2026-11-26: Live probe returned HTTP 403 with a forbidden error; health changed to blocked.


## Identity policy probe 2026-11-27

- Requests with a documented non-personal HH-User-Agent still returned HTTP 403.
- The source remains blocked until a public access policy or allowlisted client is confirmed.
