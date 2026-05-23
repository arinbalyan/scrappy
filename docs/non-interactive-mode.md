# Non-Interactive Mode

Non-interactive mode is for cron jobs, CI pipelines, Docker containers, and shell scripts — anywhere there is no human at the keyboard to answer wizard prompts.

## How to activate

Non-interactive mode activates automatically when:

1. **stdin is not a TTY** (piped or redirected), OR
2. `--non-interactive` flag is passed explicitly.

```bash
# Explicit — always non-interactive
scrappy --sites indeed --search "golang" --non-interactive

# Automatic — piped stdin forces non-interactive
echo "" | scrappy --sites indeed --search "golang"

# Automatic — no TTY in a cron job
0 6 * * * /usr/local/bin/scrappy --sites indeed --search "golang" --config /home/user/.scrappy/config.yaml
```

## When to use non-interactive

| Use case | Example |
|----------|---------|
| **cron job** | Nightly scrape of Indeed + LinkedIn |
| **CI pipeline** | GitHub Actions step that scrapes and uploads results |
| **Docker container** | `docker run scrappy scrape --non-interactive` |
| **Shell script** | Bulk scrape across multiple term/location combinations |
| **Piped processing** | `scrappy ... \| jq ...` |
| **Redirection** | `scrappy ... > /data/jobs.json` |

## All flags must be specified (or have config defaults)

In non-interactive mode there are no prompts. Every required value must come from either:

- A **CLI flag**, or
- A **config YAML default**.

```bash
# OK: CLI provides everything
scrappy --sites indeed --search "golang" --location "Remote" \
        --results-wanted 200 --format jsonl --out /data/jobs.jsonl \
        --non-interactive

# OK: config.yaml provides defaults for search/location/format
# ~/.scrappy/config.yaml:
#   defaults:
#     search: golang
#     location: Remote
#     format: jsonl
scrappy --sites indeed --results-wanted 200 --out /data/jobs.jsonl \
        --non-interactive

# ERROR: no search term provided and none in config
scrappy --sites indeed --results-wanted 200 --non-interactive
# → constraint error, exit code 1
```

## Crontab examples

```cron
# Nightly LinkedIn scrape — every day at 6 AM
0 6 * * * /usr/local/bin/scrappy --sites linkedin --search "AI Engineer" \
        --location "Remote" --results-wanted 500 --format csv \
        --out /data/linkedin-$(date +\%Y\%m\%d).csv --non-interactive

# Weekly all-sites bulk scrape — Sunday at 2 AM
0 2 * * 0 /usr/local/bin/scrappy --search "software engineer" \
        --location "Remote" --results-wanted 10000 --format parquet \
        --out /data/weekly/$(date +\%Y\%W).parquet --memory-cap 1GB \
        --timeout 1800 --non-interactive

# Hourly remoteok check (lightweight)
0 * * * * /usr/local/bin/scrappy --sites remoteok --search rust \
        --results-wanted 50 --format jsonl \
        --out /data/remoteok-$(date +\%Y\%m\%d\%H).jsonl --non-interactive
```

## GitHub Actions workflow

```yaml
name: Daily scrape
on:
  schedule:
    - cron: '0 5 * * *'   # daily at 5 AM UTC
  workflow_dispatch:       # allow manual trigger

jobs:
  scrape:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.24'

      - name: Build
        run: go build -o scrappy ./cmd/scrappy/

      - name: Run scrape
        run: |
          ./scrappy --sites remoteok,remotive,arbeitnow \
            --search "rust developer" --location "Remote" \
            --results-wanted 200 --format csv --out jobs.csv \
            --non-interactive

      - name: Upload results
        uses: actions/upload-artifact@v4
        with:
          name: jobs
          path: jobs.csv
```

## Docker

```bash
# Build the image
docker build -t scrappy .

# Run non-interactive scrape, output to mounted volume
docker run --rm -v /data:/out scrappy \
  --sites indeed,linkedin,remoteok \
  --search "golang" --location "Remote" \
  --results-wanted 500 --format jsonl --out /out/jobs.jsonl \
  --non-interactive

# With environment-based proxy
docker run --rm \
  -e SCRAPPY_PROXIES="socks5://user:pass@proxy:1080" \
  -v /data:/out scrappy \
  --sites indeed --search "golang" --results-wanted 100 \
  --non-interactive
```

## Piping output

When `--out` is omitted, results are written to stdout as pretty-printed JSON:

```bash
# Pipe to jq for field extraction
scrappy --sites remoteok --search "rust" --results-wanted 10 \
        --non-interactive | jq '.[] | {title, company_name, job_url}'

# Pipe to grep for keyword filtering
scrappy --sites remoteok --search "rust" --results-wanted 50 \
        --non-interactive | grep -i "senior"

# Redirect to file (no --out flag needed)
scrappy --sites remoteok --search "rust" --results-wanted 50 \
        --non-interactive > /tmp/jobs.json

# Chain with other tools
scrappy --sites remoteok --search "golang" --results-wanted 100 \
        --non-interactive | jq -r '.[].title' | sort -u
```

## Redirecting to file without --out

```bash
# These are equivalent:
scrappy --sites remoteok --search rust --results-wanted 10 \
        --non-interactive > /tmp/jobs.json

scrappy --sites remoteok --search rust --results-wanted 10 \
        --non-interactive --out /tmp/jobs.json
```

Note: omitting `--format` when redirecting to stdout defaults to JSON. Use `--format csv` explicitly if you need a different format written to stdout.
