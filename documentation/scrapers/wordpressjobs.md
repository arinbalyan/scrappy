# wordpressjobs

Source id: wordpressjobs
Family: feeds_api
Health: working
Endpoint: GET https://jobs.wordpress.net/feed/
Parser: rss
Fetcher: plain
Credentials: none
Rate: 20 requests per minute
Concurrency: 2
Robots: allow
Attribution: WordPress Jobs

## Quirks

- RSS items may carry `pubDate` and `guid` fields that this adapter ignores.
- Company is derived from the job URL host when the feed has no company tag.

## Break and fix log

- 2026-11-26: Ported from historical behavior into the manifest adapter with synthetic fixtures.


## Live validation

- 2026-11-26: GET returned HTTP 200 with a valid RSS feed containing current WordPress job items; parser shape matched the baseline RSS parser.
