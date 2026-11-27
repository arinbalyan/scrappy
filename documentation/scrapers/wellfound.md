# wellfound

Source id: wellfound
Family: html_static
Health: working
Endpoint: GET https://wellfound.com/jobs
Parser: json with embedded `__NEXT_DATA__`
Fetcher: plain
Rate: 15 requests per minute
Concurrency: 1
Robots: allow public job pages; skip challenged pages
Attribution: Wellfound

## Behavior

- Jobs live in the `__NEXT_DATA__` script JSON under `props.pageProps.listings` or `props.pageProps.jobs`.
- The snapshot fixture uses the direct listings path; the Apollo cache fallback from the historical scraper is not represented by a single envelope path.
- Company names use the nested company object; URL values are explicit in each listing.
- The first location string supplies the location field.
- Search-term URL construction and detail-page URL synthesis stay outside the adapter.

## Quirks

- Wellfound often serves a challenge page; a response without the embedded blob fails replay instead of falling back.
- Compensation, remote flags, posting dates, and skill lists remain outside the normalized record contract.

## Break and fix log
- 2026-11-27: Live validation returned HTTP 200 but the embedded JSON envelope was missing. Current pages expose normalized Apollo cache entries under __NEXT_DATA__ -> props.pageProps.apolloState.data, while the manifest and fixture use the older direct props.pageProps.listings path. The declarative embedded parser cannot follow __ref links or select an object as an array, so switching the envelope would break replay.

- 2026-11-26: Ported the embedded Next.js JSON path with a synthetic HTML fixture and offline replay.

## Live validation

- 2026-11-26: The endpoint was not probed successfully from this network; health stays `working` from historical evidence and offline replay.
