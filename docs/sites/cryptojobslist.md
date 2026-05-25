# CryptoJobsList

## Current integration
- Endpoint: `https://api.cryptojobslist.com/jobs.rss`
- Response shape: RSS feed with `<item>` blocks.
- Extracted fields: title, link, description, pubDate, category.
- **Extended fields**: `CompanyLogoURL` from `media:content` URL attribute, `Location.City` from `media:location`.

## Supported knobs
- `search_term` — client-side filter on title + description + category
- `results_wanted`
- global resilience knobs: `retries`, `max_rps`, `site_rps`, proxy config

## Constraints and breakpoints
- RSS feed contains all current listings — no server-side search or pagination.
- Search term filtering is client-side (case-insensitive substring match).
- `dc:creator` extracted as `CompanyName`.
- `media:content` URL attribute extracted as `CompanyLogoURL`.
- `media:location` extracted as `Location.City`.
- ID extracted from GUID (preferred) or link URL path segment.
- HTML tags stripped from description using regex.

## Debug/update playbook
1. Confirm the RSS feed URL is still `api.cryptojobslist.com/jobs.rss`.
2. Validate `<item>` structure includes `dc:creator`, `media:content`, `media:location`.
3. Check `media:content` URL attribute extraction (uses `url="..."` attribute pattern).
4. Verify `media:location` tag content is still the city name.
5. Re-run `go test ./tests/scraper/cryptojobslist` and then full suite.
