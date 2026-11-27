# internshala

Source id: internshala
Family: html_static
Health: working
Endpoint: GET https://internshala.com/jobs/{slug}-jobs/page-{query:page}
Parser: html with selectors
Fetcher: plain
Pagination: page one through eight
Rate: 10 requests per minute
Concurrency: 2
Robots: allow public job and internship cards
Attribution: Internshala
Credentials: none
Slugs: engineering

## Behavior

- Search terms are hyphenated into a slug path with a `-jobs` suffix; the frozen fixture uses engineering.
- Cards are matched by the individual_internship container. The old flow also recognized individual_job and listing variants, which this manifest does not combine in one item selector.
- Title and URL come from the job-title-href anchor; company comes from the company-name paragraph.
- The internships path shares the card shape; the fixture covers the jobs variant.

## Quirks

- Relative URLs stay relative in normalized records.
- The old flow substituted Unknown Employer when a card had no company; the HTML parser yields no company and the card is skipped.
- Location and description are dropped by the HTML parser, although the old flow read them from card text.

## Break and fix log
- 2026-11-27: Live validation returned HTTP 200 but no records matched because the company selector expected a span while the page uses a paragraph.
- 2026-11-27: Fixed the company selector to p.company-name and kept the current jobs URL template; refreshed the synthetic fixture. Verified via tier 2 replay. A live request also needs an explicit slug substitution.

- 2026-11-26: Manifest, static HTML fixture, behavior doc, and offline replay recorded during the Tier 2 port.


## Hardened live probe 2026-11-27

- Live probe returned 52 records after the challenge-classification and pagination fixes.
