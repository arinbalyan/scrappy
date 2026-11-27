# Scraper behavior records

Each registered source gets one file in this folder, named
`<source_id>.md`. The file is the record for that scraper and is updated in
the same change that fixes the scraper.

## Record format

The canonical labels below are recommended for new and updated records. Existing
records use prose and heading variants and are accepted as they stand: for
example, a health state may be a sentence rather than a `Health:` line, a
family may be described in prose, `## Quirks` and other quirks headings stand in
for `## Known quirks`, and behavior details may appear under a different
heading or in the break and fix log. A missing canonical label is not an error
when the same fact is stated elsewhere in the file.

- Source id and display name
- Adapter family: feeds_api, ats, html_static, html_rendered, keyed
- Health state: working, keyed, blocked, stale, requires_slug, browser_only. Unregistered is a live-report and audit category for historical IDs without a manifest, not a health state.
- Endpoint shape and request template
- Parser kind and fetcher kind
- Emulation profile or browser requirement
- Credential and slug needs, by name only
- Rate limit and concurrency cap
- Robots policy and attribution
- Known quirks, under `## Known quirks` or a variant such as `## Quirks`
- Break and fix log with dates

## Example

```text
# sample

Source id: sample
Family: feeds_api
Health: working
Endpoint: GET https://example.test/jobs.json
Parser: json
Fetcher: plain
Credentials: none
Rate: 30 requests per minute
Concurrency: 2
Robots: allow indexing of public job listings
Attribution: Example

## Quirks

- Endpoint returns UTC timestamps without a timezone marker.

## Break and fix log

- 2026-11-26: Parser accepted a new field shape; fixed in src/parsing/json.rs.
```

Manifest validation and parity rules live in
openspec/changes/oculus-foundation/specs/source-registry/spec.md.
