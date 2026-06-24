# Proxy Support

scrappy supports SOCKS5 and HTTP/HTTPS proxies for distributed scraping and rate-limit avoidance. Proxies are configured at startup, health-checked via TCP dial, and rotated through the pool.

## Configuration

Proxy resolution follows a precedence chain:

```
--proxy CLI flag  >  config.toml proxy: field  >  SCRAPPY_PROXIES env var
```

### CLI flag

```bash
# Single SOCKS5 proxy
scrappy --sites linkedin --proxy socks5://user:pass@proxy:1080

# Multiple proxies (round-robin)
scrappy --sites indeed --proxy socks5://proxy1:1080,socks5://proxy2:1080

# HTTP proxy
scrappy --sites remoteok --proxy http://proxy:8080
```

### Config file

```toml
proxy = "socks5://user:pass@proxy:1080"
```

### Environment variable

```bash
export SCRAPPY_PROXIES="socks5://user:pass@proxy:1080"
```

### Per-request rotation

Two env vars control rotation frequency:

```bash
# Rotate proxy every N requests
export SCRAPPY_PROXY_ROTATE_EVERY_N=50

# Keep proxy sticky for N-request window
export SCRAPPY_PROXY_STICKY_WINDOW_N=20
```

Both use `smartRT.maybeRotateProxy()` which advances the proxy index atomically.

---

## Health checks — `internal/proxy/pool.go`

Before any scraping, the CLI performs a **TCP-dial health check** on every proxy:

```go
conn, dialErr := net.DialTimeout("tcp", net.JoinHostPort(host, port), 500*time.Millisecond)
```

- Timeout: 500ms per proxy.
- Port defaults: HTTP→80, SOCKS5→1080.
- Unhealthy proxies are logged and excluded from the pool.
- The filtered healthy list is set as `SCRAPPY_PROXIES` env var for the engine.

### `Pool` — proxy rotation manager

```go
type Pool struct {
    proxies []*ProxyURL
    idx     int
    mu      sync.Mutex
}
```

- **`Next()`**: returns the next healthy proxy in round-robin order. Skips unhealthy ones. Returns empty string if all proxies are dead.
- **`MarkUnhealthy(raw)`**: marks a proxy as unhealthy. Called when `NewHTTPClient` detects unreachable proxies during `parseProxyList`.
- **`MarkAllHealthy()`**: resets all proxies to healthy at the start of a new run.
- **`Probe(ctx, px)`**: performs a HEAD request to `https://httpbin.org/ip` through the proxy to verify it can make outbound HTTP requests.

### `ProxyURL` — health state

```go
type ProxyURL struct {
    Raw      string  // original URL
    Scheme   string  // socks5 | http | https
    HostPort string
    Healthy  bool    // thread-safe via RWMutex
}
```

---

## HTTP client integration — `internal/util/http.go`

The `NewHTTPClient` function builds an `http.Client` with proxy-aware transport:

```go
func NewHTTPClient(opts ClientOptions) *http.Client {
    proxyList := parseProxyList(opts.ProxyURL)
    if len(proxyList) > 0 {
        base.Proxy = http.ProxyURL(proxyList[0])
    }
    rt := &smartRT{base: base, opts: opts, proxyList: proxyList}
    return &http.Client{Transport: rt}
}
```

### `smartRT` — retry-aware transport

The `smartRT` wrapper around `http.Transport` provides:

1. **Proxy rotation**: `maybeRotateProxy()` advances the proxy index based on `ProxyRotateEveryN` and `ProxyStickyWindowN` counters.

2. **Retry logic**: retries on 429 (rate-limit) and 5xx (server errors) with exponential backoff + jitter:

    ```
    delay = BaseDelay(300ms) * 2^attempt + jitter (80-900ms)
    ```

    Permanent errors (NXDOMAIN, TLS cert failures, unreachable networks) fail immediately — retrying won't help.

3. **User-agent rotation**: cycles through a pool of modern browser UAs.

4. **Cookie jar reset**: periodically resets the cookie jar every N requests to avoid session tracking.

### Proxy reachability during `parseProxyList`

```go
func proxyReachable(u *url.URL) bool {
    // TCP dial to host:port with 500ms timeout
}
```

Note: for SOCKS5, TCP dial only confirms the port is open, not that a SOCKS handshake succeeds. Full SOCKS handshake validation requires a SOCKS library and is not attempted here.

---

## Retry delay with jitter

```go
func (s *smartRT) retryDelay(attempt int, resp *http.Response) time.Duration {
    d := s.opts.BaseDelay * time.Duration(1<<attempt)
    if d > s.opts.MaxDelay {
        d = s.opts.MaxDelay
    }
    jitter := time.Duration(rand.Intn(120)) * time.Millisecond
    if resp != nil {
        if resp.StatusCode == http.StatusTooManyRequests {
            jitter += 300 * time.Millisecond
        }
        if resp.StatusCode >= 500 {
            jitter += 80 * time.Millisecond
        }
    }
    return d + jitter
}
```

Base delay: 300ms, max delay: 4s. Attempt 1: ~300ms, attempt 2: ~600ms, attempt 3: ~1.2s, attempt 4: ~2.4s.

---

## Best practices

- **SOCKS5** proxies are preferred for higher anonymity.
- Set `--proxy` at the CLI for one-off proxy use; use `SCRAPPY_PROXIES` env var or config.toml for persistent setups.
- Proxy credentials in URLs are redacted in logs via `url.URL.Redacted()`.
- Use multiple proxies to distribute rate limits across IPs. scrappy round-robins through the pool.
- TCP-dial health check is fast (500ms timeout) but only confirms the port is open. For production, a periodic `Probe()` through the proxy is more reliable.
