# Live source inventory 2026-11-27 (hardened)

Full-registry probe through the plain fetcher after the hardening changes. Counts are first-page yields unless a source declares supported pagination.

## Totals

- Total manifests: 107
- Working: 58
- Records: 7155
- Non-working: 49
- Observed email mentions: 306 in 172 addresses over 148 domains
- Median latency: 1442 ms
- p95 latency: 4899 ms
- Maximum latency: 12864 ms

## By family

| Family | Sources | Working | Records |
| --- | --- | --- | --- |
| FeedsApi | 55 | 36 | 2681 |
| HtmlStatic | 13 | 8 | 457 |
| Ats | 26 | 14 | 4017 |
| Keyed | 13 | 0 | 0 |

## Top yields

| Source | Records | Latency ms |
| --- | --- | --- |
| ats-workable | 2157 | 1531 |
| devopsjobs | 1000 | 12864 |
| breezyhr | 757 | 427 |
| arbeitnow | 300 | 2649 |
| ats-gem | 300 | 4155 |
| functionalworks | 259 | 4401 |
| greenhouse | 243 | 859 |
| conservationjobs | 229 | 830 |
| ats-ashby | 174 | 631 |
| bamboohr | 141 | 2588 |
| realworkfromanywhere | 133 | 2138 |
| ats-oracle | 100 | 1597 |
| cryptojobslist | 100 | 1377 |
| remoteok | 100 | 2255 |
| weworkremotely | 100 | 3673 |

## Fastest and slowest working sources

| Rank | Fastest | ms | Slowest | ms |
| --- | --- | --- | --- | --- |
| 1 | ukvisajobs | 127 | devopsjobs | 12864 |
| 2 | wellfound | 325 | ecojobs | 6513 |
| 3 | pythonjobs | 364 | jobvite | 4899 |
| 4 | jobspresso | 412 | functionalworks | 4401 |
| 5 | remotive | 417 | ats-gem | 4155 |
| 6 | breezyhr | 427 | weworkremotely | 3673 |
| 7 | berlinstartupjobs | 494 | workingnomads | 3525 |
| 8 | freelancercom | 607 | aijobs | 3032 |
| 9 | jobstreet | 624 | arbeitnow | 2649 |
| 10 | ats-ashby | 631 | bamboohr | 2588 |

## Non-working sources

| Source | Family | Outcome | HTTP | Detail |
| --- | --- | --- | --- | --- |
| crunchboard | FeedsApi | blocked | 403 | http 403 |
| echojobs | FeedsApi | blocked | 429 | http 429 |
| headhunter | FeedsApi | blocked | 403 | http 403 |
| loxo | Ats | blocked | 403 | http 403 |
| hiringcafe | HtmlStatic | challenge | 403 | challenge: Challenge { kind: Cloudflare, status: 403, evidence: "<!DOCTYPE html> |
| icrunchdata | FeedsApi | challenge | 403 | challenge: Challenge { kind: Cloudflare, status: 403, evidence: "<!DOCTYPE html> |
| remotefirstjobs | FeedsApi | challenge | 403 | challenge: Challenge { kind: Cloudflare, status: 403, evidence: "<!DOCTYPE html> |
| snagajob | FeedsApi | challenge | 403 | challenge: Challenge { kind: Cloudflare, status: 403, evidence: "<!DOCTYPE html> |
| startupjobs | FeedsApi | challenge | 403 | challenge: Challenge { kind: Cloudflare, status: 403, evidence: "<!DOCTYPE html> |
| wuzzuf | HtmlStatic | challenge | 403 | challenge: Challenge { kind: Cloudflare, status: 403, evidence: "<!DOCTYPE html> |
| canadajobbank | FeedsApi | environment |  | target host resolution failed |
| careeronestop | FeedsApi | environment |  | request failed: error sending request for url (https://api.careeronestop.org/v1/ |
| joinrise | FeedsApi | environment |  | target host resolution failed |
| nofluffjobs | FeedsApi | environment |  | response body exceeds 16777216 bytes |
| adzuna | Keyed | keyed |  | credential not present in environment |
| authenticjobs | Keyed | keyed |  | credential not present in environment |
| careerjet | Keyed | keyed |  | credential not present in environment |
| comeet | Keyed | keyed |  | credential not present in environment |
| crelate | Keyed | keyed |  | credential not present in environment |
| deel | FeedsApi | keyed |  | credential not present in environment |
| dice | FeedsApi | keyed |  | credential not present in environment |
| findwork | Keyed | keyed |  | credential not present in environment |
| fountain | FeedsApi | keyed |  | credential not present in environment |
| freshteam | Keyed | keyed |  | credential not present in environment |
| hiringthing | Keyed | keyed |  | credential not present in environment |
| infojobs | Keyed | keyed |  | credential not present in environment |
| ismartrecruit | Keyed | keyed |  | credential not present in environment |
| jazzhr | Ats | keyed |  | credential not present in environment |
| jobtechdev | Keyed | keyed |  | credential not present in environment |
| talroo | FeedsApi | keyed |  | credential not present in environment |
| usajobs | Keyed | keyed |  | credential not present in environment |
| web3career | FeedsApi | keyed |  | credential not present in environment |
| adp | Ats | skipped |  | missing slug substitution |
| ats-phenom | Ats | skipped |  | missing slug substitution |
| ats-recruiterflow | Ats | skipped |  | missing slug substitution |
| ats-successfactors | Ats | skipped |  | missing slug substitution |
| ats-talentlyft | Ats | skipped |  | missing slug substitution |
| ats-ukg | Ats | skipped |  | missing slug substitution |
| ats-workday | Ats | skipped |  | missing slug substitution |
| icims | Ats | skipped |  | missing slug substitution |
| manatal | Ats | skipped |  | missing slug substitution |
| ats-rippling | HtmlStatic | stale | 200 | response parse failed: embedded JSON envelope not found |
| djinni | FeedsApi | stale | 200 | response parse failed: invalid RSS feed: unexpected `Event::Eof` |
| gunio | HtmlStatic | stale | 200 | response parse failed: no records matched the configured selectors |
| hackernews | FeedsApi | stale | 200 | parsed with manifest pipeline |
| jobindex | HtmlStatic | stale | 204 | response parse failed: embedded JSON source not found |
| jobylon | Ats | stale | 404 | http 404 |
| themuse | FeedsApi | stale | 404 | http 404 |
| ycjobs | Keyed | stale | 200 | response parse failed: no records matched the configured selectors |
