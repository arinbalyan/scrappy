# huggingfacejobs

Source id: huggingfacejobs
Family: html_static
Health: working
Endpoint: GET https://huggingface.co/jobs
Parser: html
Fetcher: plain
Credentials: none
Rate: 20 requests per minute
Concurrency: 2
Robots: allow
Attribution: Hugging Face Jobs

## Behavior

- Single GET returns a static HTML page; no browser or emulation profile is used.
- Selectors match listing containers, so navigation anchors are excluded.
- Title comes from a title element inside the matched container; URL comes from a job-link anchor and company from a sibling span.
- The parser emits one record per matched item; the duplicate-link behavior from the prior scraper is not reproduced by the adapter and is visible in the fixture.

## Known quirks

- Relative URLs stay relative in normalized records; callers resolve them against the request URL.
- An item without a company span still normalizes with an empty company.

## Break and fix log

- 2026-11-26: Manifest, generic fixtures, and offline replay recorded during the Tier 1 port.
