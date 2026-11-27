# wellfound

Source id: wellfound
Family: html_static
Health: working
Endpoint: GET https://wellfound.com/jobs
Parser: json with embedded `__NEXT_DATA__` Apollo cache
Fetcher: plain
Rate: 15 requests per minute
Concurrency: 1
Robots: allow public job pages; skip challenged pages
Attribution: Wellfound

## Behavior

- Current pages store jobs as normalized Apollo objects under `props.pageProps.apolloState.data`, selected by `__typename = JobListing`.
- Company fields are `__ref` links into `Startup` objects and are resolved before field extraction.
- URLs are derived from each job slug with the `https://wellfound.com/jobs/` prefix.
- The first entry in `locationNames` supplies the location field; the cache does not carry descriptions.
- Search-term URL construction and detail-page enrichment stay outside the adapter.
- Search-term URL construction and detail-page URL synthesis stay outside the adapter.

## Quirks

- A challenge page has no embedded blob and fails replay instead of falling back.
- Compensation, remote flags, posting dates, descriptions, and skill lists remain outside the normalized record contract.

## Break and fix log
- 2026-11-27: Live validation returned HTTP 200 but the old `props.pageProps.listings` envelope was missing. Added typed Apollo object selection, `__ref` resolution, and slug-derived URLs; the targeted probe returned 43 records.

- 2026-11-27: Live validation returned HTTP 200 but the embedded JSON envelope was missing. Current pages expose normalized Apollo cache entries under __NEXT_DATA__ -> props.pageProps.apolloState.data, while the manifest and fixture use the older direct props.pageProps.listings path. The declarative embedded parser cannot follow __ref links or select an object as an array, so switching the envelope would break replay.

- 2026-11-26: Ported the embedded Next.js JSON path with a synthetic HTML fixture and offline replay.

## Live validation

- 2026-11-27: Targeted live validation with `SCRAPPY_LIVE_ONLY=wellfound` returned 43 records from the Apollo cache path.
- 2026-11-26: The endpoint was not probed successfully from this network; health came from historical evidence and offline replay.

## Hardened live probe 2026-11-27

- Live probe returned 45 records after the challenge-classification and pagination fixes.
