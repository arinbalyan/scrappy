# Unregistered source port notes

Date: 2026-11-27

Scope: 20 historical source IDs that were not registered in the declarative
source registry. Each was reviewed against the historical
`internal/scraper/<id>/scraper.go` implementation and current public endpoint
documentation. A source was ported only when a public, no-auth JSON or RSS
endpoint returns complete job records whose fields map directly to the
manifest adapter contract. Fixtures use synthetic values only.

## Ported

### gem -> ats-gem

- Historical behavior: single batched GraphQL POST to
  `https://jobs.gem.com/api/public/graphql/batch` with `JobBoardTheme` and
  `JobBoardList` operations. Job URLs were composed from board slug, job ID,
  and `extId`.
- Current endpoint: `GET https://api.gem.com/job_board/v0/{vanity_url_path}/job_posts/`.
  Gem states that Job Board API endpoints are public and need no authentication,
  and that the API uses no pagination.
- Response shape: a flat JSON array with `title`, `absolute_url`,
  `location.name`, `plain_text_description`, and department objects.
  `absolute_url` supplies the complete hosted job URL without composition.
- Port decision: manifest `ats-gem` in the `ats` family with `{slug}`,
  nested field aliases, and a synthetic six-file fixture. The old GraphQL
  batch endpoint is not used because its job URL must be composed from
  multiple response fields.
- Evidence: https://api.gem.com/job_board/v0/reference
  https://help.gem.com/databases/gem-help-center/the-job-board-api
  https://deriverge.com/tools/job-board-api/gem/

## Skipped

### arbeitsagentur

- Historical behavior: `ARBEITSAGENTUR_API_KEY` environment variable and a
  search endpoint, followed by a detail request with a base64-encoded `refnr`.
- Current docs require the `X-API-Key` header with the public client id
  `jobboerse-jobsuche`, and detail URLs are built from `refnr`.
- Skip reason: the endpoint is keyed, and the job URL cannot be composed from
  response fields by the manifest adapter.
- Evidence: https://github.com/bundesAPI/jobsuche-api/blob/main/openapi.yaml
  https://github.com/bundesAPI/jobsuche-api/blob/main/README.md

### ashby

- Historical behavior: public posting API with a company slug.
- Skip reason: already registered as `ats-ashby` with manifests, fixtures,
  docs, and replay coverage. Porting it again would duplicate the source id.
- Evidence: https://developers.ashbyhq.com/docs/public-job-posting-api

### avature

- Historical behavior: tenant HTML pages and regex extraction, with a browser
  fallback for some portals.
- Skip reason: no public no-auth JSON or RSS endpoint; tenant-specific HTML
  parsing is outside the declared port criteria.
- Evidence: historical `internal/scraper/avature/scraper.go` in `origin/main`.

### bayt

- Historical behavior: HTML search and detail pages.
- Skip reason: Bayt publishes no developer API for reading listings; current
  JSON offerings are third-party scrapers behind their own API keys.
- Evidence: https://datapika.com/scrape/bayt-jobs
  https://github.com/happyendpointhq/bayt-api

### bullhorn

- Historical behavior: Bullhorn REST login, `BhRestToken` session, and
  `JobOrder` entity search.
- Current public API exists at `public-rest{cls}.bullhornstaffing.com/rest-services/{token}/search/JobOrder`,
  but its documented field list has no direct job URL, and it requires a
  tenant token and swimlane obtained from Bullhorn Support.
- Skip reason: tenant-specific setup and no direct URL field.
- Evidence: https://bullhorn.github.io/Public-API/
  https://bullhorn.github.io/Getting-Started-with-REST/

### careerbuilder

- Historical behavior: HTML result pages with a browser fallback.
- Skip reason: no public no-auth job search JSON or RSS feed in current
  documentation; published APIs are partner integrations for posting jobs.
- Evidence: https://github.com/careerbuilder/DataScienceAPIDocumentation

### francetravail

- Historical behavior: OAuth client-credentials token followed by the API
  recherche endpoint.
- Current status: France Travail removed the job-offer API from public
  exposure pending a review of distribution and reuse terms.
- Skip reason: no longer publicly exposed, and it was an authenticated API.
- Evidence: https://www.data.gouv.fr/dataservices/api-offres-demploi
  https://api.francetravail.io/fr/portail/documentation

### google

- Historical behavior: Google search result scraping and embedded JSON-LD.
- Skip reason: Google has no public API for Google Jobs search results;
  available products cover publishing your own jobs or paid SERP access.
- Evidence: https://publicapis.dev/blog/google-jobs-api
  https://cleanjobdata.com/articles/google-jobs-api

### indeed

- Historical behavior: GraphQL endpoint with optional key and HTML fallback.
- Current GraphQL partner APIs require an access token; the old Publisher
  search API is retired.
- Skip reason: no public no-auth search endpoint.
- Evidence: https://docs.indeed.com/getstarted/integrate-and-call-apis
  https://www.hirebase.org/blogs/indeed-linkedin-glassdoor-jobs-api

### joincom

- Historical behavior: public API flow depended on scraping a tenant id from
  HTML before calling `/api/public/...`.
- Current docs list `https://api.join.com/v3/jobs` with Bearer token auth.
- Skip reason: keyed endpoint and a tenant discovery step that the manifest
  adapter cannot express.
- Evidence: https://docs.join.com/reference/listjobs

### linkedin

- Historical behavior: guest jobs endpoint with browser fallback and auth
  challenge handling.
- Skip reason: no public no-auth job search API; guest endpoints are subject
  to challenges and are not a stable documented feed.
- Evidence: https://learn.microsoft.com/en-us/linkedin/talent/apply-connect/create-apply-connect-jobs
  https://www.hirebase.org/blogs/indeed-linkedin-glassdoor-jobs-api

### monster

- Historical behavior: HTML search and detail pages.
- Skip reason: no public no-auth JSON or RSS job search feed.
- Evidence: historical `internal/scraper/monster/scraper.go` in `origin/main`.

### stepstone

- Historical behavior: HTML result pages with a browser fallback.
- Current public API is an employer JobFeed XML PUSH/PULL integration for
  feeding jobs into Stepstone, not a public job search endpoint.
- Skip reason: wrong direction and no no-auth search JSON or RSS.
- Evidence: https://api.stepstone.com/wp-content/uploads/2026/07/AJ360_XML-Guide.pdf

### tesla

- Historical behavior: Tesla careers pages and internal JS-driven endpoints.
- Skip reason: careers search is JavaScript-only with undocumented internal
  endpoints; public developer APIs are unrelated to job search and use OAuth.
- Evidence: https://www.tesla.com/careers/search
  https://developer.tesla.com/

### upwork

- Historical behavior: GraphQL job search.
- Current API uses `https://api.upwork.com/graphql` with OAuth 2.0 and
  client credentials.
- Skip reason: keyed GraphQL API, outside the port criteria.
- Evidence: https://www.upwork.com/developer/documentation/graphql/api/docs/index.html
  https://support.upwork.com/hc/en-us/articles/115015933448-API-authentication-and-security

### ziprecruiter

- Historical behavior: internal endpoint with a hardcoded Basic auth header.
- Current partner APIs also require an API key via Basic auth.
- Skip reason: no public no-auth job search endpoint and no real credentials
  may be used in fixtures.
- Evidence: https://www.ziprecruiter.com/partner/documentation/authentication/
  https://www.ziprecruiter.com/partner/documentation/job-api/

### ats-mercor

- Historical behavior: unauthenticated GET to
  `https://aws.api.mercor.com/work/listings-explore-page`, then job URLs built
  from `https://work.mercor.com/jobs/{listingId}/{slugified title}`.
- Skip reason: no official public API reference found, and the URL requires
  composing a listing id with a slugified title, which manifest aliases
  cannot express. Mercor documentation links to login and reports a 401 on
  its MCP surface.
- Evidence: https://apis.io/providers/mercor/
  https://work.mercor.com/

### ats-personio

- Current endpoint: `https://{slug}.jobs.personio.de/xml?language=en`, a
  documented public XML feed.
- Skip reason: the adapter has no arbitrary XML parser, and the real job URL
  is composed from the tenant slug plus `<id>`, which manifest aliases
  cannot express.
- Evidence: https://developer.personio.de/v1.0/reference/get_xml
  https://support.personio.de/hc/en-us/articles/207576365-Integrate-jobs-from-Personio-into-your-website-via-XML

### ats-taleo

- Historical behavior: anonymous POST to
  `https://{company}.taleo.net/careersection/rest/jobboard/searchjobs` with a
  tenant and career-section seed.
- Skip reason: job URLs are composed from the tenant, career section, and
  `contestNo`; current guidance also requires reading a portal id from the
  page before searching. Those steps are outside the manifest adapter.
- Evidence: https://github.com/danielhagever/job-board-apis
  https://docs.oracle.com/en/cloud/saas/taleo-enterprise/21a/otrcg/central-configuration.html

