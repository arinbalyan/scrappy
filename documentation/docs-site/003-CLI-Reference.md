# CLI Reference

## Usage

```bash
scrappy [flags]
scrappy <command> [flags]
```

## Commands

| Command | Description |
|---------|-------------|
| `scrape` | Run a scraping job (default command) |
| `doctor` | Diagnose and fix setup issues |
| `setup` | Interactive setup wizard |

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--search` | `""` | Search term (e.g. "software engineer") |
| `--sites` | all | Comma-separated site names |
| `--location` | `""` | Search location |
| `--results-wanted` | `0` | Max results per site |
| `--format` | `jsonl` | Output format: jsonl, csv, xlsx, parquet |
| `--out` | stdout | Output file path |
| `--proxy` | env | Comma-separated proxy URLs |
| `--log-level` | `INFO` | Log level: DEBUG, INFO, WARN, ERROR |
| `--timeout` | `600` | Scrape timeout in seconds |
| `--is-remote` | `false` | Only remote jobs |
| `--hours-old` | `0` | Only jobs within N hours |
| `--min-score` | `0` | Quality score floor (0-100) |
| `--max-rps` | `0` | Global max requests/second |
| `--config` | auto | Path to config file |
| `--non-interactive` | `false` | Disable interactive wizard |
| `--interactive` | `false` | Force interactive wizard |
| `--version` | | Print version and exit |
| `--help` | | Print help |

## Examples

```bash
# Basic scrape
scrappy --sites remoteok --search "golang" --results-wanted 50

# Multiple sites with output file
scrappy --sites linkedin,indeed,remoteok \
  --search "software engineer" \
  --location "Remote" \
  --results-wanted 200 \
  --format csv \
  --out ./results.csv

# With proxy
scrappy --sites linkedin \
  --search "ai engineer" \
  --proxy socks5://user:pass@proxy:1080

# All sites with quality filter
scrappy --search "python" \
  --results-wanted 10 \
  --min-score 50 \
  --format jsonl

# Debug mode
scrappy --sites remoteok --search "rust" --log-level DEBUG

# Diagnose setup
scrappy doctor

# Setup wizard
scrappy setup
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `SCRAPPY_PROXIES` | Comma-separated proxy URLs |
| `SCRAPPY_LOG_LEVEL` | Default log level |
| `SCRAPPY_INDEED_API_KEY` | Indeed API key |
| `SCRAPPY_DICE_API_KEY` | Dice API key |
| `SCRAPPY_GREENHOUSE_SEEDS` | Company slugs for Greenhouse |
| `SCRAPPY_ATS_MAX_SEEDS` | Max ATS company seeds (default: 20) |
| `ADZUNA_APP_ID` | Adzuna API credentials |
| `ADZUNA_APP_KEY` | Adzuna API credentials |
| `CAREERJET_AFFID` | Careerjet affiliate ID |
| `FINDWORK_API_KEY` | Findwork API key |
| `ARBEITSAGENTUR_API_KEY` | Arbeitsagentur API key |

See `.env.example` for the full list.
