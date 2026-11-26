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

- 2026-11-26: Ported from historical behavior with a manifest and synthetic fixture.
- 2026-11-26: Live probe returned HTTP 403 with a forbidden error; health changed to blocked.

