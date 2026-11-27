# jobsch

Source id: jobsch
Family: feeds_api
Health: working
Endpoint: GET https://www.jobs.ch/api/v1/public/search?query=engineer
Parser: JSON envelope with field aliases
Fetcher: plain
Credentials: none
Rate: 20 requests per minute
Concurrency: 2
Robots: allow
Attribution: jobs.ch

## Quirks

- Historical source built detail URLs from the first job link with an id fallback; this port reads the English detail link.
- The public search endpoint rejects requests without a query term. A fixed engineer query supplies that required parameter; other search terms and pagination are outside this adapter.

## Break and fix log
- 2026-11-27: Live validation returned HTTP 200 with 20 records after changing the frozen request from rows=25 to query=engineer. The public endpoint requires query; the rows parameter alone returns 422. The fixture body stays synthetic, so golden replay stays stable.
- 2026-11-27: Live validation returned HTTP 422 with no `documents` envelope. Public search is rendered server-side and honors filters while the JSON endpoint reportedly ignores them; a corrected JSON parameter set is not public. The manifest and fixture are unchanged.

- 2026-11-26: Ported from historical behavior into the manifest adapter with synthetic fixtures.
- 2026-11-26: Live probe returned 200 with a valid documents envelope.
