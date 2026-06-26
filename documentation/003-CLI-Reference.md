# CLI Reference

## Usage

```bash
scrappy [flags]
scrappy <command> [flags]
```

## Commands

The root command (`scrappy` with no subcommand) runs a scrape — this is the
default invocation. Two subcommands are registered:

| Command | Description |
|---------|-------------|
| `doctor` | Diagnose and fix setup issues (config, env, network) |
| `setup`  | Interactive setup wizard |

## Flags

Flags are grouped by purpose (matching `scrappy --help`). Defaults shown are
the cobra flag defaults — most are overrideable by `config.toml` or env vars.

### Scraping flags

| Flag | Default | Description |
|------|---------|-------------|
| `--search` | `""` | Comma-separated terms (e.g. `"software engineer,AI Engineer"`) |
| `--sites` | all 141 | Comma-separated site names (empty = all 141) |
| `--location` | `""` | Comma-separated locations (e.g. `"Remote,New York"`) |
| `--results-wanted` | `0` | Max results total (0 = unlimited) |
| `--timeout` | `600` | Scrape timeout in seconds |
| `--proxy` | env | Comma-separated proxy URLs (`socks5://`, `http://`); TCP-dial health-checked at startup |
| `--memory-cap` | `""` | Memory budget: `512MB`, `1GB` (0 = unlimited) |
| `--max-rps` | `0` | Global max requests per second |
| `--site-rps` | `""` | Per-site RPS overrides, e.g. `linkedin:1,indeed:10` |
| `--site-results-wanted` | `""` | Per-site result caps, e.g. `indeed:5000,linkedin:1000` |

### Filter flags

| Flag | Default | Description |
|------|---------|-------------|
| `--email` | `false` | Only jobs with ≥ 1 email address |
| `--is-remote` | `false` | Only jobs flagged as remote |
| `--remote-only` | `false` | Only truly remote jobs (no location filter) |
| `--job-type` | `""` | `fulltime`\|`parttime`\|`contract`\|`internship` |
| `--hours-old` | `0` | Jobs posted within N hours (0 = no filter) |
| `--since` | `""` | Jobs posted on or after date (`RFC3339` or `YYYY-MM-DD`) |
| `--min-score` | `0` | Quality score floor (0-100) |
| `--enforce-annual-salary` | `false` | Normalize salaries to yearly amounts |

### Output flags

| Flag | Default | Description |
|------|---------|-------------|
| `--format` | `jsonl` | `jsonl`\|`csv`\|`xlsx`\|`parquet` |
| `--out` | stdout | Output file path |
| `--csv-emails-only` | `false` | CSV: one row per email instead of one row per job |
| `--json-pretty` | auto | Pretty-print JSON (stdout only) |
| `--json-minify` | `false` | Force minified JSON even on stdout |

### GitHub flags

| Flag | Default | Description |
|------|---------|-------------|
| `--github-scrape` | `false` | Discover emails from GitHub orgs/repos instead of job scraping (requires `--search`, see [GitHub Discovery](013-GitHub-Discovery.md)) |

### Verification flag

| Flag | Default | Description |
|------|---------|-------------|
| `--verify-concurrency` | `5` | MX-lookup concurrency. **`0` skips MX verification entirely** (useful when DNS is unavailable or to speed up large runs). |

### Setup & debug flags

| Flag | Default | Description |
|------|---------|-------------|
| `--config` | auto | Path to `config.toml` (CWD → `~/.scrappy/config.toml`) |
| `--log-level` | `""` | `DEBUG`\|`INFO`\|`WARN`\|`ERROR` |
| `--non-interactive` | `false` | Disable interactive wizard (for scripts/CI) |
| `--interactive` | `false` | Force wizard mode |
| `--dedup` | `true` | Deduplicate jobs by URL across sites |
| `--dedup-by-company` | `false` | Keep only one posting per company |
| `--version` | | Print version and exit |
| `--version-json` | `false` | Print version info as JSON and exit |
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

# Discover emails from GitHub (see 013-GitHub-Discovery.md)
GITHUB_TOKEN=ghp_xxx scrappy --github-scrape --search "torvalds" --out emails.csv

# Skip MX verification (DNS unavailable, or faster bulk runs)
scrappy --sites remoteok --search "golang" --verify-concurrency 0
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `SCRAPPY_PROXIES` | Comma-separated SOCKS5/HTTP proxy URLs (lowest precedence) |
| `SCRAPPY_LOG_LEVEL` | Default log level: `DEBUG`\|`INFO`\|`WARN`\|`ERROR` |
| `SCRAPPY_INDEED_API_KEY` | Indeed API key (paid) |
| `SCRAPPY_DICE_API_KEY` | Dice API key |
| `SCRAPPY_GREENHOUSE_SEEDS` | Company slugs for Greenhouse |
| `SCRAPPY_ATS_MAX_SEEDS` | Max ATS company seeds per provider (default: 20) |
| `SCRAPPY_INDEED_CO` | Indeed company override |
| `SCRAPPY_PROXY_ROTATE_EVERY_N` | Rotate proxy every N requests |
| `SCRAPPY_PROXY_STICKY_WINDOW_N` | Proxy stickiness window |
| `GITHUB_TOKEN` | GitHub API token (for `--github-scrape`) |
| `ADZUNA_APP_ID` / `ADZUNA_APP_KEY` | Adzuna API credentials |
| `CAREERJET_AFFID` | Careerjet affiliate ID |
| `INFOJOBS_CLIENT_ID` / `INFOJOBS_CLIENT_SECRET` | InfoJobs API credentials |
| `FINDWORK_API_KEY` | Findwork API key |
| `ARBEITSAGENTUR_API_KEY` | Arbeitsagentur API key |

Precedence: `--proxy` CLI flag > `config.toml` `proxy:` field > `SCRAPPY_PROXIES` env.
See `.env.example` for the full list.
