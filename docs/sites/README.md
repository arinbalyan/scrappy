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

## Google Jobs

- Embedded in Google SERP HTML — no separate API endpoint.
- Use `--google-search-term "..."` (not plain `--search`). Copy the full Google Jobs search-box string from your browser's URL bar when a search is active.
- Rate-limited aggressively; treat as best-effort.
- `--max-rps google:2` recommended maximum.

## Workable Jobs

- Site key: `workable_jobs`.
- Native endpoint pattern: `https://apply.workable.com/api/v1/widget/accounts/{seed}/jobs`.
- Seed inputs: `--workable-seeds` / `SCRAPPY_WORKABLE_SEEDS`.
- Role filtering: flexible title/description contains + synonym expansion.

## MyWorkdayJobs

- Site key: `myworkdayjobs`.
- Native endpoint pattern: Workday CXS JSON endpoint (`.../wday/cxs/.../jobs`).
- Seed inputs: `--workday-seeds` / `SCRAPPY_WORKDAY_SEEDS`.
- Role filtering: flexible title/description contains + synonym expansion.

## AuthenticJobs

- API endpoint: `https://authenticjobs.com/api/`.
- Requires `AUTHENTICJOBS_API_KEY` env var; skipped with WARN if missing.
- Page-based pagination (`page` param). Default page size: 25.
- Fields: id, title, company, description, perks, howto_apply, post_date, telecommuting, location.

## EcoJobs

- RSS feed: `https://www.ecojobs.com/rss.xml`.
- No server-side search or pagination; client-side filter on title + description.
- ID extracted from URL path segment.

## Golang Jobs

- RSS feed: `https://www.golangprojects.com/rss.xml`.
- No server-side search or pagination; client-side filter on title + description + category.
- Niche Go-specific board; may have feed availability issues.

## Landing.jobs

- JSON API: `https://landing.jobs/api/v1/jobs`.
- Offset-based pagination (`offset`, `limit`). Max 5 pages, page size 50.
- Compensation fields: `salary_low`, `salary_high` (EUR default).
- Client-side search filter on title, role_description, and tags.

## Himalayas

- JSON API: `https://himalayas.app/jobs/api`.
- Offset-based pagination (`offset`, `limit`). Max 10 pages, page size 20.
- Remote-only board. `pubDate` is a Unix timestamp.
- Compensation fields: `minSalary`, `maxSalary` (USD default).

## CryptoJobsList

- RSS feed: `https://api.cryptojobslist.com/jobs.rss`.
- Extended extraction: `CompanyLogoURL` (`media:content`), `Location.City` (`media:location`).
- `dc:creator` maps to company name.

## Real Work From Anywhere

- RSS feed: `https://www.realworkfromanywhere.com/rss.xml`.
- Remote-only board; `IsRemote` always `true`.
- ID falls back to GUID, then simple hash of the URL.

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
