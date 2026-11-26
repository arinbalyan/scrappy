# remoteok

Source id: remoteok
Family: feeds_api
Health: working
Endpoint: GET https://remoteok.com/api
Parser: json
Fetcher: plain
Credentials: none
Rate: 20 requests per minute
Concurrency: 1
Robots: allow
Attribution: RemoteOK

## Quirks

- The first array element is metadata and has no job fields.
- Timestamps are UTC with an explicit zone marker.

## Break and fix log

- 2026-11-26: Manifest and fixture recorded during registry setup.
