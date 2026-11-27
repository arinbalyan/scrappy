# Slug verification audit — 2026-11-27

Status: complete. Repo `/home/nemesis/Projects/job-hunter-repos/scrappy`, branch `oculus`, commit `9310816`.

Scope: every source with `health = "requires_slug"` in `src/sources/manifests/`. There are 22 such manifests: 21 under `ats/` and `html_static/ats-rippling.toml`. The task calls out `ats-rippling` separately; it is included in the 22 rows here and has its own section. `rg -l 'requires_slug' src/sources/manifests` returned exactly these 22 files, so there is no separate 23rd source. No manifest, fixture, Rust file, or other repository file outside `documentation/audits/` was changed by this audit.

Probe window: 2026-11-27 15:49–16:04 UTC. Network probes used the normal desktop user agent below. The sandbox could not resolve public DNS (`curl` exit 6), so the probes were rerun with escalated network access; no credentials were used.

```text
Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36
```

## How to read this report

- **HTTP** is the final status after any redirect shown with `→`. `DNS fail` means `curl` exited 6 before any response.
- **Body** is what the response actually looked like: JSON with a jobs envelope, HTML job listings, RSS/XML, a redirect, or an error document.
- **Count** is the record count available from that response. For paginated endpoints, the table says whether the number is a page count or the vendor-reported total.
- A valid board that returned zero jobs is marked **0 jobs**, not as a broken slug.
- **Recommended seed** is the single slug or tuple worth putting in a manifest once the endpoint is corrected. `none` means no public seed was found.

## Recommended seeds at a glance

| Source | Recommended seed | Verified request | HTTP | Body | Count |
| --- | --- | --- | --- | --- | --- |
| `ats-ashby` | `notion` | `GET https://api.ashbyhq.com/posting-api/job-board/notion` | 200 | JSON `jobs` | 174 jobs |
| `ats-gem` | `coupa-software-inc-ats-1` | `GET https://api.gem.com/job_board/v0/coupa-software-inc-ats-1/job_posts/` | 200 | JSON array | 300 jobs |
| `ats-oracle` | `jpmc` | `GET https://jpmc.fa.oraclecloud.com/hcmRestApi/resources/latest/recruitingCEJobRequisitions?...&expand=requisitionList&finder=findReqs;siteNumber=CX_45001,limit=5` | 200 | JSON `items[0].requisitionList` | 6,760 total; 5 in page |
| `ats-pinpoint` | `article` | `GET https://article.pinpointhq.com/postings.json` | 200 | JSON `data` | 5 postings |
| `ats-recruitee` | `transperfect` | `GET https://transperfect.recruitee.com/api/offers` | 200 | JSON `offers` | 802 offers |
| `ats-recruiterflow` | none | `GET https://api.recruiterflow.com/api/external/job/list` | 400 | JSON error | no public key |
| `ats-rippling` | `barrys-careers` | `GET https://ats.rippling.com/barrys-careers/jobs` | 200 | HTML `__NEXT_DATA__` | 227 total |
| `ats-smartrecruiters` | `Canva` | `GET https://api.smartrecruiters.com/v1/companies/Canva/postings?offset=0&limit=100` | 200 | JSON `content` | 33 postings |
| `ats-talentlyft` | none | `GET https://api.talentlyft.com/v2/public/careers/jobs?page=1&perPage=50` | 200 | JSON `Results` | 1 non-job entry |
| `ats-teamtailor` | `polestar` | `GET https://polestar.teamtailor.com/jobs.json` | 200 | JSON Feed `items` | 15 jobs |
| `ats-trakstar` | `whatfix101` | `GET https://whatfix101.hire.trakstar.com/jobfeeds/whatfix101` | 200 | RSS `item` | 27 jobs |
| `ats-ukg` | none | `GET https://recruiting.ultipro.com/QUE1003QSTX/JobBoard/5dcfdc99-74ef-40c7-803a-1e5bd5fefa87/` | 200 | HTML board | no single-slug form |
| `ats-workable` | `huzzle` | `GET https://apply.workable.com/api/v1/widget/accounts/huzzle` | 200 | JSON `jobs` | 2,157 jobs |
| `ats-workday` | `nvidia` + `NVIDIAExternalCareerSite` | `POST https://nvidia.wd5.myworkdayjobs.com/wday/cxs/nvidia/NVIDIAExternalCareerSite/jobs` | 200 | JSON `jobPostings` | 2,000 total; 20 in page |
| `bamboohr` | `401auto` | `GET https://401auto.bamboohr.com/careers/list` | 200 | JSON `result` | 141 jobs |
| `breezyhr` | `thirdchannel-inc` | `GET https://thirdchannel-inc.breezy.hr/json` | 200 | JSON array | 757 jobs |
| `greenhouse` | `stripe` | `GET https://boards-api.greenhouse.io/v1/boards/stripe/jobs?content=true` | 200 | JSON `jobs` | 765 jobs |
| `icims` | `careers-daddario` | `GET https://careers-daddario.icims.com/jobs/search?pr=0&o=0&mode=job&iis=Internet` | 200 | HTML job listings | 4 links on page 1 |
| `jobscore` | `cresa` | `GET https://careers.jobscore.com/jobs/cresa/feed.json?sort=date&limit=25` | 200 | JSON `jobs` | 24 jobs |
| `jobvite` | `qLHaVfwh` (Egnyte company ID) | `GET https://app.jobvite.com/CompanyJobs/Xml.aspx?c=qLHaVfwh` | 200 | XML `job` | 49 jobs |
| `jobylon` | `e2eea2b2-3d8a-439d-beb5-6bf05741f38c` (Tele2) | `GET https://feed.jobylon.com/feeds/e2eea2b2-3d8a-439d-beb5-6bf05741f38c/?format=json` | 200 | JSON array | 24 jobs |
| `manatal` | `manatal` | `GET https://api.careers-page.com/open/v1/career-pages/manatal/job-posts` | 200 | JSON `items` | 40 total; 20 in page |

## Candidate probe results

### `ats-ashby`

Manifest route is current and unauthenticated. Body is `{"jobs":[...],"apiVersion":...}`.

| Candidate slug | HTTP | Body verdict | Jobs |
| --- | --- | --- | --- |
| `notion` | 200 | JSON `jobs` | 174 |
| `openai` | 200 | JSON `jobs` | 875 |
| `ramp` | 200 | JSON `jobs` | 183 |
| `linear` | 200 | JSON `jobs` | 30 |
| `cursor` | 200 | JSON `jobs` | 87 |
| `elevenlabs` | 200 | JSON `jobs` | 108 |
| `replit` | 200 | JSON `jobs` | 38 |
| `supabase` | 200 | JSON `jobs` | 82 |
| `vercel` | 200 | JSON `jobs` | 0 (empty board) |
| `cohere` | 200 | JSON `jobs` | 122 |

Recommendation: `notion`. All ten candidates are valid Ashby tenants except that `vercel` currently has no listings. The manifest envelope `["jobs"]` matches the live response.

### `ats-gem`

Manifest route is current. The response is a top-level JSON array, not an object with a `jobs` envelope.

| Candidate slug | HTTP | Body verdict | Jobs |
| --- | --- | --- | --- |
| `coupa-software-inc-ats-1` | 200 | JSON array | 300 |
| `linktree` | 200 | JSON array | 22 |
| `function-health` | 200 | JSON array | 29 |

Recommendation: `coupa-software-inc-ats-1`. The 300-entry array may be a response cap, so treat 300 as a first-response count. `linktree` is the smaller, cleaner seed if a low-volume board is preferred.

### `ats-oracle`

The manifest host form `https://{slug}.fa.oraclecloud.com` did not resolve for `ejwl` and `eeho`. Oracle tenants also serve on regional hosts such as `{tenant}.fa.{region}.oraclecloud.com`, and the response only contains `requisitionList` when `expand=requisitionList` is added.

| Candidate | Request form | HTTP | Body verdict | Jobs |
| --- | --- | --- | --- | --- |
| `ejwl` | regionless manifest host | DNS fail (curl exit 6) | no response | 0 |
| `eeho` | regionless manifest host | DNS fail (curl exit 6) | no response | 0 |
| `jpmc` | regionless manifest host | 200 | JSON search metadata only | 6,760 total; no `requisitionList` |
| `ejwl` | `ejwl.fa.us2.oraclecloud.com` | 200 | JSON search metadata | 13,174 total |
| `jpmc` | regionless host + `expand=requisitionList` | 200 | JSON `items[0].requisitionList` | 6,760 total; 5 in page |
| `ejwl` | `ejwl.fa.us2.oraclecloud.com` + `expand=requisitionList` | 200 | JSON `items[0].requisitionList` | 13,172 total; 5 in page |

Recommendation: `jpmc` with the manifest URL corrected to append `&expand=requisitionList`. The manifest's `siteNumber=CX_45001` returned plausible results for `jpmc`, so keep that site number for this seed. Tenants on regional hosts need the region in the hostname.

### `ats-pinpoint`

Manifest route is current. Body is `{"data":[...]}`.

| Candidate slug | HTTP | Body verdict | Jobs |
| --- | --- | --- | --- |
| `article` | 200 | JSON `data` | 5 |
| `deel` | 200 | JSON `data` | 1 |

Recommendation: `article`.

### `ats-recruitee`

Manifest route is current. Body is `{"offers":[...]}`.

| Candidate slug | HTTP | Body verdict | Offers |
| --- | --- | --- | --- |
| `bunq` | 200 | JSON `offers` | 19 |
| `helloprint` | 200 | JSON `offers` | 9 |
| `transperfect` | 200 | JSON `offers` | 802 |

Recommendation: `transperfect` for volume, or `bunq` for a small board.

### `ats-recruiterflow`

The seed is the `rf-api-key` credential itself. No public tenant can be probed.

| Candidate | HTTP | Body verdict |
| --- | --- | --- |
| `example-co` | 400 | JSON `{"message":"Please supply an API key"}` |
| `mbrpartners` | 400 | JSON `{"message":"Please supply an API key"}` |

Recommendation: none. A seed requires a vendor-issued API key. `mbrpartners` is a public portal name and does not authenticate the API.

### `ats-rippling`

Manifest route is an HTML board page with embedded Next.js data. The manifest's `__NEXT_DATA__` envelope path `["props","pageProps","dehydratedState","queries","0","state","data","items"]` matches the tested boards.

| Candidate slug | HTTP | Body verdict | Jobs |
| --- | --- | --- | --- |
| `rippling` | 307 → `http://www.rippling.com/careers/open-roles` | redirect, not a board | 0 |
| `barrys-careers` | 200 | HTML `__NEXT_DATA__` | 227 total (20 in first query) |
| `incredible-health` | 200 | HTML `__NEXT_DATA__` | 1 |
| `accelerant` | 200 | HTML `__NEXT_DATA__` | 24 |
| `seentvhiring` | 200 | HTML `__NEXT_DATA__` | 2 |
| `chess` | 200 | HTML `__NEXT_DATA__` | 12 |

Recommendation: `barrys-careers`. Drop `rippling` from the candidate list because that slug redirects to Rippling's own marketing careers site rather than an `ats.rippling.com` board.

### `ats-smartrecruiters`

The manifest's `/jobs` path is dead for all tested tenants. SmartRecruiters' live public route is `/postings`, which also matches the manifest's `content` envelope and `totalFound` metadata.

Manifest route `GET https://api.smartrecruiters.com/v1/companies/{slug}/jobs?offset=0&limit=100`:

| Candidate slug | HTTP | Body verdict |
| --- | --- | --- |
| `Canva` | 404 | HTML `Error` |
| `BoschGroup` | 404 | HTML `Error` |
| `ServiceNow` | 404 | HTML `Error` |
| `CERN` | 404 | HTML `Error` |
| `Accor` | 404 | HTML `Error` |
| `DeliveryHero` | 404 | HTML `Error` |
| `Dominos` | 404 | HTML `Error` |
| `SGS` | 404 | HTML `Error` |
| `GreeneKing` | 404 | HTML `Error` |

Corrected route `GET https://api.smartrecruiters.com/v1/companies/{slug}/postings?offset=0&limit=100`:

| Candidate slug | HTTP | Body verdict | Total postings |
| --- | --- | --- | --- |
| `Canva` | 200 | JSON `content` | 33 |
| `BoschGroup` | 200 | JSON `content` | 5,177 |
| `ServiceNow` | 200 | JSON `content` | 969 |
| `CERN` | 200 | JSON `content` | 63 |
| `Accor` | 200 | JSON `content` | 465 |
| `AccorHotel` | 200 | JSON `content` | 5,931 |
| `DeliveryHero` | 200 | JSON `content` | 803 |
| `Dominos` | 200 | JSON `content` | 25,262 |
| `SGS` | 200 | JSON `content` | 4,008 |
| `GreeneKing` | 200 | JSON `content` | 4,060 |

Recommendation: `Canva` after changing the manifest path from `/jobs` to `/postings`. `Dominos` is the volume option.

### `ats-talentlyft`

The public route itself exists and returns JSON, but no current tenant with real jobs was found.

| Candidate | HTTP | Body verdict |
| --- | --- | --- |
| `example-co` | 404 | JSON `{"Message":...,"Errors":[],"ErrorCode":...}` |
| `talentlyft` | 200 | JSON `Results` = 0 |
| `careers` | 200 | JSON `Results` = 1, an open-application placeholder rather than a real posting |

Recommendation: none. The `careers` tenant is TalentLyft's own career site and returned `Your next job that we should create (open application)`. The vendor's public API reference documents the `/v2/public/{subdomain}/jobs` route, so the endpoint shape is right; the missing piece is a customer subdomain with live postings.

### `ats-teamtailor`

The manifest widget route is dead. Teamtailor's public JSON Feed is served on the tenant subdomain.

Manifest route `GET https://career.teamtailor.com/widget/jobs/{slug}`:

| Candidate slug | HTTP | Body verdict |
| --- | --- | --- |
| `polestar` | 404 | HTML `Teamtailor` error page |
| `deezer` | 404 | HTML `Teamtailor` error page |
| `tibber` | 404 | HTML `Teamtailor` error page |
| `oatly` | 404 | HTML `Teamtailor` error page |
| `anicura` | 404 | HTML `Teamtailor` error page |

Corrected route `GET https://{slug}.teamtailor.com/jobs.json`:

| Candidate slug | HTTP | Body verdict | Jobs |
| --- | --- | --- | --- |
| `polestar` | 200 | JSON Feed `items` | 15 |
| `deezer` | 200 | JSON Feed `items` | 18 |
| `tibber` | 200 | JSON Feed `items` | 13 |
| `oatly` | 200 | JSON Feed `items` | 25 |
| `anicura` | 404 | empty JSON | 0 |

Recommendation: `polestar` with `https://polestar.teamtailor.com/jobs.json`. The response is JSON Feed, so the manifest's `data` envelope and field paths need adjustment.

### `ats-trakstar`

The manifest API path is protected and dead. The public feed is RSS at `/jobfeeds/{company}`.

Manifest route `GET https://{slug}.hire.trakstar.com/api/v1/openings`:

| Candidate slug | HTTP | Body verdict |
| --- | --- | --- |
| `ca852383` | 301 → 401 at `/api/v1/openings/` | JSON `error_type` |
| `chargebee` | 301 → 401 at `/api/v1/openings/` | JSON `error_type` |

Corrected route `GET https://{slug}.hire.trakstar.com/jobfeeds/{company}`:

| Candidate slug | HTTP | Body verdict | Jobs |
| --- | --- | --- | --- |
| `ca852383` | 200 | RSS, title "Jobs at US Court of Appeals for the Eighth Circuit" | 0 |
| `chargebee` | 200 | RSS, last item dated 2014 | 1 |
| `whatfix101` | 200 | RSS, title "Jobs at Whatfix" | 27 |

Recommendation: `whatfix101` with `https://whatfix101.hire.trakstar.com/jobfeeds/whatfix101`. `ca852383` is a real board but currently empty. `chargebee` is effectively stale. The feed is case-sensitive on the company segment.

### `ats-ukg`

The manifest's `OpportunitySearch` GET does not exist on the tenant host.

| Probe | HTTP | Body verdict |
| --- | --- | --- |
| `https://recruiting.ultipro.com/QUE1003QSTX/OpportunitySearch` | 404 | HTML error page |
| `https://recruiting.ultipro.com/QUE1003QSTX/JobBoard/5dcfdc99-74ef-40c7-803a-1e5bd5fefa87/` | 200 | HTML job board, 485,844 bytes |
| Same board, `JobBoardView/LoadSearchResults` with GET | 415 | HTML error |
| `https://recruiting.ultipro.com/EMC1002EMCGI/JobBoard/cc563430-39f7-4c82-bdfe-62f295bd857a/` | 404 | HTML error |

Recommendation: none for the manifest's current one-field shape. A working UKG board needs a tenant code plus a board UUID, for example `QUE1003QSTX` + `5dcfdc99-74ef-40c7-803a-1e5bd5fefa87`, and the list API is a POST (`LoadSearchResults`), not the manifest's GET. A single seed cannot identify this source.

### `ats-workable`

Manifest route is current. Body is `{"name":...,"jobs":[...]}`.

| Candidate slug | HTTP | Body verdict | Jobs |
| --- | --- | --- | --- |
| `huzzle` | 200 | JSON `jobs` | 2,157 |
| `ibm` | 200 | JSON `jobs` | 0 (empty board) |
| `huggingface` | 200 | JSON `jobs` | 9 |

Recommendation: `huzzle`. `ibm` resolves to an empty board, so it is not a useful seed.

### `ats-workday`

The manifest hardcodes data centre `wd5` and site `External`. Neither is generally correct. The API needs the full tuple `{tenant}.wd{dc}.myworkdayjobs.com/wday/cxs/{tenant}/{site}/jobs`.

Manifest route `POST https://{slug}.wd5.myworkdayjobs.com/wday/cxs/{slug}/External/jobs`:

| Candidate slug | HTTP | Body verdict |
| --- | --- | --- |
| `nvidia` | 404 | JSON `errorCode` |
| `salesforce` | 422 | JSON `HTTP_422` |
| `walmart` | 422 | JSON `HTTP_422` |
| `wellsfargo` | 422 | JSON `HTTP_422` |
| `bankofamerica` | 422 | JSON `HTTP_422` |
| `target` | 404 | JSON "not found: Job_Posting_Site_ID=External" |
| `netflix` | 422 | JSON `HTTP_422` |
| `verizon` | 422 | JSON `HTTP_422` |

Corrected tuples tested:

| Tenant + site | HTTP | Body verdict | Total jobs |
| --- | --- | --- | --- |
| `nvidia.wd5` + `NVIDIAExternalCareerSite` | 200 | JSON `jobPostings` | 2,000 |
| `salesforce.wd12` + `External_Career_Site` | 200 | JSON `jobPostings` | 1,579 |
| `target.wd5` + `TargetCareers` | 200 | JSON `jobPostings` | 2,000 |
| `verizon.wd12` + `verizon-careers` | 200 | JSON `jobPostings` | 1,201 |
| `netflix.wd108` + `Netflix` | 200 | JSON `jobPostings` | 0 |
| `walmart.wd5` + `WalmartExternal` | 422 | JSON `HTTP_422` | not measured |
| `bankofamerica.wd1` + `us-emplsv` | 422 | JSON `HTTP_422` | not measured |
| `wellsfargo.wd1` + `WellFargoCareers` | 422 | JSON `HTTP_422` | not measured |

Recommendation: `nvidia` with `NVIDIAExternalCareerSite`. `salesforce`, `target`, and `verizon` are also verified with their own site paths. The `netflix` tuple resolves but has no jobs, so it is not a usable seed. For `walmart`, a public job URL dated 2026-03-16 uses `walmart.wd5.myworkdayjobs.com/en-US/WalmartExternal/...`, which confirms the tenant and site shape, but this probe's CXS request returned 422. The `bankofamerica` and `wellsfargo` tuples remain unverified.

### `bamboohr`

Manifest route is current. Body is `{"meta":...,"result":[...]}`.

| Candidate slug | HTTP | Body verdict | Jobs |
| --- | --- | --- | --- |
| `401auto` | 200 | JSON `result` | 141 |
| `eggharborcafe` | 200 | JSON `result` | 67 |

Recommendation: `401auto`. Both candidates return JSON, so neither hit the known non-tenant redirect-to-HTML failure mode.

### `breezyhr`

Manifest route is current. Body is a top-level JSON array.

| Candidate slug | HTTP | Body verdict | Jobs |
| --- | --- | --- | --- |
| `thirdchannel-inc` | 200 | JSON array | 757 |

Recommendation: `thirdchannel-inc`.

### `greenhouse`

Manifest route is current. Body is `{"jobs":[...],"meta":...}`.

| Candidate slug | HTTP | Body verdict | Jobs |
| --- | --- | --- | --- |
| `stripe` | 200 | JSON `jobs` | 765 |
| `gitlab` | 200 | JSON `jobs` | 243 |
| `figma` | 200 | JSON `jobs` | 141 |
| `airbnb` | 200 | JSON `jobs` | 201 |
| `notion` | 404 | JSON `{"status":404,"error":"..."}` | 0 |
| `linear` | 404 | JSON `{"status":404,"error":"..."}` | 0 |
| `vercel` | 200 | JSON `jobs` | 86 |
| `databricks` | 200 | JSON `jobs` | 888 |
| `spacex` | 200 | JSON `jobs` | 2,889 |
| `andurilindustries` | 200 | JSON `jobs` | 2,599 |

Recommendation: `stripe`. `notion` and `linear` are Ashby boards, not Greenhouse boards, so remove them from the Greenhouse candidate set.

### `icims`

iCIMS returns HTML, not JSON. The manifest's `parser = "json"` cannot work for these routes even when the search page is live.

| Candidate slug | HTTP | Body verdict |
| --- | --- | --- |
| `careers-daddario` | 200 | HTML job listings, title "Careers at D'Addario \| Job Listings at D'Addario & Company", 4 job links on page 1 |
| `careers-novanthealth` | 404 | HTML "gone: careers-novanthealth.icims.com" |
| `careers-rei` | 302 → 200 | HTML, 346,293 bytes, no job-listing links found on the returned page |

Recommendation: `careers-daddario` with an HTML parser. `careers-rei` redirects to a live iCIMS page but the returned body did not expose job links in this probe, so it is not recommended. `careers-novanthealth` is gone.

### `jobscore`

Manifest route is current. Body is a JSON object with a `jobs` array.

| Candidate slug | HTTP | Body verdict | Jobs |
| --- | --- | --- | --- |
| `cresa` | 200 | JSON `jobs` | 24 |
| `allogene` | 200 | JSON `jobs` | 9 |

Recommendation: `cresa`.

### `jobvite`

The manifest's `/api/v2/job-feed/{slug}` route is dead. Every tested slug redirected to Jobvite's own support page. The working public feeds are the HTML board and the XML feed.

Manifest route:

| Candidate slug | HTTP | Body verdict |
| --- | --- | --- |
| `egnyte` | 302 → 200 | Jobvite support page, `?invalid=1` |
| `loandepot` | 302 → 200 | Jobvite support page, `?invalid=1` |
| `tylertech` | 302 → 200 | Jobvite support page, `?invalid=1` |
| `q6NaVfwI` | 302 → 200 | Jobvite support page, `?invalid=1` |

Corrected public routes:

| Candidate | Route | HTTP | Body verdict | Jobs |
| --- | --- | --- | --- | --- |
| `egnyte` | `https://jobs.jobvite.com/egnyte` | 200 | HTML, "Egnyte Careers" | 29 links |
| `loandepot` | `https://jobs.jobvite.com/loandepot` | 200 | HTML, "loanDepot Careers" | 0 links on returned page |
| `tylertech` | `https://jobs.jobvite.com/tylertech` | 200 | HTML, "Tyler Technologies Careers" | 111 links |
| `egnyte` | `https://app.jobvite.com/CompanyJobs/Xml.aspx?c=qLHaVfwh` | 200 | XML `<result><job>...` | 49 jobs |

Recommendation: `qLHaVfwh` with `https://app.jobvite.com/CompanyJobs/Xml.aspx?c=qLHaVfwh`, or `egnyte` with the HTML board route. The manifest seed field cannot hold both the friendly board slug and the numeric/short company ID, so document the mapping. The parser also needs to change from JSON `requisitions` to the XML `result/job` structure. `q6NaVfwI` did not work as a board slug in the tested route.

### `jobylon`

Manifest route is current. Body is a top-level JSON array.

| Candidate slug | HTTP | Body verdict | Jobs |
| --- | --- | --- | --- |
| `150d2f7a9df44df8a1ac9c5c794f7220` (Northvolt) | 200 | JSON array | 0 (empty feed) |
| `e2eea2b2-3d8a-439d-beb5-6bf05741f38c` (Tele2) | 200 | JSON array | 24 |

Recommendation: the Tele2 UUID. The Northvolt feed still resolves but is empty.

### `manatal`

The manifest host and path are stale. The current public API is on `api.careers-page.com`.

| Candidate / route | HTTP | Body verdict |
| --- | --- | --- |
| `manatal` on `api.manatal.com/open/v1/career-page/manatal/jobs/` | 404 | HTML `Manatal` error page |
| `manatal` on `api.careers-page.com/open/v1/career-pages/manatal/job-posts` | 200 | JSON `items`, `total=40`, `page=1`, `pages=2` |

Recommendation: `manatal` with the corrected host and path. The corrected response returned 20 items in the first page and 40 total. The manifest envelope must change from `results` to `items`.

## Endpoint corrections

| Source | Manifest pattern | Live pattern verified 2026-11-27 |
| --- | --- | --- |
| `ats-smartrecruiters` | `/v1/companies/{slug}/jobs` | `/v1/companies/{slug}/postings` |
| `ats-teamtailor` | `https://career.teamtailor.com/widget/jobs/{slug}` | `https://{slug}.teamtailor.com/jobs.json` |
| `ats-trakstar` | `https://{slug}.hire.trakstar.com/api/v1/openings` | `https://{slug}.hire.trakstar.com/jobfeeds/{company}` (RSS) |
| `ats-workday` | `https://{slug}.wd5.myworkdayjobs.com/wday/cxs/{slug}/External/jobs` | `https://{tenant}.wd{dc}.myworkdayjobs.com/wday/cxs/{tenant}/{site}/jobs` |
| `ats-oracle` | `https://{slug}.fa.oraclecloud.com/...finder=findReqs;siteNumber=CX_45001,...` | Same finder plus `expand=requisitionList`; host may need `{tenant}.fa.{region}.oraclecloud.com` |
| `manatal` | `https://api.manatal.com/open/v1/career-page/{slug}/jobs/` | `https://api.careers-page.com/open/v1/career-pages/{slug}/job-posts` |
| `jobvite` | `https://jobs.jobvite.com/api/v2/job-feed/{slug}` | `https://app.jobvite.com/CompanyJobs/Xml.aspx?c={companyId}` or `https://jobs.jobvite.com/{slug}` |
| `icims` | JSON parser over `/jobs/search` | HTML listing parser over the same search page |
| `ats-ukg` | `https://recruiting.ultipro.com/{slug}/OpportunitySearch` | `https://recruiting.ultipro.com/{tenant}/JobBoard/{boardId}/` plus POST `LoadSearchResults` |
| `ats-rippling` | `https://ats.rippling.com/{slug}/jobs` | Route is correct for tenant boards; exclude `rippling`, which redirects to the vendor's own careers site |

## Sources with no public seed

- **`ats-recruiterflow`** — the seed is the `rf-api-key` credential. Public portal names do not authenticate the API. The unauthenticated request returns HTTP 400 `Please supply an API key`.
- **`ats-talentlyft`** — the public API shape is documented and the host answers, but no customer subdomain with real jobs was found. The only live response was TalentLyft's own `careers` tenant with one open-application placeholder.
- **`ats-ukg`** — a seed must include both a tenant code and a board UUID, and the list endpoint is a POST. The single `{slug}` field cannot represent a working board.

`ats-workday` is not in this list because `nvidia`, `salesforce`, `target`, and `verizon` were verified, but its manifest needs a tenant-plus-site tuple rather than one slug.

## Commands run

All probes were run on 2026-11-27 between 15:49 and 16:04 UTC. The first batch used `-m 20`; later batches used `-m 25` because a few vendor hosts were slow.

Source inventory command:

```bash
rg -l 'requires_slug' src/sources/manifests | sort
```

Result: 22 files, matching the 22 sections below.

Network check that first failed inside the sandbox:

```bash
curl -sS -m 20 -A 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36' -o /tmp/probe-ashby.json -w '%{http_code} %{size_download} %{content_type}\n' 'https://api.ashbyhq.com/posting-api/job-board/notion'
```

Result after network escalation: `200 3228362 application/json; charset=utf-8`.

GET probe template:

```bash
UA='Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36'
curl -sS -m 20 -A "$UA" \
  -H 'accept: application/json,text/html;q=0.9,*/*;q=0.8' \
  -o "/tmp/slugprobe/body/<source>_<slug>.body" \
  -w '%{http_code} %{size_download} %{content_type}' '<URL>'
```

POST probe template, used for Workday:

```bash
curl -sS -m 20 -A "$UA" -X POST \
  -H 'content-type: application/json' -H 'accept: application/json' \
  -d '{"appliedFacets":{},"limit":20,"offset":0,"searchText":""}' \
  -o "/tmp/slugprobe/body/<source>_<slug>.body" \
  -w '%{http_code} %{size_download} %{content_type}' '<URL>'
```

Redirect-following probes added `-L` to the same command. The batch loop used the following shape, with one `source`, `slug`, method, and URL per line in `/tmp/slugprobe/probes.tsv`:

```bash
while IFS=$'\t' read -r source slug method url; do
  safe=$(printf '%s_%s' "$source" "$slug" | tr -c 'A-Za-z0-9._-' '_')
  body="/tmp/slugprobe/body/$safe.body"
  curl -sS -m 20 -A "$UA" \
    -H 'accept: application/json,text/html;q=0.9,*/*;q=0.8' \
    -o "$body" -w '%{http_code} %{size_download} %{content_type}' "$url"
  printf '%s\t%s\t%s\t%s\n' "$source" "$slug" "$url" "$?"
done < /tmp/slugprobe/probes.tsv
```

JSON record counts and envelope checks were extracted with `python3` and `json.load`; HTML verdicts came from checking the body prefix for `<html`/`<!doctype`, the `<title>`, and job-link patterns. RSS/XML counts came from counting `<item>` and `<job>` elements. For example:

```bash
python3 -c 'import json; d=json.load(open("/tmp/slugprobe/body/ats-ashby_notion.body")); print(len(d["jobs"]))'
```

Result: `174`.

## Web sources consulted

These are secondary to the probes. They were used to locate public tenants or confirm that a vendor route exists.

- [Ashby public job posting API](https://developers.ashbyhq.com/docs/public-job-posting-api) — vendor documentation, accessed 2026-11-27.
- [Pinpoint jobs JSON endpoint](https://developers.pinpointhq.com/docs/jobs-json-endpoint) — vendor documentation, accessed 2026-11-27.
- [Recruitee offers API](https://docs.recruitee.com/reference/offers) — vendor documentation, accessed 2026-11-27.
- [SmartRecruiters list postings](https://developers.smartrecruiters.com/reference/v1listpostings) — vendor documentation, accessed 2026-11-27.
- [Manatal career-page job posts](https://developers.manatal.com/reference/get_career_page_job_posts_open_v1_career_pages__client_slug__job_posts_get) — vendor documentation, accessed 2026-11-27.
- [TalentLyft public API jobs](https://developers.talentlyft.com/public-api-reference/jobs) — vendor documentation, search result crawled about 2026-03, accessed 2026-11-27.
- [Jobvite Job Feed API](https://careers.jobvite.com/careersite/job_feed_api.html) — vendor documentation, search result crawled about 2026-11-06, accessed 2026-11-27.
- [Jobvite job-listings guide](https://cleanjobdata.com/articles/how-to-scrape-jobvite-job-listings) — published about 2026-11-06, accessed 2026-11-27.
- [Trakstar public RSS feed notes](https://jobspipe.dev/sources/trakstar) — search result crawled about 2026-09, accessed 2026-11-27.
- [Whatfix careers listing](https://www.landed.jobs/companies/whatfix) — says `whatfix101.hire.trakstar.com/jobs`, search result crawled 2026-11-27.
- [UKG job board guide](https://www.hirebase.org/ats/ukg) — search result crawled about 2026-11-23, accessed 2026-11-27.
- [UKG tenant and board example](https://digital-health-jobs.com/companies/fierce-healthcare/job_posts/registration-page-no-job-details) — search result published about 2026-04, accessed 2026-11-27.
- [Verizon Workday board](https://airecruiter.co/employers/verizon) — says `verizon.wd12.myworkdayjobs.com`, board observed 2026-11-02, search result crawled 2026-11-27.
- [Verizon careers site](https://jobseye.in/jobs/companies/verizon) — says `verizon.wd12.myworkdayjobs.com/verizon-careers`, published 2026-11-27.
- [Walmart Workday job URL](https://www.fultoncountyoh.com/DocumentCenter/View/21144/03162026-Job-Leads) — published 2026-03-16, uses `walmart.wd5.myworkdayjobs.com/en-US/WalmartExternal/...`.

## Limits

- Probe results are a point-in-time snapshot. Boards change between requests, and counts are not guaranteed to match a later harvest.
- Some boards cap the first response (Gem at 300, Workday at 2,000 total, Rippling at its page size). The tables identify page counts separately from totals.
- HTML verdicts confirm the page is a job board, not that every listing can be parsed by the current manifest parser.
- No WAF challenge was attempted, no credentials were used, and no manifest or fixture was modified.

## Related audit

The prior candidate list is in [ats-slug-candidates.md](ats-slug-candidates.md). This report supersedes its probe status for the 22 `requires_slug` sources because those candidates were tested live on 2026-11-27.
