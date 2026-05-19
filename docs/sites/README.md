# Site Notes

One page per site. Each page covers: how it works, pagination, rate limits, known issues, and how to contribute if implementation is still planned.

## Indeed

- GraphQL API at `apis.indeed.com/graphql`.
- Page size: 100 jobs. Cursor-based (`nextCursor`) — effectively unbounded.
- Country subdomain: `{country}.indeed.com`. API country code passed in header `indeed-co`.
- Date filter: `dateOnIndeed` in the GraphQL query.
- Known issue: Indeed API started returning anonymous results without employer names in late 2024. If `company_name` is empty, set `--fallback-company-url` to strip employer branding from the job URL.
- Rate: 3 req/s max (`--site-rps indeed:3`).

## LinkedIn

- Guest API: `linkedin.com/jobs-guest/jobs/api/seeMoreJobPostings/search`.
- 10 jobs per page, offset-based. Hard cap `start < 1000`.
- Headers must include the full `LinkedIn` user-agent chain (see `internal/scraper/linkedin/constant.go`).
- `linkedin_fetch_description=true` doubles the request count — takes O(n).
- Rate-limit: returns HTTP 429 after ~page 10. Silent stop, not a hard failure.
- Workaround: `--linkedin-strategy rotate` permutes `f_AL`, `f_JT`, `f_TPR`, location radius, and distance across multiple scrape passes.

## Glassdoor

- Country-specific subdomains: `{subdomain}.glassdoor.{tld}`.
- US: `www.glassdoor.com`. UK: `www.glassdoor.co.uk`.
- Uses `savedSearchId` in the query for recent postings filter.
- Dates are rounded up to the next day (hence `--hours-old` is approximate).

## ZipRecruiter

- Search API at `www.ziprecruiter.com/candidate/search`.
- US/Canada only (`country_indeed` not applicable).
- Aggressive anti-bot; proxy required for any sustained run.
- Pagination: cursor-based through an API endpoint.

## Google Jobs

- Embedded in Google SERP HTML — no separate API endpoint.
- Use `--google-search-term "..."` (not plain `--search`). Copy the full Google Jobs search-box string from your browser's URL bar when a search is active.
- Rate-limited aggressively; treat as best-effort.
- `--max-rps google:2` recommended maximum.

## Wellfound (planned)

- Wellfound public job listings (`wellfound.com/j-loc/{slug}`).
- Startup-focused, remote-friendly.
- Effort: easy — public HTML, no auth.

## RemoteOK (planned)

- Single-page application, jobs embedded as JSON in a `<script id="job-map">` tag.
- 50 jobs per JSON blob; paginate via `?page=N`.
- Effort: easy.

## Remotive (planned)

- Remotive provides a JSON API at `remotive.com/api/remote-jobs`.
- Filter by category, salary range, date.
- Effort: easy.

## BuiltIn (planned)

- School-specific boards: `builtin.com/school/{san-francisco|boston|new-york|austin}`.
- Public HTML, tech-focused.
- Effort: easy-medium.

## Otta (planned)

- `otta.com` — AI talent matching. May require a cookie from a logged-in browser session.
- Effort: medium.

## Lever (planned)

- Each company has its own board at `{company}.jobs.lever.co`.
- Enumerate from a known company list or extract from LinkedIn URLs.
- Effort: medium.

## Greenhouse (planned)

- Each company has a board at `boards.greenhouse.io/{company}`.
- Same pattern as Lever.
- Effort: medium.
