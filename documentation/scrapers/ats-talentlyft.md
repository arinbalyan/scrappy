# ats-talentlyft

Source id: ats-talentlyft
Family: ats
Health: requires_slug
Endpoint: GET https://api.talentlyft.com/v2/public/{slug}/jobs?page=1&perPage={per_page}
Parser: json
Fetcher: plain
Pagination: one page per seed slug; page number and page size are query placeholders
Rate: 30 requests per minute
Concurrency: 1
Robots: allow public board endpoints; challenged boards are skipped
Attribution: TalentLyft
Credentials: none
Slugs: example-co (seed placeholder)

## Quirks

- Results live under a capitalized Results envelope with capitalized item fields.
- The old scraper used the seed slug as the company name. The fixture carries that value in a synthetic CompanyName field because the parser requires the company on each job object.
- Jobs with a blank title are skipped during normalization.
- Description HTML is retained as provided; the old scraper stripped tags after trimming, so a later port may normalize text differently.

## Break and fix log

- 2026-11-26: Ported the JSON board response with a slug manifest and synthetic fixture. The source stays requires_slug because a board slug must be supplied before a request can be built.

