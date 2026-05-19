# Proxy Setup

`internal/proxy/` — SOCKS5 / HTTP proxy pool with health checks.

## Local proxy on dev machine / VPS

Deploy [goproxy](https://github.com/snail007/goproxy) — single Go binary, no install:

```bash
# SOCKS5 on localhost:7890, optionally chain an upstream exit proxy
./goproxy -t socks5 -b 0.0.0.0:7890 --auth user:pass \
    -u socks5://exit-proxy-host:1080
```

```
scrappy scrape --sites linkedin,indeed --proxy socks5://localhost:7890 ...
```

Use `--local-proxy-port 7890` to auto-read `socks5://localhost:<port>`.

## GitHub Actions

```yaml
- name: Start local proxy
  run: |
    curl -fsSL https://github.com/snail007/goproxy/releases/download/v1.1.7/goproxy_1.1.7_linux_amd64.tar.gz | tar xz
    ./goproxy -t socks5 -b 0.0.0.0:7890 --auth ${{ secrets.PROXY_USER }}:${{ secrets.PROXY_PASS }} \
      -u socks5://${{ secrets.EXIT_PROXY }} &
    sleep 3
```

Then pass `--proxy socks5://localhost:7890` to the scrappy container.

## Health checks

Before a proxy enters the rotation pool, `http.Head("https://httpbin.org/ip")` is sent through it. Dead proxies are blacklisted for the rest of the run. Skip with `--proxy-health-check=false`.

## CLI flags

```
--proxy socks5://host:port,...   # Comma-separated
--local-proxy-port 7890          # Auto-build socks5://localhost:<port>
--proxy-health-check true       # Pre-flight probe (default: true)
```

## SOCKS5 proxy URL format

| Form | Meaning |
|---|---|
| `socks5://host:port` | No auth |
| `socks5://user:pass@host:port` | Auth |
| `socks5h://host:port` | SOCKS5, resolve DNS via proxy (not locally) |

Use `socks5h://` for LinkedIn and other sites where your IP leak would matter.
