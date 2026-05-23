# Proxy Setup

`internal/proxy/` -- SOCKS5 / HTTP proxy pool with health checks and round-robin rotation.

## Proxy URL formats

| Form | Meaning |
|------|---------|
| `socks5://host:1080` | SOCKS5, DNS resolved locally |
| `socks5://user:pass@host:1080` | SOCKS5 with auth |
| `socks5h://host:1080` | SOCKS5, DNS resolved via proxy (recommended) |
| `socks5h://user:pass@host:1080` | SOCKS5h with auth |
| `http://host:8080` | HTTP CONNECT proxy |
| `http://user:pass@host:8080` | HTTP with auth |
| `https://host:8080` | HTTPS CONNECT proxy |

Use `socks5h://` for LinkedIn and other sites where IP leak matters.

## Multi-proxy comma-separated list

```bash
scrappy --proxy socks5://user1:pass1@proxy1:1080,socks5://user2:pass2@proxy2:1080
```

Proxies are round-robined. Unhealthy proxies are excluded.

## CLI flags

| Flag | Description |
|------|-------------|
| `--proxy` | Comma-separated proxy URLs. Overrides `SCRAPPY_PROXIES` env var and config `proxy` field. |
| `--proxy-health-check` | Enable/disable startup health probes (default: true). |

## Config YAML

```yaml
defaults:
  proxy: socks5://host1:1080,socks5://host2:1080
```

Config value acts as fallback between CLI flag and environment variable.

## Environment variables

| Variable | Description |
|----------|-------------|
| `SCRAPPY_PROXIES` | Comma-separated proxy URLs (lowest priority; overridden by `--proxy` and config) |
| `SCRAPPY_PROXY_ROTATE_EVERY_N` | Rotate every N requests (0 = disabled) |
| `SCRAPPY_PROXY_STICKY_WINDOW_N` | Min requests before rotating away (default 20) |

The `SCRAPPY_PROXIES` env var is read by `internal/util.NewHTTPClient()` as a fallback. Proxy rotation and sticky-window settings are also read from env vars in `util.ClientOptions`.

## Precedence

```
--proxy CLI flag  >  config.yaml proxy field  >  SCRAPPY_PROXIES env var
```

## Startup TCP-dial health check

At startup, scrappy parses the proxy string, TCP-dials each proxy (500ms timeout), and only passes reachable proxies to the HTTP transport. Unreachable proxies are logged as warnings and excluded.

```
WARN proxy_unreachable proxy=socks5://dead-proxy:1080 err="dial tcp: i/o timeout"
INFO proxy_setup healthy=3 total=4 proxies=socks5://p1:1080,socks5://p2:1080,socks5://p3:1080
```

This happens before any scrape requests are sent. Proxies that fail the TCP dial are never used during the run.

## HTTP health probes

In addition to the TCP-dial check, the proxy pool runs an HTTP HEAD request through each proxy before adding it to the rotation:

```go
func (p *Pool) Probe(ctx context.Context, px *ProxyURL) bool {
    req, _ := http.NewRequestWithContext(ctx, http.MethodHead, "https://httpbin.org/ip", nil)
    proxyURL, _ := url.Parse(px.Raw)
    client := &http.Client{
        Timeout: 5 * time.Second,
        Transport: &http.Transport{
            Proxy: http.ProxyURL(proxyURL),
        },
    }
    resp, err := client.Do(req)
    return err == nil && resp.StatusCode >= 200 && resp.StatusCode < 400
}
```

Skip health checks with `--proxy-health-check=false` (default: `true`).

## Local proxy on dev machine / VPS

Deploy [goproxy](https://github.com/snail007/goproxy) -- single Go binary, no install:

```bash
# SOCKS5 on localhost:7890, optionally chain an upstream exit proxy
./goproxy -t socks5 -b 0.0.0.0:7890 --auth user:pass \
    -u socks5://exit-proxy-host:1080
```

```bash
scrappy --sites linkedin,indeed --proxy socks5://localhost:7890 --search "engineer"
```

## GitHub Actions

```yaml
- name: Install goproxy
  run: |
    curl -fsSL https://github.com/snail007/goproxy/releases/download/v1.1.7/goproxy_1.1.7_linux_amd64.tar.gz | tar xz
    chmod +x goproxy

- name: Start local proxy (background)
  run: |
    ./goproxy -t socks5 -b 0.0.0.0:7890 \
      --auth ${{ secrets.PROXY_USER }}:${{ secrets.PROXY_PASS }} \
      -u socks5://${{ secrets.EXIT_PROXY }} &
    sleep 3

- name: Run scrappy
  run: |
    docker run --network host scrappy:latest \
      --sites indeed,glassdoor,remoteok \
        --search "software engineer" --results-wanted 200 \
        --proxy socks5://localhost:7890
```

## Proxy rotation strategy

When multiple proxies are provided, scrappy round-robins requests across healthy proxies. Two environment variables control rotation behavior:

- `SCRAPPY_PROXY_ROTATE_EVERY_N`: Switch to the next proxy after N requests (0 = never rotate, stick to first healthy proxy).
- `SCRAPPY_PROXY_STICKY_WINDOW_N`: Minimum number of requests to send through one proxy before rotating away (default: 20).

This ensures that a failed proxy is detected quickly while maintaining session affinity for sites that require consistent IPs.
