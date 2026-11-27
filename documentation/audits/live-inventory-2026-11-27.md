# Live source inventory 2026-11-27

Full-registry probe of all 111 manifests through the plain fetcher with manifest slugs, query defaults, and 15-second timeouts. Counts are first-page yields from the manifest pipeline unless pagination is declared and supported.

## Totals

- Total manifests: 111
- Working: 54
- Records: 6778
- Non-working: 57
- Observed email mentions: 254 in 162 addresses over 138 domains

## By family

| Family | Sources | Working | Records |
| --- | --- | --- | --- |
| FeedsApi | 58 | 35 | 2661 |
| HtmlStatic | 13 | 5 | 124 |
| Ats | 26 | 14 | 3993 |
| Keyed | 14 | 0 | 0 |

## Top yields

| Source | Records | Latency ms |
| --- | --- | --- |
| ats-workable | 2157 | 1134 |
| devopsjobs | 1000 | 662 |
| breezyhr | 757 | 383 |
| arbeitnow | 300 | 3266 |
| ats-gem | 300 | 4494 |
| greenhouse | 243 | 552 |
| conservationjobs | 229 | 875 |
| ats-ashby | 174 | 618 |
| bamboohr | 141 | 2339 |
| realworkfromanywhere | 133 | 5954 |
| ats-oracle | 100 | 1693 |
| cryptojobslist | 100 | 1739 |
| remoteok | 100 | 7065 |
| weworkremotely | 100 | 1658 |
| academiccareers | 50 | 1795 |

## Fastest and slowest working sources

| Rank | Fastest | ms | Slowest | ms |
| --- | --- | --- | --- | --- |
| 1 | pythonjobs | 132 | railsjobs | 7375 |
| 2 | wellfound | 330 | remoteok | 7065 |
| 3 | breezyhr | 383 | ecojobs | 5978 |
| 4 | ukvisajobs | 388 | realworkfromanywhere | 5954 |
| 5 | huggingfacejobs | 439 | mycareersfuture | 5928 |
| 6 | cryptocurrencyjobs | 443 | aijobs | 5043 |
| 7 | jobylon | 462 | ats-gem | 4494 |
| 8 | greenhouse | 552 | jobicy | 4284 |
| 9 | jobspresso | 575 | arbeitnow | 3266 |
| 10 | ats-ashby | 618 | wordpressjobs | 2888 |

## Non-working sources

| Source | Family | Outcome | HTTP | Detail |
| --- | --- | --- | --- | --- |
| crunchboard | FeedsApi | blocked | 403 | http 403 |
| echojobs | FeedsApi | blocked | 429 | http 429 |
| headhunter | FeedsApi | blocked | 403 | http 403 |
| hiringcafe | HtmlStatic | blocked | 403 | http 403 |
| icrunchdata | FeedsApi | blocked | 403 | http 403 |
| loxo | Ats | blocked | 403 | http 403 |
| remotefirstjobs | FeedsApi | blocked | 403 | http 403 |
| snagajob | FeedsApi | blocked | 403 | http 403 |
| startupjobs | FeedsApi | blocked | 403 | http 403 |
| themuse | FeedsApi | blocked | 403 | http 403 |
| wuzzuf | HtmlStatic | blocked | 403 | http 403 |
| getonboard | FeedsApi | challenge | 200 | challenge: Challenge { kind: Captcha, status: 200, evidence: "{\"data\":[{\"id\" |
| gunio | HtmlStatic | challenge | 200 | challenge: Challenge { kind: Captcha, status: 200, evidence: "<!doctype html> <h |
| internshala | HtmlStatic | challenge | 200 | challenge: Challenge { kind: Captcha, status: 200, evidence: "<!DOCTYPE html>    |
| canadajobbank | FeedsApi | environment |  | target host resolution failed |
| careeronestop | FeedsApi | environment |  | request failed: error sending request for url (https://api.careeronestop.org/v1/ |
| functionalworks | HtmlStatic | environment |  | cannot read response body: error decoding response body |
| joinrise | FeedsApi | environment |  | target host resolution failed |
| nofluffjobs | FeedsApi | environment |  | response body exceeds 8388608 bytes |
| adzuna | Keyed | keyed |  | credential not present in environment |
| authenticjobs | Keyed | keyed |  | credential not present in environment |
| careerjet | Keyed | keyed |  | credential not present in environment |
| comeet | Keyed | keyed |  | credential not present in environment |
| crelate | Keyed | keyed |  | credential not present in environment |
| deel | FeedsApi | keyed |  | credential not present in environment |
| dice | FeedsApi | keyed |  | credential not present in environment |
| exa | Keyed | keyed |  | credential not present in environment |
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
| simplyhired | HtmlStatic | skipped |  | missing query substitution for l |
| androidjobs | FeedsApi | stale | 404 | http 404 |
| ats-rippling | HtmlStatic | stale | 200 | response parse failed: embedded JSON envelope not found |
| djinni | FeedsApi | stale | 200 | response parse failed: invalid RSS feed: unexpected `Event::Eof` |
| drupaljobs | FeedsApi | stale | 200 | response parse failed: invalid RSS feed: ill-formed document: expected `</br>`,  |
| eurojobs | FeedsApi | stale | 404 | http 404 |
| hackernews | FeedsApi | stale | 200 | parsed with manifest pipeline |
| jobindex | HtmlStatic | stale | 204 | response parse failed: embedded JSON source not found |
| jobvite | Ats | stale | 200 | invalid JSON payload: expected value at line 1 column 1 |
| ycjobs | Keyed | stale | 200 | response parse failed: no records matched the configured selectors |
