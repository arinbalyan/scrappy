# Quickstart

Get scraping in 5 minutes.

## 1. Install

```bash
go install github.com/arinbalyan/scrappy/cmd/scrappy@latest
```

Verify the binary:

```bash
scrappy --version
# scrappy v0.1.0
```

## 2. First scrape

Scrape a single site with default output (JSONL to stdout):

```bash
scrappy --sites remoteok --search "golang" --results-wanted 50
```

Each job post is printed as a JSON line. Add `--format csv --out jobs.csv` to write to a file.

## 3. Interactive mode

Run without any flags:

```bash
scrappy
```

The interactive wizard guides you through search terms, location, sites, output format, and filters. When you finish, scrappy asks if you want to save settings to `~/.scrappy/config.yaml`.

See [005-Interactive-Mode.md](005-Interactive-Mode.md) for details.

## 4. Config file

Save a config to `~/.scrappy/config.yaml` or `./config.yaml` and scrappy loads it automatically:

```yaml
defaults:
  search: "AI Engineer"
  location: "Remote"
  results_wanted: 500
  out: "/data/jobs.jsonl"
  format: jsonl
  memory_cap: 512MB
```

Now you can just run:

```bash
scrappy --non-interactive
```

scrappy auto-detects the config by looking for `config.yaml` in the current directory first, then `~/.scrappy/config.yaml`. The `.env` file is loaded from the same directory as the config (see [015-Environment-Variables.md](015-Environment-Variables.md)).

### Per-site overrides

```yaml
defaults:
  search: "AI Engineer"
  location: "Remote"
sites:
  indeed:
    search:
      - "golang developer"
      - "rust engineer"
    location: "Remote"
    country: germany           # Sets indeed-co: DE header, German search host
  linkedin:
    search: '"AI Engineer" OR "ML Engineer"'
    location: Remote
  dice:
    search: "AI Engineer"
  reed:
    location: "United Kingdom"  # UK-only results
  naukri:
    search: ai engineer
    location: India
    country: india              # Indeed India endpoint
```

Site-specific search/location replaces the global default for that site. The `country` field (supported by Indeed) sets the `indeed-co` header and search host for country-specific results.

For full config file reference, see [008-Configuration.md](008-Configuration.md).

## 5. Multi-value (cartesian product)

Pass multiple search terms and locations with commas:

```bash
scrappy --sites indeed --search "AI Engineer,Software Developer" \
  --location "Remote,New York" --results-wanted 500
```

This produces 4 scrape passes per site: (AI Engineer x Remote), (AI Engineer x New York), (Software Developer x Remote), (Software Developer x New York). Results are aggregated; errors on one combination do not fail the others.

See [007-Multi-Value.md](007-Multi-Value.md) for details.

## 6. Memory cap

Limit total memory usage when running on constrained machines:

```bash
scrappy --sites linkedin,indeed,glassdoor --search "golang" \
  --memory-cap 512MB --results-wanted 200 --format jsonl
```

Concurrency scales automatically:

| Memory cap | Concurrent scrapers |
|------------|---------------------|
| up to 256 MB | 3 |
| up to 512 MB | 5 |
| up to 1 GB | 8 |
| more than 1 GB | 12 |

A background goroutine checks heap usage every 10 seconds and warns at 80% of the cap. See [016-Memory-Management.md](016-Memory-Management.md).

## 7. Quality score filtering

Filter low-quality postings before export:

```bash
scrappy --sites remoteok --search "golang" --results-wanted 100 \
  --min-score 60
```

The deterministic score (0-100) factors salary presence, direct apply links, email-domain match with company domain, posting freshness, description length, and agency status. See [011-Quality.md](011-Quality.md).

## 8. Proxies

Use proxies to avoid rate limits and IP blocks:

```bash
# Single SOCKS5 proxy
scrappy --sites linkedin,indeed --search "AI Engineer" \
  --proxy socks5://user:pass@proxy:1080 --results-wanted 500

# Multi-proxy round-robin
scrappy --sites linkedin,indeed,glassdoor --search "developer" \
  --proxy socks5://proxy1:1080,socks5://proxy2:1080,socks5://proxy3:1080
```

Or set in config:

```yaml
defaults:
  search: "AI Engineer"
  location: "Remote"
  proxy: socks5://user:pass@proxy:1080
```

At startup, scrappy TCP-dials each proxy (500ms timeout) and excludes unreachable ones. Priority: `--proxy` CLI flag overrides `config.yaml`, which overrides `SCRAPPY_PROXIES` env var.

See [013-Proxy.md](013-Proxy.md) for detailed guidance.

## 9. Deduplication

Deduplicate across sites by job URL or company name:

```bash
# URL dedup (default: on)
scrappy --sites linkedin,indeed --search "engineer" --dedup

# URL + company dedup (keeps one posting per company)
scrappy --sites linkedin,indeed --search "engineer" --dedup --dedup-by-company
```

See [014-Dedup.md](014-Dedup.md).

## 10. Docker

Build and run with Docker (Dockerfile provided):

```bash
# Build
docker build -t scrappy .

# Run
docker run scrappy --sites remoteok,remotive --search "rust" \
  --location "Remote" --results-wanted 200 --format jsonl --out /out/jobs.jsonl

# With mounted volume
docker run -v $PWD/data:/out scrappy \
  --sites indeed --search "golang" --results-wanted 100 \
  --format csv --out /out/jobs.csv
```

See [003-Installation.md](003-Installation.md) for Docker Compose and CI setup.

## Next steps

- [003-Installation.md](003-Installation.md) -- detailed setup for dev, CI, and Docker
- [002-Architecture-Overview.md](002-Architecture-Overview.md) -- pipeline data flow, packages, design decisions
- [004-CLI-Reference.md](004-CLI-Reference.md) -- full flag reference
- [012-Scraping.md](012-Scraping.md) -- per-site notes, rate limits, API keys
