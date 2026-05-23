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

You'll see the ASCII logo and an interactive wizard. Enter search terms, location, sites, output format, and filters. When you're done, scrappy asks if you want to save settings to `~/.scrappy/config.yaml`.

## 4. Config file

Save a config to `~/.scrappy/config.yaml` or `./config.yaml` and scrappy will load it automatically:

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

scrappy auto-detects the config by looking for `config.yaml` in the current directory first, then `~/.scrappy/config.yaml`.

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
    location: "San Francisco"
  dice:
    search: "AI Engineer"
  reed:
    location: "United Kingdom"
```

Site-specific search/location replaces the global default for that site.

## 5. Multi-value (cartesian product)

Pass multiple search terms and locations with commas:

```bash
scrappy --sites indeed --search "AI Engineer,Software Developer" \
  --location "Remote,New York" --results-wanted 500
```

This produces 4 scrape passes per site: (AI Engineer × Remote), (AI Engineer × New York), (Software Developer × Remote), (Software Developer × New York). Results are aggregated; errors on one combo don't fail the others.

## 6. Memory cap

Limit total memory usage when running on constrained machines:

```bash
scrappy --sites linkedin,indeed,glassdoor --search "golang" \
  --memory-cap 512MB --results-wanted 200 --format jsonl
```

Concurrency scales automatically:

| Memory cap | Concurrent scrapers |
|---|---|
| ≤256 MB | 3 |
| ≤512 MB | 5 |
| ≤1 GB | 8 |
| >1 GB | 12 |

A background goroutine checks heap usage every 10 seconds and warns at 80% of the cap.

## 7. Docker

Build and run with Docker:

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

## Next steps

- [Installation](installation.md) — detailed setup for dev, CI, and Docker
- [Architecture](architecture.md) — pipeline data flow, packages, design decisions
- [CLI reference](cli.md) — full flag reference
- [Scraping](scraping.md) — per-site notes, rate limits, API keys
