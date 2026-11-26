# exa

Source id: exa
Family: keyed
Health: keyed
Endpoint: POST https://api.exa.ai/search
Parser: JSON with field aliases
Fetcher: plain
Credentials: EXA_API_KEY
Rate: 20 requests per minute
Concurrency: 2
Robots: allow
Attribution: Exa

## Quirks

- Historical source posted a search body with a domain allowlist and derived company from the result URL when author was absent; body filters and URL derivation are outside the adapter.

## Break and fix log

- 2026-11-26: Ported from historical behavior into the manifest adapter with synthetic fixtures.
- 2026-11-26: Live GET probe of the endpoint returned HTTP 404 because Exa accepts POST on this path.
