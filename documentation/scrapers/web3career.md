# web3career

Source id: web3career
Family: feeds_api
Health: working
Endpoint: GET https://web3.career/api/v1/jobs?limit=25
Parser: JSON with field aliases
Fetcher: plain
Credentials: WEB3CAREER_API_TOKEN
Rate: 20 requests per minute
Concurrency: 2
Robots: allow
Attribution: Web3 Career

## Quirks

- Historical source required WEB3CAREER_API_TOKEN and accepted data or jobs envelopes; this port declares the credential name and reads the data envelope. Salary and remote fields are outside this adapter.

## Break and fix log

- 2026-11-26: Ported from historical behavior into the manifest adapter with synthetic fixtures.
- 2026-11-26: Live probe returned HTTP 404 for the assumed jobs path; endpoint needs correction before use.
