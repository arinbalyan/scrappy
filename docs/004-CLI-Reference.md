# CLI Reference

## SYNOPSIS

```
scrappy [--flags]

scrappy --search "AI Engineer" --location "San Francisco, CA" \
        --sites linkedin,indeed,glassdoor --results-wanted 500 \
        --format jsonl --out /data/jobs.jsonl
```

Run without any flags on a TTY to enter the interactive wizard:

```
scrappy
```

## FLAGS

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--search` | string | `""` | Search term(s). Comma-separated for multi-value cartesian product. |
| `--location` | string | `""` | Location(s). Comma-separated for multi-value cartesian product. |
| `--sites` | string | `""` (all 55+) | Comma-separated site names. Omit to scrape every supported board. |
| `--results-wanted` | int | `0` | Maximum number of results total across all sites. |
| `--format` | string | `""` | Output format: `jsonl`, `csv`, `xlsx`, `parquet`. |
| `--out` | string | `""` | Output file path. Empty = stdout (JSON pretty-printed). |
| `--timeout` | int | `600` | Scrape timeout in seconds. |
| `--proxy` | string | `SCRAPPY_PROXIES` | Comma-separated proxy URLs (`socks5://`, `http://`). Overrides `SCRAPPY_PROXIES` env var and config `proxy` field. TCP-dial health check at startup filters unreachable proxies. |
| `--email` | bool | `false` | Only include jobs that have at least one email address. |
| `--is-remote` | bool | `false` | Only jobs flagged as remote (location-independent filter). |
| `--remote-only` | bool | `false` | Only truly remote jobs (no location filter applied). |
| `--job-type` | string | `""` | Filter: `fulltime`, `parttime`, `contract`, `internship`. |
| `--min-score` | int | `0` | Minimum quality score (0-100). Jobs below this threshold are dropped before export. See [011-Quality.md](011-Quality.md). |
| `--dedup` | bool | `true` | Drop duplicate job URLs across sites. See [014-Dedup.md](014-Dedup.md). |
| `--dedup-by-company` | bool | `false` | Keep only one posting per company across the entire result set. |
| `--max-rps` | int | `0` | Global maximum requests per second. Clamped between 2 and 16. Overrides the default concurrency of 8. |
| `--site-rps` | string | `""` | Per-site RPS overrides. Format: `site:rps,site:rps` (e.g. `linkedin:1,indeed:10`). |
| `--log-level` | string | `""` | Log verbosity: `DEBUG`, `INFO`, `WARN`, `ERROR`. |
| `--config` | string | auto | Path to config YAML. Auto-detects: `./config.yaml` then `~/.scrappy/config.yaml`. |
| `--memory-cap` | string | `""` | Memory budget. Examples: `512MB`, `1GB`, `256` (plain = MB). `0` or empty = unlimited. See [016-Memory-Management.md](016-Memory-Management.md). |
| `--non-interactive` | bool | `false` | Disable the interactive wizard. Required for cron, CI, Docker. |
| `--interactive` | bool | `true` | Force interactive wizard mode (auto-detected when TTY is available). |
| `--help` | -- | -- | Print help text and exit. |
| `--version` | -- | -- | Print version (`scrappy v0.1.0`) and exit. |

### Flag behaviour

- CLI flags **override** config YAML values.
- Config YAML values are used as **fallback defaults** when flags are omitted.
- `--sites` empty = all 55+ sites (from \`model.AllSites()\`).
- `--out` empty = JSON array written to stdout.
- `--non-interactive` disables the wizard even on a TTY; required for piped output.

## ENVIRONMENT VARIABLES

### Scrape proxies

| Variable | Description | Example |
|----------|-------------|---------|
| `SCRAPPY_PROXIES` | Comma-separated SOCKS5 proxy URLs (lowest priority) | `socks5://user:pass@proxy1:8080,...` |
| `SCRAPPY_PROXY_ROTATE_EVERY_N` | Rotate proxy every N requests | `10` |
| `SCRAPPY_PROXY_STICKY_WINDOW_N` | Sticky proxy window in seconds | `60` |

### Site-specific keys

| Variable | Required For | Get Keys At |
|----------|-------------|-------------|
| `ADZUNA_APP_ID` + `ADZUNA_APP_KEY` | adzuna | https://developer.adzuna.com/ |
| `CAREERJET_AFFID` | careerjet | https://www.careerjet.com/partners/ |
| `INFOJOBS_CLIENT_ID` + `INFOJOBS_CLIENT_SECRET` | infojobs | https://developer.infojobs.net/ |
| `FINDWORK_API_KEY` | findwork | https://findwork.dev/developers/ |
| `ARBEITSAGENTUR_API_KEY` | arbeitsagentur | https://rest.arbeitsagentur.de/ |
| `SCRAPPY_INDEED_API_KEY` | indeed (paid) | Indeed Affiliate API |
| `SCRAPPY_INDEED_CO` | indeed (company) | Indeed company override |

### Other

| Variable | Description |
|----------|-------------|
| `SCRAPPY_LOG_LEVEL` | Default log level: `DEBUG`, `INFO`, `WARN`, `ERROR` |
| `SCRAPPY_GREENHOUSE_SEEDS` | Comma-separated company names for Greenhouse scraping |

Env vars can be set in `.env` files (auto-loaded beside config.yaml) or exported in the shell. See [015-Environment-Variables.md](015-Environment-Variables.md).

## SITES

scrappy supports **55+ job boards**:

```
linkedin          indeed            naukri            internshala
builtin           startupjobs       greenhouse        gunio
himalayas         hiringcafe        huggingfacejobs   jobindex
remoteok          remotive          remotefirstjobs   jobspresso
hasjob            vuejobs           larajobs          arbeitnow
arbeitsagentur    hackernews        cryptocurrencyjobs androidjobs
jobicy            devopsjobs        crunchboard       cryptojobslist
dribbble          aijobs            workingnomads     ycjobs
ukvisajobs        google            glassdoor         adzuna
simplyhired       careerbuilder     careerjet         jooble
dice              monster           infojobs          reed
themuse           jobsdb            snagajob          djinni
headhunter        mycareersfuture   jobstreet         4dayweek
eurojobs          findwork          web3career
```

Pass them to `--sites` as comma-separated lowercase names:

```bash
scrappy --sites linkedin,indeed,remoteok,glassdoor \
        --search "rust developer" --location "Remote" \
        --results-wanted 200
```

Omit \`--sites\` entirely to scrape all 55+.

> **Browser fallback:** Sites behind anti-bot challenges (naukri [reCAPTCHA], monster [DataDome], jooble [Cloudflare]) can use an optional Playwright headless Chromium fallback. Install with `cd scripts && npm install`. See [docs/012-Scraping.md](012-Scraping.md#browser-fallback-anti-bot).

## EXIT CODES

| Code | Meaning |
|------|---------|
| `0` | Success -- jobs scraped and written. |
| `1` | General error -- invalid flags, network failure, config parse error, constraint violation. Non-zero exits from underlying tools propagate. |

## EXAMPLES

### 1. Interactive wizard (new users)

```bash
scrappy
```

### 2. Basic scrape, two sites

```bash
scrappy --sites remoteok,remotive --search "golang" \
        --location "Remote" --results-wanted 100
```

### 3. CSV export to file

```bash
scrappy --sites indeed,glassdoor --search "software engineer" \
        --location "San Francisco" --results-wanted 500 \
        --format csv --out /data/jobs.csv
```

### 4. Parquet export with memory cap

```bash
scrappy --sites linkedin,indeed,monster,dice \
        --search "data engineer" --location "Remote" \
        --results-wanted 1000 --format parquet \
        --out /data/jobs.parquet --memory-cap 512MB
```

### 5. Multi-value (cartesian product): 2 terms x 2 locations = 4 passes

```bash
scrappy --sites indeed --search "AI Engineer,Software Engineer" \
        --location "Remote,New York" --results-wanted 500
```

### 6. Filter by remote + full-time + min quality score

```bash
scrappy --sites linkedin,indeed,google \
        --search "rust developer" --is-remote \
        --job-type fulltime --min-score 60 --results-wanted 200
```

### 7. Non-interactive for cron

```bash
scrappy --sites indeed --search "golang" --location "Remote" \
        --results-wanted 200 --format jsonl --out /data/jobs.jsonl \
        --non-interactive
```

### 8. Pipe JSON to jq

```bash
scrappy --sites remoteok --search "rust" --results-wanted 10 \
        --non-interactive | jq '.title'
```

### 9. All 55+ sites, email filter, remote-only

```bash
scrappy --search "AI Engineer" --location "Remote" \
        --results-wanted 5000 --format jsonl \
        --email --remote-only --timeout 1200
```

### 10. Custom config path and log level

```bash
scrappy --config /etc/scrappy/production.yaml \
        --log-level DEBUG --non-interactive
```

### 11. With SOCKS5 proxy

```bash
scrappy --sites linkedin,indeed,glassdoor --search "AI Engineer" \
        --location "Remote" --results-wanted 500 \
        --proxy socks5://user:pass@proxy:1080
```

### 12. Multi-proxy round-robin

```bash
scrappy --sites indeed,monster --search "developer" \
        --location "Remote" --results-wanted 200 \
        --proxy socks5://proxy1:1080,socks5://proxy2:1080
```

### 13. With max RPS and per-site overrides

```bash
scrappy --sites linkedin,indeed,remoteok --search "engineer" \
        --location "Remote" --results-wanted 500 \
        --max-rps 10 --site-rps linkedin:1,indeed:5
```

### 14. Dedup by company

```bash
scrappy --sites linkedin,indeed --search "engineer" \
        --results-wanted 200 --dedup --dedup-by-company
```

### 15. Site-specific searches with country override (config.yaml)

```yaml
# config.yaml
defaults:
  search: AI Engineer
  location: Remote
  results_wanted: 1000
  out: /data/jobs.csv
  format: csv

sites:
  indeed:
    search:
      - '"AI Engineer" OR "ML Engineer"'
      - '"GTM Engineer" OR "Forward Deployed Engineer"'
    location: Remote
    country: germany           # Sets indeed-co: DE header
  linkedin:
    search: '"AI Engineer" OR "Machine Learning Engineer"'
    location: Remote
  reed:
    search: '"AI Engineer" OR "ML Engineer"'
    location: United Kingdom   # UK-only results
  internshala:
    search:
      - ai engineer
      - machine learning
    location: India
  naukri:
    country: india
```

### 16. Site-specific searches from CLI with config

```bash
# CLI search/location apply to all sites except those with overrides in config
scrappy --config my-sites.yaml --results-wanted 500 --format jsonl
```

### 17. With proxy from config

```yaml
# config.yaml
defaults:
  search: AI Engineer
  location: Remote
  results_wanted: 1000
  proxy: socks5://user:pass@proxy:1080
```
