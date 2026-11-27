# Source transport taxonomy — 2026-11-27

Scope: all 111 TOML manifests under `src/sources/manifests/` on branch `oculus`, matched one-to-one with the 111 behavior records in `documentation/scrapers/`. This is a structural audit of transport, parser, and fetch shape. No live HTTP probe was run for this document, and no external web search was used. Values in the `health (manifest)` column are copied from each manifest. Live-validation observations quoted below keep the dates recorded in the scraper docs.

The corpus has four populated families: `feeds_api` (58), `ats` (26), `keyed` (14), and `html_static` (13). `html_rendered/` contains only its family README. Every one of the 111 manifests declares `fetcher = "plain"`, `requires_js = false`, and `profile = "none"`. Thirty declare a `slugs` seed, nineteen declare `credentials`, and fifteen declare a pagination mode other than `none`. Request shape follows the parser split: 77 manifests use `json` (six of them extract embedded JSON from HTML), 26 use `rss`, and 8 use `html`. Twenty-eight manifests put `{slug}` in the request URL, 23 put `{query:}`, and six put `{key:}`; the other credential sources carry the key in a header or body. Three manifests declare POST (`ats-workday`, `mycareersfuture`, `exa`); the rest are GET.

## How the classes were assigned

| Class | Rule used for this audit | Total |
| --- | --- | ---: |
| JSON/RSS API | A direct HTTP JSON or RSS/Atom payload with no tenant slug and no credential. Includes public search APIs, Firebase fan-out, SEEK v5, and the Workable widget JSON. | 53 |
| static HTML | The HTTP response is HTML that already carries the records, either in DOM markup or in an embedded JSON blob (`__NEXT_DATA__` or a Stash variable). A plain GET is sufficient. | 13 |
| JS-rendered HTML needing a browser | Records appear only after client-side JavaScript execution. | 0 |
| keyed API | A named API key, token, or client credential is required before the request can be built. | 19 |
| slug-based ATS | A `{slug}` selects a tenant or board. Includes public ATS JSON, widget, and board-page routes. | 26 |

The classes are transport classes, so they deliberately cut across manifest families. Five cross-family mismatches matter: `jazzhr` is in `ats` but is a keyed API, `huggingfacejobs` is in `html_static` but is a Workable JSON API, `ats-rippling` is in `html_static` but is a slug-based ATS board, `ycjobs` is in `keyed` but has no credential and only parses an HTML fallback, and `ukvisajobs` is in `feeds_api` but serves an HTML page with embedded JSON.

`parser = "json"` appears in the table as `json (embedded)` for the six sources that read JSON out of an HTML host: `ats-rippling`, `hiringcafe`, `jobindex`, `reed`, `ukvisajobs`, and `wellfound`. Their transport is still static HTML because plain HTTP returns the embedded payload.

## Redeemability legend

| Value | Meaning |
| --- | --- |
| `yes` | Public, unauthenticated data is reachable with the current request shape. This is a structural judgment, not a live-status claim. |
| `yes*` | Public data exists, but the current route, seed, or parser needs repair or could not be verified. All placeholder ATS boards fall here because a real tenant slug is required. |
| `key` | Credential-only. Usable with credentials, but not redeemable from public data. Some keys are self-service. |
| `no` | No public endpoint, dead product, wrong content, or challenge-only. |
| `?` | Insufficient evidence to classify as either redeemable or non-redeemable. |

## Source table

| id | family | transport class | requires_js | parser | health (manifest) | redeemable |
| --- | --- | --- | --- | --- | --- | --- |
| `4dayweek` | feeds_api | JSON/RSS API | false | json | working | `yes` |
| `academiccareers` | feeds_api | JSON/RSS API | false | rss | working | `yes` |
| `androidjobs` | feeds_api | JSON/RSS API | false | rss | working | `no` |
| `arbeitnow` | feeds_api | JSON/RSS API | false | json | working | `yes` |
| `berlinstartupjobs` | feeds_api | JSON/RSS API | false | rss | working | `yes` |
| `canadajobbank` | feeds_api | JSON/RSS API | false | json | working | `yes*` |
| `careeronestop` | feeds_api | JSON/RSS API | false | json | working | `yes*` |
| `conservationjobs` | feeds_api | JSON/RSS API | false | rss | working | `yes` |
| `coroflot` | feeds_api | JSON/RSS API | false | rss | working | `yes` |
| `crunchboard` | feeds_api | JSON/RSS API | false | rss | working | `no` |
| `cryptocurrencyjobs` | feeds_api | JSON/RSS API | false | rss | working | `yes` |
| `cryptojobslist` | feeds_api | JSON/RSS API | false | rss | working | `yes` |
| `devopsjobs` | feeds_api | JSON/RSS API | false | rss | working | `yes` |
| `djinni` | feeds_api | JSON/RSS API | false | rss | stale | `?` |
| `drupaljobs` | feeds_api | JSON/RSS API | false | rss | stale | `no` |
| `echojobs` | feeds_api | JSON/RSS API | false | json | working | `?` |
| `ecojobs` | feeds_api | JSON/RSS API | false | rss | working | `yes` |
| `eurojobs` | feeds_api | JSON/RSS API | false | rss | working | `no` |
| `fossjobs` | feeds_api | JSON/RSS API | false | rss | working | `yes` |
| `freelancercom` | feeds_api | JSON/RSS API | false | json | working | `yes*` |
| `getonboard` | feeds_api | JSON/RSS API | false | json | working | `yes*` |
| `golangjobs` | feeds_api | JSON/RSS API | false | rss | working | `yes` |
| `hackernews` | feeds_api | JSON/RSS API | false | json | working | `yes*` |
| `hasjob` | feeds_api | JSON/RSS API | false | rss | working | `yes` |
| `headhunter` | feeds_api | JSON/RSS API | false | json | blocked | `yes*` |
| `higheredjobs` | feeds_api | JSON/RSS API | false | rss | working | `yes` |
| `himalayas` | feeds_api | JSON/RSS API | false | json | working | `yes` |
| `hnalgolia` | feeds_api | JSON/RSS API | false | json | working | `yes` |
| `huggingfacejobs` | html_static | JSON/RSS API | false | json | working | `yes` |
| `icrunchdata` | feeds_api | JSON/RSS API | false | rss | blocked | `no` |
| `jobdataapi` | feeds_api | JSON/RSS API | false | json | working | `yes*` |
| `jobicy` | feeds_api | JSON/RSS API | false | json | working | `yes` |
| `jobsch` | feeds_api | JSON/RSS API | false | json | working | `?` |
| `jobsdb` | feeds_api | JSON/RSS API | false | json | working | `yes` |
| `jobsinjapan` | feeds_api | JSON/RSS API | false | rss | working | `no` |
| `jobspresso` | feeds_api | JSON/RSS API | false | rss | working | `yes` |
| `jobstreet` | feeds_api | JSON/RSS API | false | json | working | `yes` |
| `joinrise` | feeds_api | JSON/RSS API | false | json | working | `yes*` |
| `landingjobs` | feeds_api | JSON/RSS API | false | json | working | `yes*` |
| `mycareersfuture` | feeds_api | JSON/RSS API | false | json | working | `yes` |
| `nofluffjobs` | feeds_api | JSON/RSS API | false | json | working | `yes*` |
| `pythonjobs` | feeds_api | JSON/RSS API | false | rss | working | `yes` |
| `railsjobs` | feeds_api | JSON/RSS API | false | rss | working | `yes` |
| `realworkfromanywhere` | feeds_api | JSON/RSS API | false | rss | working | `yes` |
| `remotefirstjobs` | feeds_api | JSON/RSS API | false | rss | working | `no` |
| `remoteok` | feeds_api | JSON/RSS API | false | json | working | `yes` |
| `remotive` | feeds_api | JSON/RSS API | false | json | working | `yes` |
| `snagajob` | feeds_api | JSON/RSS API | false | json | blocked | `no` |
| `startupjobs` | feeds_api | JSON/RSS API | false | json | blocked | `no` |
| `themuse` | feeds_api | JSON/RSS API | false | json | working | `yes*` |
| `weworkremotely` | feeds_api | JSON/RSS API | false | rss | working | `yes` |
| `wordpressjobs` | feeds_api | JSON/RSS API | false | rss | working | `yes` |
| `workingnomads` | feeds_api | JSON/RSS API | false | json | working | `yes` |
| `aijobs` | html_static | static HTML | false | html | working | `yes` |
| `builtin` | html_static | static HTML | false | html | working | `yes` |
| `functionalworks` | html_static | static HTML | false | html | working | `yes` |
| `gunio` | html_static | static HTML | false | html | working | `yes` |
| `hiringcafe` | html_static | static HTML | false | json (embedded) | working | `yes` |
| `internshala` | html_static | static HTML | false | html | working | `yes*` |
| `jobindex` | html_static | static HTML | false | json (embedded) | working | `yes*` |
| `reed` | html_static | static HTML | false | json (embedded) | working | `yes` |
| `simplyhired` | html_static | static HTML | false | html | working | `yes*` |
| `ukvisajobs` | feeds_api | static HTML | false | json (embedded) | working | `yes*` |
| `wellfound` | html_static | static HTML | false | json (embedded) | working | `yes*` |
| `wuzzuf` | html_static | static HTML | false | html | working | `no` |
| `ycjobs` | keyed | static HTML | false | html | blocked | `no` |
| `adp` | ats | slug-based ATS | false | json | blocked | `no` |
| `ats-ashby` | ats | slug-based ATS | false | json | requires_slug | `yes*` |
| `ats-gem` | ats | slug-based ATS | false | json | requires_slug | `yes*` |
| `ats-oracle` | ats | slug-based ATS | false | json | requires_slug | `yes*` |
| `ats-phenom` | ats | slug-based ATS | false | json | stale | `yes*` |
| `ats-pinpoint` | ats | slug-based ATS | false | json | requires_slug | `yes*` |
| `ats-recruitee` | ats | slug-based ATS | false | json | requires_slug | `yes*` |
| `ats-recruiterflow` | ats | slug-based ATS | false | json | requires_slug | `no` |
| `ats-rippling` | html_static | slug-based ATS | false | json (embedded) | requires_slug | `yes*` |
| `ats-smartrecruiters` | ats | slug-based ATS | false | json | requires_slug | `yes*` |
| `ats-successfactors` | ats | slug-based ATS | false | json | stale | `no` |
| `ats-talentlyft` | ats | slug-based ATS | false | json | requires_slug | `?` |
| `ats-teamtailor` | ats | slug-based ATS | false | json | requires_slug | `yes*` |
| `ats-trakstar` | ats | slug-based ATS | false | json | requires_slug | `yes*` |
| `ats-ukg` | ats | slug-based ATS | false | json | requires_slug | `yes*` |
| `ats-workable` | ats | slug-based ATS | false | json | requires_slug | `yes*` |
| `ats-workday` | ats | slug-based ATS | false | json | requires_slug | `yes*` |
| `bamboohr` | ats | slug-based ATS | false | json | requires_slug | `yes*` |
| `breezyhr` | ats | slug-based ATS | false | json | requires_slug | `yes*` |
| `greenhouse` | ats | slug-based ATS | false | json | requires_slug | `yes*` |
| `icims` | ats | slug-based ATS | false | json | requires_slug | `yes*` |
| `jobscore` | ats | slug-based ATS | false | json | requires_slug | `yes*` |
| `jobvite` | ats | slug-based ATS | false | json | requires_slug | `yes*` |
| `jobylon` | ats | slug-based ATS | false | json | requires_slug | `yes*` |
| `loxo` | ats | slug-based ATS | false | json | blocked | `no` |
| `manatal` | ats | slug-based ATS | false | json | requires_slug | `yes*` |
| `adzuna` | keyed | keyed API | false | json | keyed | `key` |
| `authenticjobs` | keyed | keyed API | false | json | keyed | `key` |
| `careerjet` | keyed | keyed API | false | json | keyed | `key` |
| `comeet` | keyed | keyed API | false | json | keyed | `key` |
| `crelate` | keyed | keyed API | false | json | keyed | `key` |
| `deel` | feeds_api | keyed API | false | json | keyed | `key` |
| `dice` | feeds_api | keyed API | false | json | keyed | `key` |
| `exa` | keyed | keyed API | false | json | keyed | `key` |
| `findwork` | keyed | keyed API | false | json | keyed | `key` |
| `fountain` | feeds_api | keyed API | false | json | keyed | `key` |
| `freshteam` | keyed | keyed API | false | json | keyed | `key` |
| `hiringthing` | keyed | keyed API | false | json | keyed | `key` |
| `infojobs` | keyed | keyed API | false | json | keyed | `key` |
| `ismartrecruit` | keyed | keyed API | false | json | keyed | `key` |
| `jazzhr` | ats | keyed API | false | json | keyed | `key` |
| `jobtechdev` | keyed | keyed API | false | json | keyed | `key` |
| `talroo` | feeds_api | keyed API | false | json | keyed | `key` |
| `usajobs` | keyed | keyed API | false | json | keyed | `key` |
| `web3career` | feeds_api | keyed API | false | json | keyed | `key` |

## Counts by transport class

| Transport class | Total | working | requires_slug | keyed | blocked | stale | public yes | public yes* | credential key | no | ? |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| JSON/RSS API | 53 | 47 | 0 | 0 | 4 | 2 | 30 | 11 | 0 | 9 | 3 |
| static HTML | 13 | 12 | 0 | 0 | 1 | 0 | 6 | 5 | 0 | 2 | 0 |
| slug-based ATS | 26 | 0 | 22 | 0 | 2 | 2 | 0 | 21 | 0 | 4 | 1 |
| keyed API | 19 | 0 | 0 | 19 | 0 | 0 | 0 | 0 | 19 | 0 | 0 |

Every `working` source is either JSON/RSS or static HTML. No slug-based ATS or keyed API has `health = "working"`: the ATS family is dominated by `requires_slug`, and every keyed source is declared `keyed` before a request can be built.

## Counts by family

| Family | Total | working | requires_slug | keyed | blocked | stale | Class mix |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| `feeds_api` | 58 | 47 | 0 | 5 | 4 | 2 | JSON/RSS API 52, static HTML 1, keyed API 5 |
| `ats` | 26 | 0 | 21 | 1 | 2 | 2 | slug-based ATS 25, keyed API 1 |
| `html_static` | 13 | 12 | 1 | 0 | 0 | 0 | JSON/RSS API 1, static HTML 11, slug-based ATS 1 |
| `keyed` | 14 | 0 | 0 | 13 | 1 | 0 | static HTML 1, keyed API 13 |

The headline is that `feeds_api` carries most of the working inventory (47 of 59), while `ats` has no working source because 21 of 26 boards are waiting on a tenant slug. `html_static` is 12 of 13 working, but only six of those twelve are plain `yes`; five are `yes*` and one is `no`.

## Manifest health versus structural judgment

Manifest health totals: `working` 59, `requires_slug` 22, `keyed` 19, `blocked` 7, `stale` 4. Those are declarations, not measured live results.

- Of the 59 `working` declarations, 51 are `yes` or `yes*`. Eight are `no` or `?`: `androidjobs`, `crunchboard`, `echojobs`, `eurojobs`, `jobsch`, `jobsinjapan`, `remotefirstjobs`, and `wuzzuf`.
- Twenty-two sources are structurally public but not declared working. Twenty are seed-blocked public boards (all `requires_slug` sources except `ats-recruiterflow` and `ats-talentlyft`), joined by `ats-phenom` (real public boards, different route) and `headhunter` (public API, 403 from the test environment).
- The largest systematic gap is not transport failure. It is seed coverage: 20 of the 22 `requires_slug` sources have a public route but ship `example-co` or equivalent.
- Manifest health is inconsistent with the docs for `androidjobs`, `eurojobs`, and `jobsinjapan`: all three remain `working` while the recorded probes returned 404, 404, and a non-job blog feed. Treat those `working` labels as replay-contract labels, not live evidence.

Totals across all 111 sources: 73 are public-data redeemable at the structural level (36 `yes`, 37 `yes*`) and 38 are not (19 credential-only, 15 `no`, 4 `?`). The `yes*` group is large because twenty ATS or ATS-like boards have a public route but ship a placeholder seed.

## Not redeemable from public data

### No public data, dead product, wrong content, or challenge-only

| id | Class | One-line reason |
| --- | --- | --- |
| `adp` | slug-based ATS | Workforce Now `cid` is a tenant GUID; no public `cid` was found and the placeholder tenant returns 404. |
| `ats-recruiterflow` | slug-based ATS | The seed is sent as the `RF-Api-Key`; no public board slug grants API access. |
| `ats-successfactors` | slug-based ATS | OData v2 is tenant role-gated; public postings live on Career Site Builder HTML instead. |
| `loxo` | slug-based ATS | HTTP 403 and no public tenant/API reference was found. |
| `androidjobs` | JSON/RSS API | `/jobs.rss` returns HTTP 404; no replacement feed was verified. |
| `crunchboard` | JSON/RSS API | `/jobs.rss` returns HTTP 403 behind a challenge; no replacement feed was verified. |
| `drupaljobs` | JSON/RSS API | jobs.drupal.org shut down in September 2026; the URL serves an HTML shutdown notice. |
| `eurojobs` | JSON/RSS API | `/rss` returns HTTP 404; no replacement feed was verified. |
| `icrunchdata` | JSON/RSS API | Cloudflare challenge on `/rss/`; health is already `blocked`. |
| `jobsinjapan` | JSON/RSS API | `/feed/` is a blog feed with no job company or job-type fields; no job feed was verified. |
| `remotefirstjobs` | JSON/RSS API | `/feed/` returns HTTP 403 behind a challenge; no replacement feed was verified. |
| `snagajob` | JSON/RSS API | Cloudflare returns HTTP 403 even with a browser User-Agent; challenge-only. |
| `startupjobs` | JSON/RSS API | Cloudflare challenge is unresolved; live probes return HTTP 403. |
| `wuzzuf` | static HTML | The static search page returns HTTP 403 behind a challenge; no alternate markup was verified. |
| `ycjobs` | static HTML | Only the GET HTML fallback is parsed; the working path is an unported derived-key POST API. |

`authenticjobs` also has a recorded Cloudflare 403, but its primary blocker is the API key, so it is counted in the credential-only group.

### Credential-only

| id | Family | Declared credential(s) | One-line reason |
| --- | --- | --- | --- |
| `adzuna` | `keyed` | adzuna_app_id, adzuna_app_key | `adzuna_app_id` + `adzuna_app_key`; developer registration, but still credential-only. |
| `authenticjobs` | `keyed` | AUTHENTICJOBS_API_KEY | `AUTHENTICJOBS_API_KEY`; a Cloudflare 403 also remains a deployment risk. |
| `careerjet` | `keyed` | careerjet_affid | `careerjet_affid`; affiliate ID issued by Careerjet. |
| `comeet` | `keyed` | COMEET_API_TOKEN | Company UID plus `COMEET_API_TOKEN`; both values come from the employer board settings. |
| `crelate` | `keyed` | CRELATE_API_KEY | `CRELATE_API_KEY` in `X-Api-Key`; a portal slug alone is insufficient. |
| `exa` | `keyed` | EXA_API_KEY | `EXA_API_KEY`; a generic web search API, not a job board. |
| `findwork` | `keyed` | findwork_auth | `findwork_auth` authorization token. |
| `freshteam` | `keyed` | FRESHTEAM_API_TOKEN | Tenant subdomain plus `FRESHTEAM_API_TOKEN` bearer token. |
| `hiringthing` | `keyed` | HIRINGTHING_API_TOKEN | `HIRINGTHING_API_TOKEN` bearer token. |
| `infojobs` | `keyed` | INFOJOBS_CLIENT_ID, INFOJOBS_CLIENT_SECRET, INFOJOBS_BASIC_TOKEN | `INFOJOBS_CLIENT_ID` + `INFOJOBS_CLIENT_SECRET` + Basic token. |
| `ismartrecruit` | `keyed` | ISMARTRECRUIT_API_KEY | `ISMARTRECRUIT_API_KEY` query parameter. |
| `jobtechdev` | `keyed` | JOBTECHDEV_API_KEY | `JOBTECHDEV_API_KEY` in the `api-key` header. |
| `usajobs` | `keyed` | USAJOBS_API_KEY, USAJOBS_EMAIL | `USAJOBS_API_KEY` plus a contact email in the User-Agent. |
| `jazzhr` | `ats` | JAZZHR_API_KEY | `JAZZHR_API_KEY` query parameter. |
| `deel` | `feeds_api` | DEEL_API_TOKEN | `DEEL_API_TOKEN` bearer token. |
| `dice` | `feeds_api` | SCRAPPY_DICE_API_KEY | `SCRAPPY_DICE_API_KEY` in `x-api-key`. |
| `fountain` | `feeds_api` | FOUNTAIN_API_KEY | `FOUNTAIN_API_KEY`. |
| `talroo` | `feeds_api` | TALROO_PUBLISHER_ID, TALROO_PUBLISHER_PASS | `TALROO_PUBLISHER_ID` + `TALROO_PUBLISHER_PASS`; publisher account required. |
| `web3career` | `feeds_api` | WEB3CAREER_API_TOKEN | `WEB3CAREER_API_TOKEN`; the assumed v1 path also returns HTTP 404. |

This group is usable in principle, but not from public data. Some keys are obtainable through developer registration and some are vendor-, affiliate-, or account-issued; the manifest does not distinguish the two. `jazzhr` needs an API key with no slug at all, and `authenticjobs` carries a Cloudflare 403 on top of its key requirement.

### Unresolved, not counted as public-data redeemable

| id | Class | One-line reason |
| --- | --- | --- |
| `djinni` | JSON/RSS API | HTTP 200 with a zero-byte RSS body; the full feed is unverified from the test network. |
| `echojobs` | JSON/RSS API | HTTP 429 rate/environment response; the endpoint could not be confirmed. |
| `jobsch` | JSON/RSS API | The JSON route returns HTTP 422; public search is server-rendered and corrected JSON parameters are not public. |
| `ats-talentlyft` | slug-based ATS | No public API docs or verified tenant slug; the placeholder returns HTTP 404. |

## Browser and JavaScript work versus plain HTTP

No source in this corpus needs a browser to render its records. Every manifest declares a plain fetcher with `requires_js = false`, and the `html_rendered` family has no manifests. The six embedded-payload pages are the closest cases, and all six ship complete data over a plain GET:

| Source | Embedded payload | Why a plain GET is sufficient |
| --- | --- | --- |
| `hiringcafe` | `__NEXT_DATA__` → `props.pageProps.ssrHits` | The doc describes server-side hits in the homepage HTML. |
| `wellfound` | `__NEXT_DATA__` → Apollo cache | The 2026-11-27 targeted probe returned 43 records from the cache path. |
| `reed` | `__NEXT_DATA__` → `props.pageProps.searchResults.jobs` | The doc describes an HTML page with the JSON envelope inline. |
| `ukvisajobs` | `__NEXT_DATA__` → `props.pageProps.result.experienced_jobs` | One GET returns the public board; the doc explicitly says no browser profile is used. |
| `jobindex` | Stash variable → `searchResponse.results` | The search page embeds the Stash payload; the 2026-11-27 probe returned 204, not a JS-only page. |
| `ats-rippling` | `__NEXT_DATA__` → `dehydratedState` | The board page carries the item array inline; the only issue is the placeholder slug. |

The selector-based static HTML sources are `aijobs`, `builtin`, `functionalworks`, `gunio`, `internshala`, `simplyhired`, `wuzzuf`, and `ycjobs`. The docs for `aijobs`, `gunio`, `simplyhired`, and `wuzzuf` explicitly say no browser or emulation profile is used; `ukvisajobs` says the same for its embedded payload. `functionalworks` switched from a GraphQL endpoint to server-rendered cards.

What does look browser-adjacent is anti-bot handling, which is a different problem from rendering:

- Challenge-gated sources: `snagajob` and `startupjobs` record Cloudflare 403s, `icrunchdata` records a Cloudflare challenge, `crunchboard` and `remotefirstjobs` record 403s, and `wuzzuf` records a 403 on its static search page. `authenticjobs` adds a Cloudflare 403 on top of its API key. The docs for `icrunchdata` and `snagajob` mention browser emulation, but as a challenge bypass, not as a rendering requirement.
- Browser-shaped headers: `builtin`, `functionalworks`, `jobindex`, `simplyhired`, `ats-rippling`, `ukvisajobs`, and `wellfound` send a `Mozilla/5.0` User-Agent. The docs record 204, 406, or challenged variants being fixed by that header. Header shaping is still plain HTTP.
- Next.js or Apollo front ends with server-side data: `hiringcafe`, `wellfound`, `reed`, `ukvisajobs`, and `ats-rippling`. Their HTML contains the payload, so a renderer would add no records.

If future work adds a browser fetcher, the justification should be a documented client-only page or a challenge that survives header shaping. None of the 111 behavior records currently shows a client-only job payload. The manifest schema also has no way to express one today: `fetcher` is `plain` for all 111, and `profile` is `none` for all 111.

## Method and limitations

- Manifest fields were read directly from the 111 TOML files. Behavior records were read from the matching `documentation/scrapers/<id>.md` files. The only file written by this audit is this document.
- No live requests were made. The 2026-11-27 observations quoted above come from the scraper docs; they are not re-probed here. The environment used for those docs also reports DNS, TCP, 403, and 429 failures for some public hosts, so several `blocked` and `?` labels may be environmental rather than source-side.
- `yes*` assumes a real public tenant slug or a corrected public route. It does not assert that the current placeholder seed can produce records.
- Transport class is assigned from the manifest request shape plus the behavior record, not from an HTTP response captured for this audit. A source that changes from SSR to client-only rendering would move to the browser class on the next audit.
