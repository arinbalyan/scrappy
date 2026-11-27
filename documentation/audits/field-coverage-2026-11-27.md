# Field coverage audit for the emitted Job schema

Audit date: 2026-11-27

Branch: `oculus`

Commit audited: `9310816` (`oculus` tip)

Working-tree note: the shared checkout received uncommitted changes while this
audit ran. Those changes extend `Record`, add JSON defaults for several
enrichment fields, read RSS `pubDate`, and copy the new record fields into
`JobInput`. They are not part of commit `9310816` and are summarized in
"Working-tree changes observed during the audit" below. The per-source tables
and priorities describe the committed tip.

Scope: `src/output/record.rs`, `src/parsing/mod.rs`, `src/parsing/json.rs`,
`src/parsing/html.rs`, `src/parsing/rss.rs`, `src/parsing/jsonld.rs`,
`src/sources/adapter.rs`, `src/sources/manifest.rs`, and all 111 manifests
under `src/sources/manifests/`.

## Verdict at commit 9310816

The `Job` schema is broad enough for most job-search automation. The
production data path is much narrower than the schema.

- The schema has 22 fields.
- 11 fields are always set by the production path.
- 2 fields are set only for JSON and RSS sources.
- 9 fields are never populated by any production path: `is_remote`,
  `date_posted`, `compensation`, `apply_method`, `job_url_direct`,
  `company_url`, `company_domain`, `company_description`, and
  `industry`.
- Parsers can supply at most five source fields: URL, title, company,
  location, and description. HTML supplies the first three only.
- All 111 manifests together use only five distinct alias keys: `url`,
  `title`, `company`, `location`, and `description`. No manifest defines
  an alias or selector for a date, salary, remote flag, direct apply URL,
  company URL, or industry field. Apply-style URLs appear only as fallback
  values for the canonical `url`.
- The effective maximum `quality_score` in production is 40 out of 100.

The result is a useful title, company, location, description, and URL feed
with generated identifiers. It does not yet carry posting dates, pay,
application routing, company identity, or remote status, even where the
source payload already contains those fields.

## Field status

| Field | Production status | Produced by | Notes |
|---|---|---|---|
| `schema_version` | always | `record.rs:215` | constant `1` |
| `source_id` | always | `record.rs:216` | manifest id |
| `external_id` | always | `record.rs:212` | content hash with source prefix |
| `canonical_url` | always | `record.rs:204`, `record.rs:218` | validated and normalized |
| `title` | always | `record.rs:219` | from `Record` |
| `company` | always | `record.rs:220` | from `Record` |
| `location` | conditional | `record.rs:221` | JSON and RSS only |
| `description` | conditional | `record.rs:222` | JSON and RSS only |
| `fetched_at` | always | `record.rs:223` | run timestamp |
| `content_hash` | always | `record.rs:224` | deterministic hash |
| `is_remote` | never | `record.rs:225` | `JobInput` default is `None` |
| `date_posted` | never | `record.rs:226` | `JobInput` default is `None` |
| `compensation` | never | `record.rs:227` | `JobInput` default is `None` |
| `apply_method` | never | `record.rs:228` | `JobInput` default is `None` |
| `job_url_direct` | never | `record.rs:229` | `JobInput` default is `None` |
| `company_url` | never | `record.rs:230` | `JobInput` default is `None` |
| `company_domain` | never | `record.rs:211`, `record.rs:231` | derived from `company_url` only |
| `company_description` | never | `record.rs:232` | `JobInput` default is `None` |
| `industry` | never | `record.rs:233` | `JobInput` default is `None` |
| `quality_score` | always | `record.rs:234` | 0 to 40 in practice |
| `source_ids` | always | `record.rs:213`, `record.rs:235` | single-element vector |
| `emails` | conditional | `record.rs:236` | scanned from description |

## What `fetched_at` means

`fetched_at` is the scrape run start time. The CLI calls
`pipeline::now_utc()` once when it builds `RunInput` (`src/main.rs:92` and
`src/main.rs:180`). `now_utc` formats the current UTC time to whole seconds
(`src/engine/pipeline.rs:145`). The pipeline copies that single value into
every `JobInput` for the run (`src/engine/pipeline.rs:410`). `from_record`
rejects an empty value and copies it to the job (`src/output/record.rs:201`,
`src/output/record.rs:223`).

Consequences:

- Every job from one run has the same timestamp, including jobs from different
  sources and different pages.
- Treat the value as the run start time. The time a particular response was
  parsed and the time the employer posted the job are separate facts.
- A long run can emit records many minutes or hours after the value suggests.
- `date_posted` is a separate concept and is currently never set. Consumers
  cannot compute job age or freshness from the output.

A separate `scraped_at` field is not needed to answer "when did this run
scrape". `fetched_at` already carries that meaning at run granularity. Add a
per-record timestamp only if consumers need response-level timing. Keep that
distinct from `date_posted`, which must come from the source payload.

## Code-level gaps

The gaps below describe commit `9310816`. The working-tree addendum later in
this document records the in-flight changes.

### G1. The parser record has only five fields

`Record` contains `url`, `title`, `company`, `location`, and
`description` (`src/parsing/mod.rs:15`). There is no place for a parser to
put a date, salary, remote flag, apply URL, company URL, industry, or native
source id. Every parser therefore discards those values before enrichment.

### G2. Only five alias names are ever read

`record_from_aliases_with_url` reads exactly `url`, `title`, `company`,
`location`, and `description` from the alias map
(`src/parsing/json.rs:100`). Manifest validation accepts any lowercase alias
key (`src/sources/manifest.rs:402`), so a future `date_posted` or
`compensation` alias would validate cleanly and have no effect. That is a
silent-drop trap for manifest authors.

### G3. The pipeline never fills the optional enrichment fields

The only production `JobInput` construction sets `source_id`, `fetched_at`,
and `emails` (`src/engine/pipeline.rs:408`). All other optional fields fall
back to `JobInput::default()`, which is `None`. `from_record` copies those
`None` values into the job (`src/output/record.rs:225` through
`src/output/record.rs:233`).

### G4. Nine schema fields have no producer

These fields are never set anywhere in production code:

| Field | Effect |
|---|---|
| `is_remote` | remote status always null |
| `date_posted` | posting date always null |
| `compensation` | pay always null |
| `apply_method` | application route always null |
| `job_url_direct` | direct apply URL always null |
| `company_url` | company website always null |
| `company_domain` | always null because it depends on `company_url` |
| `company_description` | always null |
| `industry` | always null |

Unit tests construct `JobInput` with some of these values
(`src/output/stream.rs:87`, `src/output/writer.rs:260`). Those tests cover
serialization. Production coverage is separate.

### G5. `company_domain` depends on a field that is never set

`company_domain` is computed only from `company_url`
(`src/output/record.rs:139`, `src/output/record.rs:211`). No manifest maps a
company URL and no pipeline code supplies one. The output contract says the
domain may come from source identity or `company_url`
(`openspec/specs/output-contract/spec.md:65`). The
source-identity path is not implemented.

### G6. `quality_score` cannot exceed 40 in production

The score adds 20 for a description, 10 for a remote flag, 10 for a posting
date, 10 for a company URL, 10 for a direct apply URL, 20 for an observed
email, and 20 for an email that matches the company domain
(`src/output/record.rs:150` through `src/output/record.rs:185`). Only the
description and observed-email components can fire in production. The domain
match needs `company_url`, which is always absent. A typical record scores 20
or 40.

### G7. Alias values must be JSON strings

`alias_field` calls `Value::as_str` (`src/parsing/json.rs:69`). Numeric
salaries such as `salary_min: 60000`, boolean remote flags such as
`isRemote: true`, and object salaries such as
`compensation: { minPay, maxPay }` are invisible to the current parser even
if an alias is added. Salary mapping needs number and object coercion in
addition to new aliases.

### G8. Location and salary cannot be composed

The alias lookup returns the first non-empty alias
(`src/parsing/json.rs:69`). It does not join `city`, `state`, and
`country`, and it does not format `min` and `max` into one salary string.
Manifests that list components produce only the first component, usually the
city. Manifests with salary ranges would need a formatter that does not exist.

### G9. HTML selectors cannot express description, location, or date

`Selectors` supports only `item`, `title`, `company`, `url`, and
`url_attribute` (`src/sources/manifest.rs:78`). `HtmlParser` emits only
those fields (`src/parsing/html.rs:71`). The eight HTML-parser sources
therefore always emit `location: null` and `description: null`.

### G10. RSS parsing drops `pubDate` and `guid`

`Item` has only `title`, `link`, `description`, and `creator`
(`src/parsing/rss.rs:32`). The 26 RSS sources cannot emit a posting date even
though 20 committed RSS fixture bodies contain a posting timestamp. Nineteen
use `pubDate` and `hasjob` uses `published`. The Atom `Entry` type also
omits `published` and `updated`.

### G11. The JSON-LD parser is unreachable

`src/parsing/jsonld.rs` extracts `JobPosting` blocks and can read
`title`, `url`, `hiringOrganization.name`, and description aliases
(`src/parsing/jsonld.rs:25`). `parse_response` dispatches only `json`,
`rss`, and `html` (`src/sources/adapter.rs:157`). No manifest uses
`parser = "jsonld"`. A manifest that did would fail with an unsupported
parser error. JSON-LD `datePosted`, `baseSalary`, `jobLocationType`,
`employmentType`, and `directApply` are therefore unavailable.

### G12. `external_id` is a local content hash

The field is documented as a stable external identifier
(`src/output/record.rs:65`). The implementation derives it from the source id
and the content hash (`src/output/record.rs:212`). It changes when the title,
company, URL, or description changes. Native ids present in payloads, such as
`id`, `slug`, or `ref`, are discarded because `Record` has nowhere to
keep them.

### G13. `source_ids` never merges

The pipeline deduplicates by canonical URL and keeps the first job
(`src/engine/pipeline.rs:416`). `from_record` always sets
`source_ids = vec![source_id]` (`src/output/record.rs:213`). The field
cannot show that two sources contributed the same job.

### G14. Relative HTML URLs are dropped at emission

`HtmlParser` returns the raw `href` (`src/parsing/html.rs:73`).
`from_record` requires an absolute `http` or `https` URL
(`src/engine/dedup.rs:50`). A relative href such as `/job/backend-701`
fails and the pipeline counts the record as skipped
(`src/engine/pipeline.rs:422`). The committed Built In fixture contains a
relative href, so the loss occurs on a live parsing path.

### G15. The `is_remote` configuration is unused

`Config` exposes `is_remote` (`src/config/mod.rs:196`) and the CLI parses
it. No pipeline code reads it. It cannot set the job field or filter jobs.

## Per-source coverage

Legend:

- `no` means no alias or selector exists for the field.
- `no†` means no mapping exists and the committed fixture body contains a
  usable field, so the loss is proven for that source.
- `no*` means no mapping exists for `job_url_direct`, and the manifest uses an
  apply URL as the canonical URL instead.
- `yes` means the field is mapped.
- `part` means RSS location is available only from Atom `<location>` or from
  a third title segment.
- `weak` means a description is mapped from a teaser, category, or code label
  in place of the full description.

`company_domain` and `industry` are `no` for every source. They are shown
in each table so the absence is explicit.

`comeet` and `greenhouse` fixtures carry updated timestamps
(`time_updated` and `updated_at`) rather than posting dates, so their date
column is `no`.

### ATS

| source | parser | date | salary | apply | domain | industry | location | remote | description |
|---|---|---|---|---|---|---|---|---|---|
| `adp` | json | no† | no† | no | no | no | yes | no | yes |
| `ats-ashby` | json | no† | no | no* | no | no | yes | no† | yes |
| `ats-gem` | json | no† | no | no | no | no | yes | no | yes |
| `ats-oracle` | json | no | no | no | no | no | yes | no | no |
| `ats-phenom` | json | no† | no | no* | no | no | yes | no† | yes |
| `ats-pinpoint` | json | no | no | no | no | no | yes | no† | yes |
| `ats-recruitee` | json | no† | no | no | no | no | yes | no† | yes |
| `ats-recruiterflow` | json | no† | no | no | no | no | yes | no | yes |
| `ats-smartrecruiters` | json | no† | no | no | no | no | yes | no† | yes |
| `ats-successfactors` | json | no† | no | no | no | no | yes | no | yes |
| `ats-talentlyft` | json | no† | no | no | no | no | yes | no | yes |
| `ats-teamtailor` | json | no | no | no* | no | no | yes | no† | yes |
| `ats-trakstar` | json | no | no | no* | no | no | yes | no | yes |
| `ats-ukg` | json | no | no | no* | no | no | yes | no | yes |
| `ats-workable` | json | no† | no | no | no | no | yes | no | yes |
| `ats-workday` | json | no† | no | no | no | no | yes | no | no |
| `bamboohr` | json | no | no† | no* | no | no | yes | no | yes |
| `breezyhr` | json | no† | no | no* | no | no | yes | no† | yes |
| `greenhouse` | json | no | no | no | no | no | yes | no | yes |
| `icims` | json | no† | no | no* | no | no | yes | no | yes |
| `jazzhr` | json | no† | no | no* | no | no | yes | no | yes |
| `jobscore` | json | no† | no | no | no | no | yes | no | yes |
| `jobvite` | json | no† | no | no* | no | no | yes | no | yes |
| `jobylon` | json | no | no | no | no | no | yes | no | yes |
| `loxo` | json | no | no | no* | no | no | yes | no | yes |
| `manatal` | json | no† | no† | no* | no | no | yes | no | yes |

### Feeds and APIs

| source | parser | date | salary | apply | domain | industry | location | remote | description |
|---|---|---|---|---|---|---|---|---|---|
| `4dayweek` | json | no | no | no | no | no | yes | no | yes |
| `academiccareers` | rss | no | no | no | no | no | part | no | yes |
| `androidjobs` | rss | no† | no | no | no | no | part | no | yes |
| `arbeitnow` | json | no | no | no | no | no | yes | no† | yes |
| `berlinstartupjobs` | rss | no† | no | no | no | no | part | no | yes |
| `canadajobbank` | json | no | no | no | no | no | yes | no | weak |
| `careeronestop` | json | no | no | no | no | no | yes | no | yes |
| `conservationjobs` | rss | no† | no | no | no | no | part | no | yes |
| `coroflot` | rss | no | no | no | no | no | part | no | yes |
| `crunchboard` | rss | no† | no | no | no | no | part | no | yes |
| `cryptocurrencyjobs` | rss | no† | no | no | no | no | part | no | yes |
| `cryptojobslist` | rss | no | no | no | no | no | part | no | yes |
| `deel` | json | no | no | no* | no | no | yes | no | yes |
| `devopsjobs` | rss | no† | no | no | no | no | part | no | yes |
| `dice` | json | no | no | no | no | no | yes | no | yes |
| `djinni` | rss | no† | no | no | no | no | part | no | yes |
| `drupaljobs` | rss | no† | no | no | no | no | part | no | yes |
| `echojobs` | json | no | no | no | no | no | yes | no | yes |
| `ecojobs` | rss | no† | no | no | no | no | part | no | yes |
| `eurojobs` | rss | no† | no | no | no | no | part | no | yes |
| `fossjobs` | rss | no† | no | no | no | no | part | no | yes |
| `fountain` | json | no | no | no* | no | no | yes | no | yes |
| `freelancercom` | json | no | no | no | no | no | yes | no | yes |
| `getonboard` | json | no† | no† | no | no | no | yes | no† | yes |
| `golangjobs` | rss | no† | no | no | no | no | part | no | yes |
| `hackernews` | json | no | no | no | no | no | no | no | yes |
| `hasjob` | rss | no† | no | no | no | no | part | no | yes |
| `headhunter` | json | no† | no | no | no | no | yes | no | yes |
| `higheredjobs` | rss | no | no | no | no | no | part | no | yes |
| `himalayas` | json | no† | no† | no* | no | no | yes | no | yes |
| `hnalgolia` | json | no | no | no | no | no | no | no | yes |
| `icrunchdata` | rss | no | no | no | no | no | part | no | yes |
| `jobdataapi` | json | no† | no† | no* | no | no | yes | no† | yes |
| `jobicy` | json | no | no† | no | no | no | yes | no | yes |
| `jobsch` | json | no† | no | no | no | no | no | no | weak |
| `jobsdb` | json | no† | no† | no | no | no | yes | no | weak |
| `jobsinjapan` | rss | no | no | no | no | no | part | no | yes |
| `jobspresso` | rss | no† | no | no | no | no | part | no | yes |
| `jobstreet` | json | no† | no† | no | no | no | yes | no | weak |
| `joinrise` | json | no | no | no | no | no | yes | no | yes |
| `landingjobs` | json | no | no | no | no | no | yes | no | yes |
| `mycareersfuture` | json | no† | no | no | no | no | yes | no | yes |
| `nofluffjobs` | json | no† | no† | no | no | no | yes | no† | weak |
| `pythonjobs` | rss | no† | no | no | no | no | part | no | yes |
| `railsjobs` | rss | no† | no | no | no | no | part | no | yes |
| `realworkfromanywhere` | rss | no† | no | no | no | no | part | no | yes |
| `remotefirstjobs` | rss | no† | no | no | no | no | part | no | yes |
| `remoteok` | json | no† | no | no* | no | no | yes | no | no |
| `remotive` | json | no | no | no | no | no | yes | no | yes |
| `snagajob` | json | no | no | no* | no | no | yes | no | yes |
| `startupjobs` | json | no | no | no | no | no | yes | no | yes |
| `talroo` | json | no | no | no | no | no | yes | no | yes |
| `themuse` | json | no | no | no | no | no | yes | no | yes |
| `ukvisajobs` | json | no | no | no | no | no | yes | no | yes |
| `web3career` | json | no | no | no | no | no | yes | no | yes |
| `weworkremotely` | rss | no† | no | no | no | no | part | no | yes |
| `wordpressjobs` | rss | no† | no | no | no | no | part | no | yes |
| `workingnomads` | json | no† | no | no | no | no | yes | no | yes |

### HTML (static)

| source | parser | date | salary | apply | domain | industry | location | remote | description |
|---|---|---|---|---|---|---|---|---|---|
| `aijobs` | html | no | no | no | no | no | no | no | no |
| `ats-rippling` | json | no | no | no | no | no | yes | no | yes |
| `builtin` | html | no | no | no | no | no | no | no | no |
| `functionalworks` | html | no | no | no | no | no | no | no | no |
| `gunio` | html | no | no | no | no | no | no | no | no |
| `hiringcafe` | json | no | no | no* | no | no | no | no | no |
| `huggingfacejobs` | json | no | no | no | no | no | yes | no | yes |
| `internshala` | html | no | no | no | no | no | no | no | no |
| `jobindex` | json | no | no | no | no | no | yes | no | yes |
| `reed` | json | no | no | no | no | no | yes | no | yes |
| `simplyhired` | html | no | no | no | no | no | no | no | no |
| `wellfound` | json | no | no | no | no | no | yes | no | no |
| `wuzzuf` | html | no | no | no | no | no | no | no | no |

### Keyed

| source | parser | date | salary | apply | domain | industry | location | remote | description |
|---|---|---|---|---|---|---|---|---|---|
| `adzuna` | json | no† | no† | no | no | no | yes | no | yes |
| `authenticjobs` | json | no† | no | no | no | no | yes | no | yes |
| `careerjet` | json | no† | no† | no | no | no | yes | no | yes |
| `comeet` | json | no | no | no | no | no | yes | no | yes |
| `crelate` | json | no† | no | no* | no | no | yes | no† | yes |
| `exa` | json | no† | no | no | no | no | no | no | yes |
| `findwork` | json | no† | no | no | no | no | yes | no† | yes |
| `freshteam` | json | no† | no | no* | no | no | yes | no† | yes |
| `hiringthing` | json | no | no | no* | no | no | yes | no | yes |
| `infojobs` | json | no† | no | no | no | no | yes | no | yes |
| `ismartrecruit` | json | no | no | no* | no | no | yes | no | yes |
| `jobtechdev` | json | no | no | no* | no | no | yes | no | yes |
| `usajobs` | json | no† | no | no | no | no | yes | no | yes |
| `ycjobs` | html | no | no | no | no | no | no | no | no |

## Prioritized remediation queue

Priority follows fixture evidence and the size of the parser change. A `†`
in the tables above is the evidence marker.

| Priority | Sources | Missing | Evidence | Why this order |
|---|---|---|---|---|
| P1 | `adp`, `adzuna`, `careerjet`, `getonboard`, `himalayas`, `jobdataapi`, `jobsdb`, `jobstreet`, `manatal`, `nofluffjobs` | date and salary | both fields are in the fixture | fixes two dead schema fields with known payload keys |
| P2 | 15 ATS, 20 RSS, 5 JSON feeds, 7 keyed sources listed below | date | timestamp is in the fixture | posting date is the highest-value single missing field |
| P3 | `bamboohr`, `jobicy` | salary | pay is in the fixture, no date | useful but lower coverage |
| P4 | 52 remaining sources | date and salary | no fixture evidence | needs live payload inspection before aliases are written |

### P1. Date and salary already present

| Source | Payload fields seen in fixture | Parser constraint |
|---|---|---|
| `adp` | `postedDate`, `compensation.maxPay`, `compensation.minPay` | nested object and numbers |
| `adzuna` | `created`, `salary_min`, `salary_max` | numbers |
| `careerjet` | `date`, `salary_min`, `salary_max`, `salary_type`, `salary_currency_code` | numbers |
| `getonboard` | `published_at`, `min_salary`, `max_salary` | numbers |
| `himalayas` | `pubDate`, `minSalary`, `maxSalary` | numbers |
| `jobdataapi` | `date_posted`, `salary_min`, `salary_max`, `salary_currency` | numbers and nulls |
| `jobsdb` | `listingDate`, `salaryLabel` | plain strings |
| `jobstreet` | `listingDate`, `salaryLabel` | plain strings |
| `manatal` | `created_at`, `salary_min`, `salary_max`, `salary_currency` | numbers |
| `nofluffjobs` | `posted`, `salary.from`, `salary.to`, `salary.currency`, `salary.type` | nested object |

### P2. Date present, salary absent

ATS sources: `ats-ashby` (`publishedDate`), `ats-gem`
(`created_at`), `ats-phenom` (`postedDate`), `ats-recruitee`
(`created_at`), `ats-recruiterflow` (`created_at`),
`ats-smartrecruiters` (`releasedDate`), `ats-successfactors`
(`postingStartDate`), `ats-talentlyft` (`CreatedAt`), `ats-workable`
(`published_on`), `ats-workday` (`postedOn`), `breezyhr`
(`published_date`, `creation_date`), `icims` (`datePosted`),
`jazzhr` (`original_open_date`), `jobscore` (`created_at`), and
`jobvite` (`date`).

RSS sources: `androidjobs`, `berlinstartupjobs`, `conservationjobs`,
`crunchboard`, `cryptocurrencyjobs`, `devopsjobs`, `djinni`,
`drupaljobs`, `ecojobs`, `eurojobs`, `fossjobs`, `golangjobs`,
`hasjob`, `jobspresso`, `pythonjobs`, `railsjobs`,
`realworkfromanywhere`, `remotefirstjobs`, `weworkremotely`, and
`wordpressjobs`. All use `pubDate` except `hasjob` (`published`).

JSON feed sources: `headhunter` (`published_at`), `jobsch`
(`publication_date`), `mycareersfuture` (`postedDate`), `remoteok`
(`date`), and `workingnomads` (`pub_date`).

Keyed sources: `authenticjobs` (`post_date`), `crelate`
(`created_date`), `exa` (`publishedDate`), `findwork`
(`date_posted`), `freshteam` (`created_at`), `infojobs`
(`published`), and `usajobs` (`PublicationStartDate`).

### P3. Salary present, date absent

| Source | Payload fields seen in fixture |
|---|---|
| `bamboohr` | `compensation` string |
| `jobicy` | `annualSalaryMin`, `annualSalaryMax`, `salaryCurrency` |

### P4. No fixture evidence

The remaining 52 sources need live payload inspection before aliases are
added. Their committed fixtures are trimmed examples and omit both fields.
Writing aliases from API documentation without a fixture would create
unverifiable mappings.

## Other schema observations

The following gaps are outside the per-source matrix but affect whether the
schema captures useful information.

- No stable source job id. `external_id` is a local content hash
  (`src/output/record.rs:212`). Consumers cannot reconcile updates to the same
  posting. Add a `source_job_id` field or preserve the native id in
  `external_id`.
- No structured salary. `compensation` is a free-form string
  (`src/output/record.rs:86`). Min, max, currency, and period are lost even
  when a source provides them.
- No employment type, seniority, department, team, or skills. Several
  payloads contain these values, for example `workplaceType` and
  `employmentType`.
- No closing or expiry date. `freshteam` exposes `closing_date` in its
  committed fixture.
- `apply_method` is a free-form string with no enum for email, ATS, or
  external URL. It is never populated.
- `source_ids` suggests cross-source attribution that the pipeline does not
  perform.
- `deny_unknown_fields` on `Job` (`src/output/record.rs:59`) makes additive
  schema evolution strict. A consumer built against version 1 will reject a
  version 1 record that adds an optional field, so a field addition needs a
  schema version decision.

## Working-tree changes observed during the audit

The shared checkout was not stable while this audit ran. At the time of
writing, uncommitted edits had made these changes:

- `Record` gained `date_posted`, `compensation`, `apply_method`,
  `job_url_direct`, `company_url`, `industry`, and `is_remote`.
- The JSON parser gained fallback readers for those fields, using common key
  names such as `posted_at`, `salary`, `apply_url`, and `isRemote`.
- The RSS parser gained `pubDate` and Atom `published` or `updated`.
- The pipeline gained code that copies the new record fields into
  `JobInput`.
- `company_description` still has no producer.
- Manifests still define only the five original alias keys.
- HTML selectors are unchanged and still cannot read description, location,
  or date.
- JSON values are still read through `Value::as_str`, so numeric salary
  ranges and salary objects remain unsupported.

When committed, those edits will partially close G1 and G4, close G3 and G10,
and make the new JSON fields honor aliases, which is part of G2. G2 remains
for `company_description` and for RSS and HTML parsers. G7, G8, G9, G11,
G12, G13, G14, and G15 remain. The per-source tables above describe commit
`9310816` and do not include this uncommitted work.

## Method and limits

- The audit read the source files and parsed all 111 manifest documents with
  Python's TOML reader. Alias counts are exact.
- Fixture evidence comes from the committed `response.body` files under
  `tests/fixtures/`. The scan looked for date, salary, remote, and apply keys
  in JSON bodies and XML tags.
- Fixtures are synthetic and sometimes trimmed. A missing key proves nothing
  about the live source. A present key proves the current parser drops a value
  that the fixture supplies.
- No live network requests were made and no tests were run.
- No code, manifest, fixture, or test file was changed by this audit.
- Line references use commit `9310816`. The shared working tree changed
  during the audit. See "Working-tree changes observed during the audit" for
  the uncommitted field-coverage work.
