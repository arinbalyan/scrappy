# Scraper Status

## Overview

scrappy supports 141 sites across 3 categories:

| Category | Count | Description |
|----------|-------|-------------|
| Working | 43 | Return jobs out of the box |
| Niche | 10 | RSS feeds work, no SWE jobs expected |
| Not working | 45 | Timeout, blocked, or broken |
| Needs API key | 15 | Require env vars to function |
| Needs company slug | 28 | ATS providers need seed companies |

## Working sites

These 43 sites return job listings with the default configuration:

`internshala` `builtin` `himalayas` `devopsjobs` `reed` `jobsdb`
`mycareersfuture` `jobstreet` `bayt` `ats-ashby` `ats-smartrecruiters`
`remoteok` `ycjobs` `linkedin` `freelancercom` `workingnomads`
`remotive` `jobindex` `aijobs` `gunio` `remotefirstjobs` `jobicy`
`wuzzuf` `landingjobs` `railsjobs` `realworkfromanywhere`
`weworkremotely` `huggingfacejobs` `arbeitnow` `androidjobs`
`cryptocurrencyjobs` `fossjobs` `pythonjobs` `hackernews` `golangjobs`
`functionalworks` `hasjob` `crunchboard` `themuse` `ats-recruitee`
`jobspresso` `ukvisajobs` `ats-bamboohr`

## Niche boards

These sites work but return 0 for "software engineer" searches:

`conservationjobs` `coroflot` `drupaljobs` `ecojobs` `higheredjobs`
`snagajob` `wordpressjobs` `larajobs` `vuejobs` `pyjobs`

## Sites needing API keys

| Site | Env Var |
|------|---------|
| adzuna | `ADZUNA_APP_ID`, `ADZUNA_APP_KEY` |
| careerjet | `CAREERJET_AFFID` |
| indeed | `SCRAPPY_INDEED_API_KEY` |
| dice | `SCRAPPY_DICE_API_KEY` |
| findwork | `FINDWORK_API_KEY` |
| arbeitsagentur | `ARBEITSAGENTUR_API_KEY` |
| web3career | `WEB3CAREER_API_TOKEN` |
| jobtechdev | `JOBTECHDEV_API_KEY` |
| authenticjobs | `AUTHENTICJOBS_API_KEY` |

## ATS sites needing company seeds

42 ATS providers with company slug support. 25 have embedded slugs, 17 are empty.

See `documentation/status/` for the full breakdown.
