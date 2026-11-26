# cryptocurrencyjobs

Source id: cryptocurrencyjobs
Name: Cryptocurrency Jobs
Family: feeds_api
Health: working
Endpoint: GET https://cryptocurrencyjobs.co/index.xml
Parser: rss
Fetcher: plain
Pagination: none
Profile: none
Credentials: none
Slugs: none
Rate: 20 requests per minute
Concurrency: 1
Robots: allow
Attribution: Cryptocurrency Jobs
 Title separator: `" at "`
 Company: Title suffix supplies the company; titles use `Role at Company`; `company_first = false`

## Quirks

- Items carry CDATA-wrapped HTML descriptions and RFC1123 pubDate values.
- The company is taken from the text after ` at ` in each title.
- Category values are not mapped into normalized records.

## Break and fix log

- 2026-11-26: Tier 1 manifest, synthetic six-file fixture, and offline replay test added.

