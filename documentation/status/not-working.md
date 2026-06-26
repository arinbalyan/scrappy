# Not Working — Sites that return 0 or timeout

## Timeout (120s still not enough)

These ATS scrapers have company slugs populated but the API endpoints are unreachable, slow, or require authentication:

| Site | Slugs | Likely Cause |
|------|-------|-------------|
| ats-adp | 31 | ADP API unreachable or slow |
| ats-freshteam | 35 | Needs API key |
| ats-icims | 39 | API slow |
| ats-jobvite | 32 | API timeout |
| ats-pinpoint | 34 | API slow/unreachable |
| ats-trakstar | 29 | API timeout |
| ats-ukg | 41 | API timeout |

## ATS with 0 jobs (slugs are stale/incorrect)

These scrapers find slugs from the embedded file but the slugs don't resolve to valid career pages. Many companies switch ATS providers, so researched slugs become stale:

| Site | Slugs | Verified Working | Issue |
|------|-------|-----------------|-------|
| ats-breezyhr | 35 | 3/35 (`checkr`,`algolia`,`zoom`) | Most slugs redirect to breezy.hr/ |
| ats-bullhorn | 35 | Unknown | Needs auth token |
| ats-comeet | 35 | Unknown | Needs API key |
| ats-crelate | 35 | 0 | Needs API key |
| ats-deel | 35 | Unknown | Needs auth |
| ats-fountain | 42 | Unknown | Needs API key |
| ats-hiringthing | 15 | Unknown | API endpoint may be wrong |
| ats-ismartrecruit | 37 | Unknown | Needs API key |
| ats-jazzhr | 26 | Unknown | Needs API key |
| ats-jobscore | 34 | Unknown | Slugs stale |
| ats-jobylon | 24 | Unknown | Slugs stale |
| ats-joincom | 26 | Unknown | Slugs stale |
| ats-loxo | 26 | Unknown | Slugs stale |
| ats-manatal | 15 | Unknown | Slugs stale |
| ats-mercor | 28 | Unknown | Slugs stale |
| ats-oracle | 35 | Unknown | Needs company ID format |
| ats-personio | 53 | Unknown | URL format wrong |
| ats-phenom | 34 | Unknown | Slugs stale |
| ats-recruiterflow | 35 | Unknown | Slugs stale |
| ats-successfactors | 31 | Unknown | Slugs stale |
| ats-taleo | 40 | Unknown | Slugs stale |
| ats-teamtailor | 50 | Unknown | Slugs stale |
| ats-workday | 69 | Unknown | URL format uncertain |

## Blocked by WAF / Needs Proxy

| Site | Type | Challenge |
|------|------|-----------|
| startupjobs | html_parse | 403 blocked |
| bayt | html_parse | 403 blocked |
| echojobs | html_parse | 403 blocked |
| headhunter | html_parse | 403 blocked |
| icrunchdata | html_parse | 403 blocked |
| berlinstartupjobs | html_parse | 403 blocked |
| stepstone | html_parse | 403 blocked |
| tesla | html_parse | Akamai challenge |
| monster | hybrid | DataDome challenge |
| ziprecruiter | html_parse | 403 blocked |
| academiccareers | html_parse | 403 blocked |
| wellfound | html_parse | 403 blocked (AngelList) |

## Dead / Moved APIs

| Site | Type | Error |
|------|------|-------|
| canadajobbank | html_parse | DNS lookup failed (API moved) |
| careeronestop | html_parse | Connection timeout |
| joinrise | html_parse | DNS lookup failed (domain dead) |
| jobdataapi | html_parse | API parameters changed |
| jobsch | html_parse | HTTP 422 (bad request) |
| eurojobs | html_parse | 404 — site restructured |

## Unsupported (no scraper implementation)

| Site |
|------|
| devitjobs |
| germantechjobs |
| greenjobsboard |
| guardianjobs |
| opensourcedesignjobs |
| powertofly |
| swissdevjobs |
| techcareers |
| undpjobs |
| virtualvocations |
