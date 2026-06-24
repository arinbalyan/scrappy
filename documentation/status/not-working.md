# Not Working — Sites that return 0 or timeout

## Timeout (60s not enough)

These sites take longer than 60 seconds per scrape. Either the scraper hangs or the API is slow.

| Site | Type | Likely Cause |
|------|------|-------------|
| indeed | hybrid | API timeout or Playwright render timeout |
| greenhouse | ats | ATS API slow with 147 company slugs |
| 4dayweek | html_parse | Site response slow or scraper hangs |
| ats-crelate | ats | ATS API timeout |
| ats-gem | ats | ATS API timeout |
| ats-hiringthing | ats | ATS API timeout |
| ats-icims | ats | ATS API timeout |
| ats-personio | ats | ATS API timeout |
| ats-pinpoint | ats | ATS API timeout |
| ats-trakstar | ats | ATS API timeout |

## Blocked by WAF / Needs Proxy

These sites return HTTP 403 with anti-bot challenges. Fixable with residential proxies.

| Site | Type | Challenge |
|------|------|-----------|
| startupjobs | html_parse | 403 blocked |
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

## Broken / Stale Scrapers

These sites return HTTP 200 but the parser extracts 0 jobs. Site structure changed.

| Site | Type | Error |
|------|------|-------|
| google | hybrid | Captcha wall (needs stealth Playwright or paid API) |
| simplyhired | html_parse | Recently fixed — verify with latest build |
| hiringcafe | html_parse | API endpoint changed (405) |
| getonboard | html_parse | JSON schema changed |
| nofluffjobs | html_parse | Response exceeds 4MB limit |
| eurojobs | html_parse | 404 — site restructured |
| jobsinjapan | rss | RSS returns but no items |
| cryptojobslist | html_parse | RSS feed empty |
| djinni | rss | RSS feed blocked (Ukraine geo-restriction) |

## DNS / Connection Issues

| Site | Type | Error |
|------|------|-------|
| canadajobbank | html_parse | DNS lookup failed (API moved) |
| joinrise | html_parse | DNS lookup failed (domain dead) |
| careeronestop | html_parse | Connection timeout |
| jobdataapi | html_parse | Rate limited (429) |
| jobsch | html_parse | HTTP 422 (bad request) |
| usajobs | html_parse | HTTP 401 (needs API key) |

## Unsupported (no scraper implementation)

These sites have a constant in `model.Site` but no actual scraper wired in the engine:

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
