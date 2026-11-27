# huggingfacejobs

Source id: huggingfacejobs
Family: html_static
Health: working
Endpoint: GET https://apply.workable.com/api/v1/widget/accounts/huggingface
Parser: json
Fetcher: plain
Credentials: none
Rate: 20 requests per minute
Concurrency: 2
Robots: allow
Attribution: Hugging Face Jobs

## Behavior

- Single GET returns the public Workable widget JSON for the Hugging Face account.
- The jobs envelope holds the records. Title and URL come from title and url or shortlink; company comes from a per-item field.
- Location reads city, then the first entry of locations; descriptions are absent without the details query.

## Known quirks

- Location fields arrive separately; this adapter keeps the chosen city only.
- Accounts with no published jobs return an empty list.

## Break and fix log
- 2026-11-27: Live validation returned HTTP 200 but no records matched the configured selectors. The static HTML selectors need updating.
- 2026-11-27: Fixed by switching to the public Workable widget endpoint; replaced HTML selectors with JSON field aliases and refreshed the synthetic fixture and golden records. Verified via tier 1 replay.

- 2026-11-26: Manifest, generic fixtures, and offline replay recorded during the Tier 1 port.
