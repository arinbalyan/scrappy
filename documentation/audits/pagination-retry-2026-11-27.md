# Pagination, retry, concurrency, and ban-avoidance audit

Research date 2026-11-27 (Asia/Kolkata). Repository `scrappy`, branch `oculus`.
The canonical snapshot is commit `9310816ebfdd0670a240ff5c9a89f0c6ec7699fc`
(`feat: align engine with spec and add live harvest audit`). The old baseline is
branch `main`, which is a Go implementation, read with `git show main:<path>`.
Only this report was created. No Rust, manifest, fixture, or script file was
modified.

The working tree was being edited by another workstream while this audit ran.
At 2026-11-27 21:20 IST an untracked `src/engine/pagination.rs` and an
uncommitted pagination loop in `src/engine/pipeline.rs` appeared. They are not
part of commit `9310816`. The main report describes the committed branch. The
draft is documented separately in Appendix A because it changes several
findings if it lands.

## Summary

- The committed engine performs one manifest request per selected source. The
  only repetitions are transport retries and redirect hops.
  `Manifest.pagination` is deserialized, required to be non-empty, and never
  read by the pipeline or adapter. There is no page loop, offset loop, cursor
  follow, or fanout expansion.
- Fifteen of 111 manifests declare something other than `none`. Fourteen of
  those fifteen put a `{query:...}`, `{slug}`, or `{key:...}` placeholder in
  the request. The CLI supplies no substitution values because
  `config_values` returns an empty map, so those fourteen fail in
  `build_request` before the first HTTP call. The remaining one,
  `hackernews`, fetches a JSON array of story IDs once and parses zero jobs
  from it.
- Retry is present but only sees transport errors. `PlainFetcher` returns
  `Ok(Response)` for HTTP 429 and 5xx, so the retry loop never retries a
  status failure. `parse_retry_after` and `Failure::Permanent` are used only by
  unit tests.
- Concurrency limits are per source. A source acquires one governor permit for
  its entire run, and `run` iterates sources sequentially, so
  `max_concurrency` has no observable effect. There is no global request
  concurrency bound.
- Ban avoidance is thinner than on old `main`. The plain path has no proxy,
  user-agent rotation, cookie jar, or browser fallback, and it builds a new
  `reqwest::Client` for every request. Pacing is better centralized, but
  session identity and connection reuse are gone.
- The exact-match robots check silently skips 44 of 111 manifests whose policy
  text begins with `allow` but is not the single word `allow`.

## Method and limits

- Read current `src/engine/pipeline.rs`, `src/engine/retry.rs`,
  `src/engine/governor.rs`, `src/engine/limits.rs`,
  `src/engine/breaker.rs`, `src/engine/challenge.rs`,
  `src/sources/manifest.rs`, `src/sources/adapter.rs`,
  `src/fetching/plain.rs`, `src/main.rs`, and every manifest under
  `src/sources/manifests/`.
- Read the old pagination and request code with `git show main:<path>` for
  the relevant scrapers, `internal/util/http.go`,
  `internal/util/scrapeguard.go`, `pkg/scrappy/engine.go`, and the old rate
  limiting documentation.
- Counted manifest fields mechanically. There are 111 manifests with a
  `pagination` field, 96 declare `none`, and 15 declare another value.
- No public host was reachable from the sandbox. A local CLI probe stopped at
  DNS resolution, so this report makes no live HTTP or record-count claims.
  All current-behavior statements come from source reading and the existing
  offline tests.

## Old main baseline

Old `main` is a Go codebase with one scraper per source. Pagination was
implemented inside each scraper with hardcoded page sizes, caps, and stop
conditions. There was no manifest pagination field.

### How old main paginated

At least 24 scraper files contain explicit page or offset loops. Page-size
constants appear in more than two dozen files. The loops share a pattern but
not a helper. Typical behavior was a sequential request per page, a check for
an empty or short page, a hard page cap in many scrapers, a total-count check
in some, and a sleep or ticker between requests.

| Old source | Strategy | Bound and page size | Stop conditions | Pacing |
| --- | --- | --- | --- | --- |
| Arbeitnow | page number, starting at 1 | 3 pages | empty page or enough jobs | none beyond the client |
| Himalayas | limit and offset | 10 pages, 20 per page | empty page or enough jobs | none between pages |
| Adzuna | page number | 20 pages, 50 per page | short page or enough jobs | 500 ms between pages |
| CareerJet | page number | 10 pages, 50 per page | short page or API page count reached | 350 ms ticker |
| SimplyHired | page number | `wanted/20 + 3` pages | empty page or enough jobs | 200 to 600 ms jitter |
| Internshala | page-number path | 8 pages | empty page or enough jobs | none between pages |
| Built In | page number, starting at 0 | 6 pages | empty page or enough jobs | none between pages |
| SmartRecruiters | offset, limit 100 | until a short page, per slug | empty or short page | none between pages |
| FindWork | cursor from a `next` field | until the field is absent | absent cursor or enough jobs | 200 to 500 ms jitter |
| Reed | page number `pageno` | until enough jobs, no hard cap | empty page | 200 to 500 ms jitter plus retry backoff |
| JobsDB and Jobstreet | page number | 10 pages, 30 per page | total count reached or short page | 200 to 500 ms jitter |
| Hacker News | fanout over item IDs | 45 IDs, batches of 10 | enough jobs or 30 misses with a search term | client pacing |
| TalentLyft | page 1 only | single request | not applicable | none |
| Greenhouse | no pagination | one request per company seed | short page per board | 5 concurrent board workers |

Representative hard caps show how source-specific the old behavior was.
Dice used 50 pages of 20. CareerBuilder used up to 100 pages of 25. Workday
used 20 per page, SuccessFactors 20 per `$skip` step, Taleo 25, and Phenom
25. FindWork had no page cap and relied on cursor exhaustion. Reed had no page
cap and relied on an empty page.

A separate group followed cursors or next links. Indeed used a GraphQL
`nextCursor` and stopped when the cursor was empty or repeated. ZipRecruiter
used a cursor loop. JobDataAPI followed a `next` URL with a five-page cap.
FindWork followed a `next` URL with no page cap. These loops deduplicated by
job ID or URL because pages could overlap.

The old per-host token bucket in `internal/rate/rate.go` is referenced only by
`tests/rate/rate_test.go`. The engine and scrapers did not use it. Real
spacing came from per-scraper tickers, jitter sleeps, and the HTTP transport
retry delay.

### How old main retried and backed off

Retries lived in a custom `http.RoundTripper` shared by a scraper's HTTP
client. Retry counts were opt-in per scraper. Many clients set only a timeout
and therefore made one attempt. Others set two or three retries.

| Behavior | Old main |
| --- | --- |
| Attempts | `Retries + 1`, caller supplied per client |
| Methods | GET, HEAD, and OPTIONS only |
| Retryable statuses | 429 and 5xx |
| Non-retryable statuses | 401, 403, 406, and other 4xx |
| Permanent transport errors | DNS NXDOMAIN, TLS or certificate failure, unreachable network or no route |
| Repeated 429 | second consecutive 429 failed the request immediately as permanent |
| Base delay | 300 ms |
| Max delay | 4 s |
| Backoff | `300 ms * 2^attempt`, capped at 4 s |
| Jitter | random 0 to 119 ms, plus 300 ms for 429, 80 ms for 5xx, and 600 ms for 401, 403, or 406 |
| Retry-After | not parsed or honored |
| Client reuse | one client and cookie jar per scraper, with keep-alive and idle connection limits |

The 600 ms bonus for 401, 403, and 406 was unreachable in practice because
those statuses were returned before the retry delay was computed.

### How old main limited concurrency

- A global channel semaphore was created once per run. Its size was 3 for a
  memory cap of 256 MB or less, 5 for 512 MB or less, 8 for 1 GB or less, and
  12 above that. Without a memory cap it used `max-rps` clamped to 2 through
  16, with a default of 8.
- A per-site channel semaphore was created only for sites present in
  `--site-rps`. Its capacity was the site's RPS value clamped to 1 through 8.
- The engine launched one goroutine per site. Each goroutine held a global slot
  and, when configured, a site slot. Search terms and locations were then
  processed sequentially inside that goroutine.
- Greenhouse had a separate five-worker pool over company seeds.
- Memory pressure was checked before launch. Above 90 percent of the cap the
  engine forced GC and waited. It resumed below 75 percent.
- The global and site semaphores bounded concurrent site goroutines. They did
  not bound individual page requests inside a scraper beyond that.

### How old main avoided bans

- Three browser user agents were rotated randomly or every N requests.
- Browser-like headers were added for non-API paths, including
  `Accept-Language`, `Sec-Fetch-*`, `Upgrade-Insecure-Requests`, and
  `Sec-CH-UA-*` values matched to the user agent.
- A cookie jar was kept per client and cleared every 200 requests by default.
- Proxies were health-checked with a 500 ms TCP dial, filtered, rotated
  round-robin, and tied to request-count or sticky-window thresholds.
- Per-scraper tickers and jitter sleeps paced pages and paginated requests.
- Challenge detection looked for DataDome, Cloudflare, and JavaScript-required
  markers.
- Google, Monster, Indeed, and LinkedIn could fall back to a Playwright
  browser path with a stealth plugin. The browser path could reuse cookies
  from a block.
- An adaptive RPS suggestion was recorded after each site run. It was not
  applied automatically.
- Site failures could fail open with a reason such as challenge, rate limit,
  access denied, timeout, or unknown.

## Current engine behavior at commit 9310816

### One request per source

`pipeline::run` selects manifests and iterates them in a plain sequential
`for` loop (`src/engine/pipeline.rs:198`). For each manifest it checks robots,
registers a governor budget, acquires one permit, checks the breaker, and calls
`run_source` (`src/engine/pipeline.rs:211-236`).

`run_source` contains one `build_request` call, one `retry_async` call around
`fetcher.fetch`, one `parse_response` call, and one record loop
(`src/engine/pipeline.rs:334-426` at HEAD). There is no loop over pagination
values and no second manifest request. Retries repeat the same URL, and
redirects follow the response location.

### The pagination field is inert

`Manifest` declares `pub pagination: String`
(`src/sources/manifest.rs:204`). Validation only rejects an empty string
(`src/sources/manifest.rs:300`). A repository search for `pagination` outside
tests finds only the schema, validation, and literal test manifests. The
adapter never reads it. The pipeline never reads it.

The untracked helper discussed in Appendix A is not referenced by the
committed pipeline.

### Runtime substitutions are empty

`main::config_values` ignores its argument and returns
`BTreeMap::new()` (`src/main.rs:106-110`). Both CLI entry paths pass that empty
map into `RunInput.values` (`src/main.rs:94`, `src/main.rs:182`).

`build_request` expands `{slug}`, `{key:<name>}`, and `{query:<name>}`
placeholders and returns an error for any missing value. It does not fall back
to a literal or an empty string (`src/sources/adapter.rs:296-350`).

The consequence at HEAD is stronger than "only page one is fetched" for most
paginated sources. For 14 of the 15 non-`none` manifests, the first request
cannot be built at all. A run with `--source arbeitnow` fails with a missing
query substitution for `page` before any socket is opened.

A working-tree run can behave differently because the uncommitted draft in
Appendix A seeds page, offset, and slug values. That draft is not part of
this snapshot.

Only `hackernews` has a request with no placeholders. It fetches
`jobstories.json` once. That response is an array of numbers. The configured
JSON path expects objects, so `record_from_aliases_with_url` returns `None` for
each number and the source emits zero jobs after a successful HTTP call.

The unit and replay tests pass because they construct `TemplateValues`
explicitly. That exercises the adapter without exercising the CLI value
plumbing.

### Retry only covers transport errors

`PlainFetcher::fetch` returns `Ok(Response)` for every HTTP status, including
429 and 5xx. It only returns `Err` for client construction, DNS, TLS, send, or
body-read failures (`src/fetching/plain.rs:37-82`).

`retry_async` therefore receives `Ok` for a 429 or 500 and returns it
immediately. The status check happens after the retry loop
(`src/engine/pipeline.rs:382-390`). The classifier at
`src/engine/pipeline.rs:354-360` inspects an error string for `429` or
`throttled`. No code path produces such a string from a status response.

Consequences at HEAD:

- HTTP 429 and 5xx are never retried.
- `parse_retry_after` (`src/engine/retry.rs:225`) is never called outside its
  own unit test.
- `Failure::Permanent` is never produced by the pipeline. DNS NXDOMAIN, TLS
  certificate failures, and unreachable networks are retried with the default
  policy.
- `retry_async` computes jitter as `attempt as u64` milliseconds
  (`src/engine/retry.rs:338`). The delays are effectively 200 ms, 401 ms, and
  802 ms for the default three retries. There is no random jitter.
- The retry loop does not inspect the HTTP method. A POST manifest can be
  retried after a transport error, which risks duplicate submissions.
- `config.timeout` only lowers `RetryPolicy.max_delay`
  (`src/engine/pipeline.rs:290-292`). It does not set the request timeout.
  `PlainFetcher::new()` keeps the 30 second default from
  `src/fetching/mod.rs:23`.
- There is no repeated-429 fast stop. The breaker only records challenge
  events.

The retry defaults are three retries after the first attempt, a 200 ms base
delay, and a 30 s maximum delay (`src/engine/retry.rs:12-16`).

### Concurrency is a per-source cap with no parallelism

`Governor` gives every source a `governor` crate token bucket and a Tokio
semaphore (`src/engine/governor.rs:100-124`). The bucket is
`Quota::per_minute(rate).allow_burst(1)` (`src/engine/governor.rs:115`).
`acquire` first takes a concurrency permit, then waits for a rate token
(`src/engine/governor.rs:136-170`).

At HEAD the permit is acquired once per manifest and held until that source
finishes (`src/engine/pipeline.rs:222`). Because `run_source` performs one
logical fetch and the source loop is sequential, `max_concurrency` never produces
more than one in-flight request. The semaphore is effectively a no-op.

`config.concurrency` replaces each source's manifest `max_concurrency`
(`src/engine/pipeline.rs:281`). It is not an aggregate bound. If sources are
parallelized later, the total could reach the number of sources multiplied by
the per-source value.

Rates come from `config.rates[source]` or the manifest
`requests_per_minute` (`src/engine/pipeline.rs:274-281`). The manifest
distribution is 65 sources at 20 requests per minute, 38 at 30, two at 10, two
at 12, one at 15, one at 60, and two at 170. At 20 per minute with burst 1 a
source gets one request every 3 seconds. A three-page run takes at least
6 seconds and a twenty-page run at least 57 seconds.

### Challenge detection and the breaker

`detect` classifies Cloudflare, DataDome, captcha, and JavaScript-required
responses. A challenge becomes an attention event and increments the
per-source breaker. The breaker opens after three failures for 300 seconds
(`src/engine/breaker.rs:14-17`).

`run_source` records the challenge before returning
(`src/engine/pipeline.rs:374`). The caller records the same event again in
its `SourceOutcome::Challenge` arm (`src/engine/pipeline.rs:244`). Two
challenge responses therefore reach the three-failure threshold and open the
breaker.

One ordering issue matters for retry and ban avoidance. `record_success` is
called before the status check (`src/engine/pipeline.rs:381-390`). A 429, 500,
or any other non-challenge failure therefore clears the source's failure
history. Repeated 429 or 5xx responses can never open the breaker.

### Plain-path identity and session handling

`PlainFetcher::fetch` builds a fresh `reqwest::Client` for every request
(`src/fetching/plain.rs:38-45`). It does not attach a default user agent, a
cookie jar, or a proxy. Only manifest headers are sent. Thirty-one of 111
manifests declare a `user-agent`, so the other 80 send no user agent.
`ProxySelector` exists but is wired only into `BrowserFetcher`
(`src/fetching/browser.rs:227-302`). The CLI always passes
`PlainFetcher::new()` (`src/main.rs:102`, `src/main.rs:190`).

`Manifest.fetcher`, `Manifest.profile`, and `Manifest.requires_js` are also
not consulted by the pipeline. Browser and emulation code exists but is not
reachable from a normal run.

The robots check is an exact string comparison against `allow`, `allowed`, or
`permit` (`src/engine/pipeline.rs:87-90`). Many manifests use a sentence such
as `allow public tenant board endpoints; skip access-controlled boards`.
Those sources are skipped with a `policy_skip` event and no failure. Sixty-seven
manifests pass the check and 44 are skipped.

### What the current engine already does well

- The per-source token bucket centralizes pacing. Burst 1 is stricter than the
  old per-scraper mix of fixed tickers and jitter.
- Governor waits and retry waits are cancellation-aware, and the kill switch
  interrupts both.
- Challenge detection covers Cloudflare, DataDome, captcha, and
  JavaScript-required pages and records scrubbed evidence.
- The breaker isolates one source and has a cooldown, even though the
  double-count bug described above makes it open earlier than intended.
- Response bodies are bounded, redirect targets are validated, and private
  address targets are rejected before a fetch.
- Deduplication by canonical URL works across pages once pages exist, because
  the seen set is shared for the whole run.

## Missing capabilities

### Pagination

| Capability | Old main | Current HEAD | Effect |
| --- | --- | --- | --- |
| Page-number iteration | per scraper | none | only one URL is requested |
| Offset iteration | per scraper | none | offset APIs return page one only, if they build at all |
| Cursor or next-link following | FindWork, Indeed, ZipRecruiter, JobDataAPI | none | a multi-page cursor API returns one page |
| Fanout over IDs | Hacker News | none | the ID list is treated as records and yields nothing |
| Page size control | per scraper constants | none | the first request uses the manifest literal or placeholder value |
| Empty-page stop | common | not applicable | no loop to stop |
| Short-page stop | common | not applicable | no loop to stop |
| Total-count stop | JobsDB and Jobstreet | not applicable | no loop to stop |
| Hard page caps | 3 to 100 depending on source | declared as free text | no cap is enforced |
| Repeated-page guard | per-source dedup | none | a cursor loop could repeat forever if implemented naively |
| Per-page request budget | implicit through `wanted` and caps | none | a loop could exhaust quota |
| Cap-aware request stop | loop checks before each page | record loop only | a cap cannot stop a page loop |
| Per-page progress | site telemetry | source-level event only | page progress is not observable |
| Strategy validation | not applicable | only non-empty | unsupported strings silently mean one request |

### Retry and backoff

| Capability | Old main | Current HEAD | Effect |
| --- | --- | --- | --- |
| Status-aware retry | 429 and 5xx | no | 429 and 5xx fail immediately |
| Retry-After | not supported | parser present but unused | server pacing instructions are ignored |
| Permanent error classification | DNS, TLS, route failures | no | hopeless requests consume retries |
| Method-aware retry | GET, HEAD, OPTIONS | no | POST can be retried |
| Repeated-429 stop | second consecutive 429 | no | repeated throttling is not detected |
| Random jitter | 0 to 119 ms plus bonuses | deterministic 0, 1, 2 ms | retry waves can align |
| Backoff cap tied to timeout | max delay only | max delay only | no request timeout change |
| Retry budget per run | no | no | a large source set can multiply retries |

### Concurrency

| Capability | Old main | Current HEAD | Effect |
| --- | --- | --- | --- |
| Global request cap | 2 to 16, default 8 | none | total in-flight work is unbounded if sources are parallelized |
| Per-source cap | 1 to 8 through `--site-rps` | manifest value | present but unused at one request per source |
| Per-host cap | token bucket existed but unused | none | two sources on one host do not share a budget |
| Permit scope | site goroutine | entire source run | a permit can cover retries and all future pages |
| Seed or item fanout | Greenhouse 5 workers | none | large boards are fetched one at a time or not at all |
| Memory-aware launch | 90 percent hard, 75 percent resume | one check before source | no page-level memory pressure check |

### Ban avoidance

| Capability | Old main | Current HEAD | Effect |
| --- | --- | --- | --- |
| Connection reuse | one client per scraper | new client per request | extra TLS handshakes and a changing fingerprint |
| Cookie jar | per client, reset every 200 | none | no session continuity across pages |
| User-agent rotation | three agents | 31 manifests, 80 without one | default client fingerprint is obvious |
| Browser-like headers | non-API paths | manifest headers only | WAF-sensitive pages look automated |
| Proxy use | health-checked rotation | browser path only | plain requests cannot distribute load |
| Browser challenge fallback | Playwright stealth | not wired | a challenge becomes attention with no escalation |
| Pacing between pages | tickers and jitter | per-source bucket | safe but slower and not page-aware |
| Adaptive rate | suggestion only | none | no feedback from throttling |
| robots parsing | not applicable | exact match | 44 manifests are skipped |

## Manifest pagination strings

Every manifest has the field. Ninety-six declare `none`. The table below
covers the fifteen that declare another value and explains what the string
means from the request template, the old scraper, and the unused parser draft.

| Declared string | Sources | Intended meaning | HEAD outcome |
| --- | --- | --- | --- |
| `none` | 96 manifests | one request | one request, or an early build error when other placeholders are unsupplied |
| `page:3` | `arbeitnow`, `simplyhired` | up to three numbered pages starting at 1 | build error before fetch for both |
| `page:10` | `careerjet` | pages 1 through 10 | build error before fetch |
| `page:20` | `adzuna` | pages 1 through 20 | build error before fetch |
| `page:1` | `ats-talentlyft` | page 1 only | build error before fetch |
| `page:1:8` | `internshala` | pages 1 through 8 | build error before fetch |
| `page:0:6` | `builtin` | pages 0 through 6, seven pages | build error before fetch |
| `offset:20:10` | `himalayas` | offsets 0, 20, ... 180, ten pages of 20 | build error before fetch |
| `offset:100` | `ats-smartrecruiters` | 100-item pages with the offset advancing by page length until a short page | build error before fetch |
| `offset:0:50` | `freelancercom` | unclear. the old scraper made one request with `limit=wanted` and no offset, so this has no old counterpart | build error before fetch |
| `cursor:next` | `findwork` | follow the response `next` URL until absent | build error before fetch |
| `fanout:10:45` | `hackernews` | 45 item IDs in batches of 10 | one request for the ID array and zero parsed jobs |
| `numbered pages` | `jobsdb`, `jobstreet` | sequential page numbers with a 10-page cap and 30 per page in old main | build error before fetch |
| `numbered pages with 429 retry backoff` | `reed` | sequential `pageno` until empty, with 429 backoff | build error before fetch |

### Why the strings are ambiguous

- `page:3` can mean page three or three pages. The unused draft reads it as
  pages one through three.
- `page:0:6` introduces a start and end with no declared step. Old Built In
  stopped at page 5, so six pages, while the string spans seven values.
- `offset:100` has one argument, `offset:20:10` has two, and
  `offset:0:50` is a third shape. The first likely means page size, the
  second means step and count, and the third could mean start and step.
- `offset:0:50` also has no old-code counterpart. The old Freelancer.com
  scraper made a single request with `limit=wanted` and no offset. Treat the
  declaration as unverified until the API's offset behavior is checked.
- `page:3` for SimplyHired is narrower than the old dynamic cap of
  `wanted/20 + 3` pages. The manifest string is a simplification.
- `numbered pages` is free text. It carries no page size, cap, or parameter
  name.
- Query names differ across requests. They include `page`, `pn`, `pageno`,
  `offset`, and `limit`. A parser that only looks for `{query:page}` or
  `{query:offset}` cannot cover `simplyhired`, `reed`, or
  `ats-smartrecruiters`.

## Proposed manifest pagination design

The design keeps the existing single `pagination` string and the existing
`{query:<name>}` request placeholder syntax. No new TOML key is required for
the first version. The string becomes a typed plan that is parsed once at
registry load, validated against the request template, and executed by one
generic loop.

### Goals

- Every declared strategy either executes correctly or fails validation. A
  silent one-page fallback is not acceptable.
- Every page is cancellable, rate-limited, concurrency-limited, and bounded by
  a request budget.
- The plan works for JSON APIs, path-based page numbers, static HTML, and
  cursor responses without source-specific code.
- The plan is additive to the current schema and does not borrow old scraper
  structure.

### Value resolution beyond pagination

The plan owns only the page, offset, cursor, and fanout values. Manifests also
use `{query:search}`, `{query:location}`, `{query:keywords}`,
`{query:limit}`, `{query:pagesize}`, `{query:where}`, and
`{query:per_page}`. The current CLI supplies none of them. A pagination
executor on its own will still fail for Arbeitnow, CareerJet, Adzuna, Reed,
SimplyHired, Himalayas, Freelancer.com, FindWork, JobsDB, Jobstreet, and
Built In until those values are resolved.

The implementation needs a small value resolver with three ordered sources.
Explicit caller or config values come first. Plan-derived values come second.
Manifest defaults come last. The plan can supply a page size to `limit`,
`pagesize`, `per_page`, or `results_per_page` when the offset step or an
explicit size makes it unambiguous. Search terms, locations, and keywords need
caller configuration. `until_short` is valid only when the page size is known.
A missing required value should fail at run start with a diagnostic that names
the placeholder.

### Grammar

The canonical grammar makes the query parameter explicit. This removes the
`page` versus `pn` versus `pageno` ambiguity.

| Form | Meaning |
| --- | --- |
| `none` | One request. |
| `page:<param>:<first>:<last>[:<step>]` | Set `{query:<param>}` to `first`, `first + step`, and so on through `last`. `last` may be `until_empty`. The default step is 1. |
| `offset:<param>:<start>:<step>:<count>` | Set `{query:<param>}` to `start + n * step` for `n` in `0..count`. `count` may be `until_short`. |
| `cursor:url:<path>` | Read the next URL from the JSON response at `path` and follow it until the value is absent or empty. |
| `cursor:token:<path>:<param>` | Read a token from the JSON response at `path`, set `{query:<param>}`, and repeat until the token is absent or empty. |
| `fanout:<batch>:<limit>:<id_path>:<item_url>` | Read IDs at `id_path`, expand `item_url` with `{id}`, and fetch in batches of `batch` up to `limit` requests. |

Rules:

- Parsing is case-insensitive and trims whitespace.
- `page` and `offset` require the named placeholder to exist in the request.
  Validation fails when it does not.
- `until_empty` and `until_short` need a request budget. Use a default hard
  cap of 100 requests per source per run, with a lower value from the result
  cap when one is set.
- `cursor:url` resolves relative URLs against the current page. The target
  must pass the existing SSRF and host validation before every request.
- `fanout` preserves input ID order when merging results. Worker concurrency
  is bounded by the source and global concurrency budgets.
- For `fanout`, split on the first four colons only. The final field is the
  item URL and may contain `:` and `//`. The executor expands `{id}` before
  validation. It does not pass `{id}` to the shared request renderer.
- An empty `id_path` in a `fanout` plan means the response root is the ID
  array, which fits the Hacker News shape.
- Unknown forms fail manifest validation with a diagnostic that names the
  manifest and the unsupported strategy.

### Manifest migration

The existing strings map to canonical plans as follows. These manifest edits
are part of the implementation change, not part of this audit.

| Source | Current | Canonical |
| --- | --- | --- |
| `arbeitnow` | `page:3` | `page:page:1:3` |
| `simplyhired` | `page:3` | `page:pn:1:3` |
| `careerjet` | `page:10` | `page:page:1:10` |
| `adzuna` | `page:20` | `page:page:1:20` |
| `ats-talentlyft` | `page:1` | `page:page:1:1` with the literal `page=1` changed to `{query:page}` |
| `internshala` | `page:1:8` | `page:page:1:8` |
| `builtin` | `page:0:6` | `page:page:0:6` with `page={query:page}` added to the request |
| `himalayas` | `offset:20:10` | `offset:offset:0:20:10` |
| `ats-smartrecruiters` | `offset:100` | `offset:offset:0:100:until_short` with `offset={query:offset}` added |
| `freelancercom` | `offset:0:50` | remove `pagination` until the API's offset behavior is verified, then `offset:offset:0:50:until_short` with `offset={query:offset}` added if supported |
| `findwork` | `cursor:next` | `cursor:url:next` |
| `hackernews` | `fanout:10:45` | `fanout:10:45:<id_field>:<item_url_template>` |
| `jobsdb` | `numbered pages` | `page:page:1:until_empty` or `page:page:1:10` to keep the old cap |
| `jobstreet` | `numbered pages` | `page:page:1:until_empty` or `page:page:1:10` to keep the old cap |
| `reed` | `numbered pages with 429 retry backoff` | `page:pageno:1:until_empty`. The 429 behavior belongs to retry policy, not pagination |

The `hackernews` migration needs the item endpoint in the manifest because
the current request only names the ID list. That endpoint is the only new
literal the design adds.

### Execution loop

1. Resolve the strategy at registry load. Reject unsupported forms and missing
   placeholders. Do this once, not per request.
2. Seed a per-source plan state with the first page, offset, cursor, or ID
   batch.
3. Before each request, check cancellation, the kill switch, the breaker, the
   memory ceiling, the result cap, and the remaining request budget.
4. Acquire the per-host rate token, the per-source concurrency permit, and the
   global concurrency permit. Release the permits after the response body is
   read.
5. Merge caller values with plan values and build the request. Plan values own
   their named keys. Other placeholders still come from caller configuration.
6. Fetch once and classify the result before retry handling.
7. On a challenge, emit an attention event and stop or open the breaker.
8. Parse the page, accumulate records, and deduplicate by canonical URL.
9. Evaluate stop conditions and choose one reason from `LastValue`,
   `EmptyPage`, `ShortPage`, `TotalReached`, `CursorExhausted`,
   `RecordCap`, `RequestBudget`, `RepeatedPage`, `Error`, or
   `Cancelled`.
10. Record one progress event per page with the page index, request count, new
    job count, cumulative count, and stop reason.

Error semantics:

- A first-page transport, status, or parse failure fails the source.
- A later-page failure stops the plan and returns the pages already collected.
  It must be reported as a partial failure, not as a clean success.
- A challenge on any page stops the plan and produces an attention event.

Cap semantics:

- Count deduplicated jobs that will actually be emitted, not raw parsed rows.
- Check the cap before issuing the next request.
- Stop the plan when the cap is reached, even if more pages exist.

Repeated-page guard:

- Keep the last request URL or cursor token per source.
- Keep a small set of seen cursor values or page keys.
- Stop with `RepeatedPage` when the next state equals a previous state.

### Retry design

Move classification ahead of the success return so status responses enter the
retry path.

| Observation | Classification | Action |
| --- | --- | --- |
| 2xx | success | parse |
| 3xx after redirect handling | success | parse the final response |
| 408, 425, 429 | throttled or retryable | honor `Retry-After`, then back off |
| 500 through 599 | retryable | exponential backoff with jitter |
| 400 through 499 except 408, 425, 429 | permanent | no retry |
| DNS NXDOMAIN, TLS certificate failure, no route | permanent | no retry |
| Connect reset, timeout, incomplete body | retryable | exponential backoff with jitter |
| Challenge markers | attention | no blind retry |

Additional rules:

- Parse `Retry-After` as seconds and as an HTTP date. Support both forms.
- Use full jitter or equal jitter, not `attempt as u64`.
- Apply retries only to GET, HEAD, and OPTIONS by default. A POST manifest must
  opt in explicitly before it can be retried.
- Keep a per-request attempt cap and a per-run retry budget.
- Open the breaker on repeated 429 or 5xx responses. Do not call
  `record_success` before the status check.
- Use separate settings for request timeout and maximum backoff. Do not derive
  one from the other.

### Concurrency design

- A global Tokio semaphore limits total in-flight requests across all sources.
  The default should match the old range, with 4 to 8 as a practical starting
  point. `config.concurrency` sets it.
- A per-source semaphore comes from `max_concurrency`. Acquire it per request,
  not per source run.
- A per-host token bucket comes from the source rate. Key it by host, and keep
  the per-source budget on top. When two sources share a host, the host bucket
  prevents combined bursts.
- Acquire rate and concurrency slots with cancellation-aware waits. The current
  order, concurrency then rate, is acceptable if the permit is per request.
  Document that paced waiting occupies a slot.
- Fanout workers are bounded by the smaller of the batch size and the source
  concurrency. They share the global and host budgets.
- The memory ceiling is checked before each request, not only before a source.

### Ban-avoidance design

- Reuse one client per source run so connection pooling, TLS sessions, and
  cookies persist across pages. A cookie jar per source is enough. It does not
  need to be global.
- Send a consistent browser identity for HTML sources and a minimal API
  identity for JSON sources. Use manifest headers first. Add a default user
  agent only when the manifest omits one.
- Wire `ProxySelector` into the plain path. Select per request, reject a proxy
  after a transport failure, and never log credentials.
- Honor `Retry-After` and use jitter after throttling so retries do not form a
  wave.
- Escalate to the browser fetcher only when the manifest declares
  `requires_js`, a browser `fetcher`, or a non-`none` `profile`. Otherwise a
  challenge becomes attention.
- Parse robots as a leading token. Treat `allow` or `permit` as allowed and
  `deny` or `disallow` as skipped. Reject unknown text at validation instead
  of silently skipping.
- Cap requests per source per run and cap total pages. A source that never
  returns an empty page must still terminate.

### Validation and tests

All tests can run offline with synthetic responses and a mock fetcher.

| Case | Assertion |
| --- | --- |
| Three-page plan | three request URLs, correct page values, merged jobs |
| Offset until short page | stops after the short page, correct offsets |
| Cursor URL two pages | follows the next URL, stops when absent |
| Cursor token | token is read and placed in the named query parameter |
| Fanout three IDs | three item requests, stable output order |
| Repeated cursor | stops with `RepeatedPage` after one repeat |
| Result cap | no request after the cap is reached |
| Request budget | stops at the configured page limit |
| 429 with Retry-After | waits at least the header value |
| 500 then 200 | retries once and succeeds |
| 404 | no retry, permanent failure |
| POST transport error | no retry by default |
| Repeated 429 | breaker opens and later requests stop |
| Cancel during backoff | returns promptly with a cancelled outcome |
| Global concurrency two | at most two requests in flight |
| Two sources on one host | host bucket paces the combined traffic |
| Unknown strategy | manifest validation fails |
| Missing placeholder | manifest validation fails |
| Later-page failure | partial jobs returned with a failure reason |

### Rollout order

1. Add canonical parsing and validation, and migrate the fifteen manifests.
   Fail closed for anything unsupported.
2. Implement the page and offset executors with stop reasons, caps, request
   budgets, and per-page events.
3. Implement cursor and fanout executors.
4. Move retry classification ahead of the success return and wire
   `Retry-After`, jitter, method policy, and repeated-429 handling.
5. Add the global semaphore, per-request permits, and per-host buckets.
6. Add client reuse, proxy wiring, identity defaults, browser escalation, and
   robots parsing.

Steps 2 and 4 are independent. Step 3 depends on a response path extractor and
the fanout item template. Step 5 changes latency and should be measured against
the manifest rate distribution before enabling more than one source at a time.

## Appendix A. Uncommitted pagination draft observed during the audit

Snapshot time 2026-11-27 21:21 IST. These files were not part of commit
`9310816` and were still changing:

- `src/engine/pagination.rs`, untracked, 148 lines,
  SHA-256 `ec44cc77ebf47adab55cbf65ccd62fa7cf3540ceb77661da2ab6b4c60ccf4749`.
- `src/engine/pipeline.rs`, modified and still changing during the audit.
  Hashes observed were
  `891a9bf8d3c7f18b7174c877a77935074f0a71c18204fae37812878bd45b6602` at
  21:21 IST, `9da7e949d2b9ad3b8201c38d6fd531c6170ceb6bff48901cc4a1e36d8c6a555d`
  at 21:31 IST, and
  `8896cd85cf7eda596fb84434229d8f1e813c97608c9fefd1ce0ac4c8f1b974ed` at
  21:38 IST. The list below describes behavior common to those snapshots.
  Re-check before relying on it.
- `src/engine/mod.rs`, modified to declare `pub mod pagination`.

The draft adds a `Strategy` enum with `None`, `Page`, and `Offset`, a
`MAX_PAGES` constant of 1000, a parser, and a `template_supports` check. The
pipeline draft loops over `strategy.values()`, inserts the value into the
`page` or `offset` query key, and repeats the existing fetch and parse path.

Gaps in that draft, if it lands unchanged:

- Only `{query:page}` and `{query:offset}` are recognized. `simplyhired`
  uses `pn`, `reed` uses `pageno`, and `ats-smartrecruiters` has a literal
  offset, so those declarations still fall back to `None` and one request.
- `offset:100` parses to a step of 100 with one page, which means a single
  request at offset 0.
- `offset:0:50` has a zero step and parses to `None`.
- `cursor:next`, `fanout:10:45`, and both `numbered pages` forms parse to
  `None`.
- There is no short-page, total-count, cursor, or repeated-page stop. Only an
  empty page ends a page or offset plan.
- The result cap is checked against raw accumulated records before each page.
  It does not account for deduplication or per-source caps cleanly.
- A non-first-page fetch, status, or parse failure breaks the loop and then
  reports success. There is no partial-failure diagnostic.
- `record_success` still runs before the status check.
- Status retries are unchanged. The draft still cannot retry 429 or 5xx.
- Runtime values remain empty for most query placeholders. The draft now seeds
  a slug from `manifest.slugs.first()`, so tenant sources pass the slug check.
  Paginated sources that also need search, limit, or key values still fail at
  `build_request` before their first page. That covers CareerJet, Adzuna,
  Himalayas, Freelancer.com, FindWork, Reed, JobsDB, Jobstreet, Built In, and
  SimplyHired.
- There are no per-page progress events, no request budget, and no global
  concurrency change.

The draft is a useful first step for page and offset plumbing. It does not
close the manifest grammar, stop-condition, retry, concurrency, or
ban-avoidance gaps described above.

## Appendix B. Evidence index

| Claim | Evidence |
| --- | --- |
| One request per source | `src/engine/pipeline.rs:334-426` at commit 9310816 |
| Pagination field only validated | `src/sources/manifest.rs:204`, `src/sources/manifest.rs:300` |
| Empty runtime values | `src/main.rs:106-110` |
| Missing placeholder is an error | `src/sources/adapter.rs` render path |
| Fetcher returns all statuses as success | `src/fetching/plain.rs:37-82` |
| Retry classifier inspects error strings | `src/engine/pipeline.rs:354-360` |
| Retry-After parser unused | `src/engine/retry.rs:225` and search results |
| Deterministic retry jitter | `src/engine/retry.rs:338` |
| Timeout only lowers max delay | `src/engine/pipeline.rs:290-292` |
| Per-source permit held for the run | `src/engine/pipeline.rs:222` |
| Burst of one | `src/engine/governor.rs:115` |
| Concurrency before rate wait | `src/engine/governor.rs:136-170` |
| Success recorded before status check | `src/engine/pipeline.rs:381-390` |
| Challenge recorded twice | `src/engine/pipeline.rs:374` and `src/engine/pipeline.rs:244` |
| Breaker threshold and cooldown | `src/engine/breaker.rs:14-17` |
| New client per request | `src/fetching/plain.rs:38-45` |
| Browser-only proxy wiring | `src/fetching/browser.rs:227-302` |
| Robots exact match | `src/engine/pipeline.rs:87-90` |
| Old global semaphore | `main:pkg/scrappy/engine.go` `globalConcurrency` and `Scrape` |
| Old site semaphores | `main:pkg/scrappy/engine.go` `buildSiteSemaphores` |
| Old retry transport | `main:internal/util/http.go` `smartRT` |
| Old pagination examples | `main:internal/scraper/*/scraper.go` |
| Old rate pool unused outside tests | `main:internal/rate/rate.go` and `main:tests/rate/rate_test.go` |
