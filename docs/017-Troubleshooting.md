# Troubleshooting

## Sites return 0 jobs

**Cause:** API keys not set, site is geo-restricted, or search term matched nothing.

**Solution:**

```bash
# Check which sites need keys
scrappy --help | grep -A 20 "SETUP"

# Verify env vars are loaded
env | grep -E '^(ADZUNA_|CAREERJET_|INFOJOBS_|FINDWORK_|ARBEITSAGENTUR_)'

# Run with debug logging to see per-site telemetry
scrappy --log-level DEBUG --sites indeed --search "engineer" --results-wanted 100
```

Check the telemetry output for `fail_open_reason`: `missing_credentials` means a key is needed.

## LinkedIn returns 429

**Cause:** Too many requests too fast. LinkedIn rate-limits aggressively -- typically after ~10 pages (100 jobs).

**Solution:**

```bash
# Slow down
scrappy --sites linkedin --search "engineer" --site-rps linkedin:1

# Use the rotate strategy to work around the 1k cap
scrappy --sites linkedin --search "engineer" --linkedin-strategy rotate

# Always use proxies for LinkedIn
scrappy --proxy socks5://localhost:7890 --sites linkedin --search "engineer"
```

LinkedIn 429s are handled silently (fail-open) -- the run continues for other sites.

## Indeed returns empty company names

**Cause:** Indeed's API started returning anonymous results in late 2024. Employer names are sometimes stripped.

**Solution:** Extract company name from the job URL path or use `--fallback-company-url` in library mode to parse employer branding from the URL.

## Docker build fails

**Cause:** Go version mismatch, CGO disabled, missing dependencies.

**Solution:**

```bash
# Verify Go version matches go.mod
go version  # should be 1.26+

# Build with explicit flags
CGO_ENABLED=0 go build -ldflags="-s -w" -o scrappy ./cmd/scrappy

# Multi-stage build ensures consistent toolchain
docker build --no-cache -t scrappy .
```

## Config not loading

**Cause:** Wrong path, invalid YAML, file permissions.

**Solution:**

```bash
# Default search order:
# 1. ./config.yaml (current directory)
# 2. ~/.scrappy/config.yaml

# Validate YAML syntax
python3 -c "import yaml; yaml.safe_load(open('config.yaml'))"

# Check file permissions
ls -la config.yaml        # should be readable (644)

# Run with explicit config path
scrappy --config /path/to/config.yaml --search "engineer"
```

## Email extraction shows no results

**Cause:** Many job boards hide contact emails by design (LinkedIn Easy Apply, Indeed direct apply). Emails in descriptions are rare (~10-25%).

**Solution:**

```bash
# Enable company-page enrichment (catches ~60-80%)
scrappy --sites indeed --search "engineer" --email-enrich \
  --email-enrich-domains careers,contact,about,team

# Lower the email-max-per-job limit (default 3)
scrappy --sites remoteok --search "golang" --email-max-per-job 10
```

## Out of memory

**Cause:** Running 55+ scrapers without a memory cap can consume 500+ MB.

**Solution:**

```bash
# Set a memory cap
scrappy --memory-cap 512MB --sites linkedin,indeed,remoteok --search "engineer"

# Reduce the number of sites
scrappy --sites indeed,remoteok --search "rust" --memory-cap 256MB
```

See [016-Memory-Management.md](016-Memory-Management.md).

## Scrape is too slow

**Cause:** Too many sites with low concurrency, or per-site rate limits are too aggressive.

**Solution:**

```bash
# Increase global concurrency (default 8)
scrappy --max-rps 12 --sites linkedin,indeed --search "engineer"

# Remove slow/broken sites
scrappy --sites indeed,remoteok,arbeitnow --search "golang"

# Increase per-site rate limits
scrappy --site-rps indeed:10,remoteok:8 --sites indeed,remoteok --search "engineer"

# Reduce results-wanted per site
scrappy --results-wanted 200 --sites linkedin,indeed --search "engineer"
```

## CSV output is empty

**Cause:** `--out` not specified (writes to stdout as JSON by default), or filter flags drop all results.

**Solution:**

```bash
# Always specify --out for CSV
scrappy --format csv --out /data/jobs.csv --sites indeed --search "engineer"

# Check if --min-score is too aggressive
scrappy --min-score 0 --sites indeed --search "engineer" --results-wanted 10

# Check if --email filter is too restrictive
scrappy --sites indeed --search "engineer" --results-wanted 10
```

## JSONL file is huge

**Cause:** JSONL is uncompressed text. Descriptions can be 10-50 KB each.

**Solution:**

```bash
# Use Parquet for 5-10x compression
scrappy --format parquet --out /data/jobs.parquet --sites indeed --search "engineer"

# Filter before export
scrappy --min-score 60 --sites indeed --search "engineer" --format jsonl --out /data/jobs.jsonl

# Pipe through gzip
scrappy --sites indeed --search "engineer" --out /dev/stdout | gzip > jobs.jsonl.gz
```

## Per-site telemetry shows errors

Run with `--log-level DEBUG` to see detailed telemetry per site. Common patterns:

| FailOpenReason | Meaning | Fix |
|----------------|---------|-----|
| `missing_credentials` | API key not set | Set the required env var (see [015-Environment-Variables.md](015-Environment-Variables.md)) |
| `challenge_detected` | CAPTCHA or Cloudflare | Use proxies, reduce rate |
| `rate_limited` | HTTP 429 | Lower `--site-rps` for this site |
| `access_denied` | HTTP 403/401 | Check geo-restrictions or API key validity |
| `timeout` | Request exceeded deadline | Increase timeout, check network |
| `unsupported_site` | Site name not in the scraper list | Check spelling, see `--help` for valid site names |
