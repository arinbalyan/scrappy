# Scrape Validation Results (2026-05-20)

## Scope
- Repository: `scrappy`
- Mode requested: run tests, run CLI scrape without proxy, run CLI scrape with localhost proxy, record results.

## Test run
Command:
- `go test ./...`

Result:
- PASS (all test packages succeeded)

## CLI scrape runs

### 1) Requested small scrape target (no proxy): `sites=indeed,linkedin`, `search="software engineer"`, `location="Remote"`, `results=10`
Command:
- `go run ./cmd/scrappy --non-interactive --sites indeed,linkedin --search "software engineer" --location "Remote" --results-wanted 10 --format jsonl --out /tmp/scrappy-no-proxy.jsonl`

Result:
- FAIL
- Error: `scrape indeed: indeed api status 401`

Notes:
- The run exits on Indeed failure before producing combined output.

### 2) Fallback no-proxy sanity scrape (single site): `sites=linkedin`
Command:
- `go run ./cmd/scrappy --non-interactive --sites linkedin --search "software engineer" --location "Remote" --results-wanted 10 --format jsonl --out /tmp/scrappy-no-proxy.jsonl`

Result:
- SUCCESS (command exit code 0)
- Output file existed but was empty in this run (`0` lines).

### 3) Additional no-proxy verification with a stable API site: `sites=remoteok`
Command:
- `go run ./cmd/scrappy --non-interactive --sites remoteok --search "software engineer" --location "Remote" --results-wanted 5 --format jsonl --out /tmp/scrappy-no-proxy-remoteok.jsonl`

Result:
- SUCCESS
- Output lines: `5`
- Sample rows observed with valid ids/URLs/timestamps.

### 4) Proxy run (localhost SOCKS5): `socks5://localhost:7890`
Command:
- `HTTPS_PROXY="socks5://localhost:7890" HTTP_PROXY="socks5://localhost:7890" go run ./cmd/scrappy --non-interactive --sites linkedin --search "software engineer" --location "Remote" --results-wanted 10 --format jsonl --out /tmp/scrappy-with-proxy.jsonl`

Result:
- FAIL
- Error: `proxyconnect tcp: dial tcp [::1]:7890: connect: connection refused`

Interpretation:
- Scrappy did attempt to use the proxy settings.
- Local proxy service was not accepting connections on port `7890` at execution time.

## Summary
- Core tests are green.
- CLI scraping works against at least one live source (RemoteOK).
- Requested Indeed+LinkedIn batch currently fails due to Indeed 401.
- Proxy path is wired and attempted, but local proxy endpoint was unavailable during run.
