# Rate Limiting

scrappy implements rate limiting at two levels: a global token bucket limiting total outbound requests, and per-site semaphores limiting concurrent requests to individual job boards. On top of this, the HTTP client adds retry logic with exponential backoff and jitter.

---

## Per-site rate limiting — `internal/rate/rate.go`

### Token bucket pool

```go
type Pool struct {
    mu   sync.Mutex
    pool map[string]*rate.Limiter
}
```

Uses `golang.org/x/time/rate` token-bucket limiters, one per hostname/site:

```go
func (p *Pool) Get(key string, rps int) *rate.Limiter {
    if lim, ok := p.pool[key]; ok {
        return lim
    }
    lim := rate.NewLimiter(rate.Limit(rps), rps)
    p.pool[key] = lim
    return lim
}
```

- **Key**: site name or hostname (e.g. `"linkedin"`, `"indeed"`).
- **Burst**: equal to the RPS value (so a 5 RPS limiter allows bursts of 5).
- **`Wait(ctx, key, rps)`**: blocks until a token is available, respecting context cancellation.

The pool lazily creates limiters — only sites that are actually scraped get a limiter allocated.

### Per-site semaphores

In addition to the token bucket, the engine builds per-site semaphores from `--site-rps`:

```go
func buildSiteSemaphores(input model.ScraperInput) map[model.Site]chan struct{} {
    out := make(map[model.Site]chan struct{})
    for site, rps := range input.SiteRPS {
        capN := rps
        if capN <= 0 { capN = 1 }
        if capN > 8  { capN = 8 }
        out[site] = make(chan struct{}, capN)
    }
    return out
}
```

Usage: `linkedin:1,indeed:10` limits LinkedIn to 1 concurrent request and Indeed to 10. Clamped between 1 and 8.

---

## Global rate limiting — `pkg/scrappy/engine.go`

### Global concurrency semaphore

The engine uses a channel-based semaphore for global concurrency. The size is determined by:

1. **Memory cap** (`--memory-cap`): scales linearly:

    | Memory cap | Global concurrency |
    |------------|-------------------|
    | ≤ 256 MB | 3 |
    | ≤ 512 MB | 5 |
    | ≤ 1 GB | 8 |
    | > 1 GB | 12 |

2. **Max RPS** (`--max-rps`): used when no memory cap is set. Clamped between 2 and 16.

```go
func globalConcurrency(input model.ScraperInput) int {
    if input.MemoryCapMB > 0 {
        switch {
        case input.MemoryCapMB <= 256:  return 3
        case input.MemoryCapMB <= 512:  return 5
        case input.MemoryCapMB <= 1024: return 8
        default:                        return 12
        }
    }
    if input.MaxRPS > 0 {
        if input.MaxRPS < 2  { return 2 }
        if input.MaxRPS > 16 { return 16 }
        return input.MaxRPS
    }
    return 8
}
```

### Site-level concurrency

Each site goroutine holds the global semaphore slot plus its own site-specific semaphore slot. This means:

```
Global: max 8 concurrent sites
Site:   max N concurrent requests to a single site
```

---

## Retry logic — `internal/util/http.go`

The `smartRT` transport implements retry with exponential backoff and jitter:

```go
func (s *smartRT) RoundTrip(req *http.Request) (*http.Response, error) {
    attempts := s.opts.Retries + 1
    if !isRetryableMethod(req.Method) {
        attempts = 1  // only retry GET/HEAD/OPTIONS
    }
    for i := 0; i < attempts; i++ {
        s.maybeRotateProxy()
        resp, err := s.base.RoundTrip(req)
        if err == nil && !isRetryableStatus(resp.StatusCode) {
            return resp, nil  // success
        }
        // retry with backoff
        time.Sleep(s.retryDelay(i, resp))
    }
}
```

### Retryable status codes

```go
func isRetryableStatus(code int) bool {
    if code >= 500 && code < 600 {
        return true
    }
    return code == http.StatusTooManyRequests
}
```

- **429 (Too Many Requests)**: retried.
- **5xx (Server Error)**: retried.
- **4xx (Client Error)**: NOT retried — 403, 401, 406 indicate permanent conditions.

### Retryable methods

```go
func isRetryableMethod(method string) bool {
    switch strings.ToUpper(method) {
    case http.MethodGet, http.MethodHead, http.MethodOptions:
        return true
    default:
        return false
    }
}
```

POST and PUT requests are never retried to avoid duplicate side effects.

### Permanent errors (no retry)

```go
func isPermanentError(err error) bool {
    // DNS NXDOMAIN: domain doesn't exist
    // TLS/certificate errors
    // Network unreachable
    return true // fail immediately
}
```

### Backoff formula

```
delay = BaseDelay(300ms) × 2^attempt + jitter
```

| Attempt | Base delay | Max jitter | Total range |
|---------|-----------|------------|-------------|
| 1 | 300ms | 120ms + potential status-based bonus | ~300-420ms |
| 2 | 600ms | 120ms + bonus | ~600-720ms |
| 3 | 1.2s | 120ms + bonus | ~1.2-1.32s |
| 4 | 2.4s | 120ms + bonus | ~2.4-2.52s |

**Status-based jitter bonuses:**
- 429 (rate limit): +300ms
- 5xx: +80ms
- 403/401/406: +600ms

Max delay is capped at 4 seconds.

### Repeated 429 detection

If the same request gets two consecutive 429 responses, the transport fails permanently:

```go
if resp.StatusCode == http.StatusTooManyRequests && saw429 {
    return nil, fmt.Errorf("permanent: rate limited (repeated 429)")
}
```

This prevents infinite retry loops against aggressively rate-limiting servers.

---

## `suggestRPS` — adaptive rate suggestion

After each scrape, the engine calls `suggestRPS()` to propose an adjusted RPS for the site:

```go
func suggestRPS(current int, err error) int {
    if current <= 0 { current = 3 }
    if err == nil {
        if current < 10 { return current + 1 }  // increase on success
        return current
    }
    if containsAny(err.Error(), "429", "rate", "too many requests", "captcha") {
        if current > 1 { return current - 1 }  // decrease on rate-limit
        return 1
    }
    return current
}
```

- On success: gradually increase RPS (capped at 10).
- On rate-limit or captcha: decrease RPS (minimum 1).
- Stored in `RunTelemetry.SuggestedSiteRPS` for inspection but not automatically applied (the operator chooses whether to adopt the suggestion).

---

## Usage

```bash
# Global max RPS
scrappy --sites linkedin,indeed --search "golang" --max-rps 5

# Per-site RPS
scrappy --sites linkedin,indeed --search "golang" --site-rps "linkedin:1,indeed:10"

# Memory-constrained (automatically sets global concurrency)
scrappy --sites all --search "engineer" --memory-cap 512MB

# Both memory cap and per-site limits
scrappy --sites all --search "engineer" --memory-cap 1GB --site-rps "linkedin:1"
```
