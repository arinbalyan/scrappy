# Configuration File

scrappy uses a YAML configuration file to store persistent defaults and per-site overrides. This lets you set up your preferences once and omit repetitive flags on every invocation.

## File location (auto-detect order)

When you run `scrappy`, it searches for config in this order:

| Priority | Path | Purpose |
|----------|------|---------|
| 1st | `./config.yaml` (current working directory) | Project-level config |
| 2nd | `~/.scrappy/config.yaml` | User-wide defaults |

The `.env` file is loaded from the **same directory** as the chosen config file, falling back to `~/.scrappy/.env`.

```bash
# Both load ./config.yaml (and ./.env if it exists)
cd /home/user/project && scrappy

# Loads ~/.scrappy/config.yaml
cd /tmp && scrappy
```

Override with `--config`:

```bash
scrappy --config /etc/scrappy/production.yaml
```

## Structure

```yaml
defaults:
  search: AI Engineer
  location: Remote
  results_wanted: 500
  out: /data/jobs.jsonl
  format: jsonl
  memory_cap: 512MB
  proxy: socks5://user:pass@proxy:1080
  min_score: 60
  is_remote: false
  remote_only: false
  job_type: ""
  dedup: true
  dedup_by_company: false
  max_rps: 0
  site_rps: ""

sites:
  site_name:
    search: "search term"
    location: "location"
    is_remote: true
```

## `defaults:` block

| Field | Type | Description |
|-------|------|-------------|
| `search` | string or list | Default search term(s). List form enables multi-value cartesian product. |
| `location` | string or list | Default location(s). List form enables multi-value cartesian product. |
| `results_wanted` | int | Maximum results across all sites. |
| `out` | string | Output file path. Empty = stdout. |
| `format` | string | Output format: `jsonl`, `csv`, `xlsx`, `parquet`. |
| `memory_cap` | string | Memory budget: `512MB`, `1GB`, or a plain number as MB. `0` = unlimited. |
| `proxy` | string | Comma-separated proxy URLs (lower priority than `--proxy` CLI flag, higher than `SCRAPPY_PROXIES` env var). |
| `min_score` | int | Minimum quality score (0-100). Jobs below this threshold are dropped before export. |
| `is_remote` | bool | Only jobs flagged as remote (location-independent filter). |
| `remote_only` | bool | Only truly remote jobs (no location filter applied). |
| `job_type` | string | Filter: `fulltime`, `parttime`, `contract`, `internship`. |
| `dedup` | bool | Drop duplicate job URLs across sites (default: true). |
| `dedup_by_company` | bool | Keep only one posting per company across the entire result set. |
| `max_rps` | int | Global maximum requests per second. Clamped between 2 and 16. |
| `site_rps` | string | Per-site RPS overrides. Format: `site:rps,site:rps` (e.g. `linkedin:1,indeed:10`). |

### `multiString` format

`search` and `location` accept either a single string or a YAML list. This type is called `multiString` internally.

**Single string:**

```yaml
defaults:
  search: AI Engineer
```

**List (generates cartesian product):**

```yaml
defaults:
  search:
    - AI Engineer
    - Software Engineer
    - Rust Developer
  location:
    - Remote
    - New York
    - Hyderabad
```

Each site iterates over every `(search, location)` pair -- 3 x 3 = 9 passes per site.

See [007-Multi-Value.md](007-Multi-Value.md) for details.

## `sites:` block

Define per-site overrides that replace the global defaults for a specific board:

```yaml
sites:
  indeed:
    search:
      - '"AI Engineer" OR "ML Engineer"'
      - '"GTM Engineer" OR "Forward Deployed Engineer"'
    location: Remote
    is_remote: true
  linkedin:
    search: "AI Engineer OR Machine Learning Engineer"
    location: Remote
  mycareersfuture:
    search: "AI Engineer"
    location: Singapore
```

| Field | Type | Description |
|-------|------|-------------|
| `search` | string or list | Replaces `defaults.search` for this site only. |
| `location` | string or list | Replaces `defaults.location` for this site only. |
| `country` | string | Country override (e.g. `germany`, `uk`, `india`). Passed to scrapers that support per-country endpoints (Indeed, Glassdoor). |
| `is_remote` | bool | Overrides the global `is_remote` for this site. |

### Per-site override rules

1. If a site has its own `search` in the `sites:` block, the global `defaults.search` is **ignored** for that site.
2. Same for `location`.
3. Sites without a `sites:` entry use the global defaults.
4. CLI flags (`--search`, `--location`) **always** take precedence over config.

## Complete example

```yaml
defaults:
  search: AI Engineer
  location: Remote
  results_wanted: 100000
  out: /home/user/data/jobs.csv
  format: csv
  memory_cap: 0
  proxy: socks5://user:pass@proxy:1080
  min_score: 60
  is_remote: true
  remote_only: false
  job_type: fulltime
  dedup: true
  dedup_by_company: false
  max_rps: 10
  site_rps: "linkedin:1,indeed:5"

sites:
  indeed:
    search:
      - '"AI Engineer" OR "ML Engineer" OR "LLM Engineer"'
      - '"GTM Engineer" OR "Forward Deployed Engineer"'
      - '"AI Agent Engineer" OR "AI Product Engineer"'
    location: Remote
  linkedin:
    search:
      - '"AI Engineer" OR "Machine Learning Engineer" OR "LLM Engineer"'
      - '"GTM Engineer" OR "Forward Deployed Engineer"'
      - '"AI Agent Engineer" OR "AI Product Engineer"'
      - '"Applied Scientist" OR "Research Scientist"'
    location: Remote
  indeed:
    search:
      - '"AI Engineer" OR "ML Engineer" OR "LLM Engineer"'
      - '"GTM Engineer" OR "Forward Deployed Engineer"'
      - '"AI Agent Engineer" OR "AI/ML Engineer"'
    location: Remote
  remoteok:
    search:
      - ai engineer
      - machine learning
      - llm engineer
      - forward deployed
      - gtm engineer
    location: Remote
  remotive:
    search:
      - machine learning engineer
      - ai engineer
      - llm engineer
      - forward deployed
    location: Remote
  ycjobs:
    search:
      - ai agent engineer
      - ai engineer
      - ai developer
      - llm engineer
      - forward deployed engineer
    location: Remote
  reed:
    search:
      - '"AI Engineer" OR "Machine Learning Engineer"'
      - '"GTM Engineer" OR "Forward Deployed Engineer"'
    location: United Kingdom
      - llm engineer
    location: India
  mycareersfuture:
    search:
      - '"AI Engineer" OR "Machine Learning Engineer"'
      - '"GTM Engineer" OR "Forward Deployed Engineer"'
    location: Singapore
  jobsdb:
    search:
      - '"AI Engineer" OR "Machine Learning Engineer"'
      - '"GTM Engineer" OR "Forward Deployed Engineer"'
    location: Singapore
  ukvisajobs:
    search:
      - '"AI Engineer" OR "Machine Learning Engineer"'
      - '"GTM Engineer" OR "Software Engineer"'
    location: United Kingdom
  internshala:
    search:
      - ai engineer
      - machine learning
      - llm engineer
    location: India
```

## Tips

### Use OR operators

Many sites support boolean operators in the search string:

```yaml
defaults:
  search: '"AI Engineer" OR "ML Engineer" OR "LLM Engineer"'
```

Quoting each term preserves the phrase for sites that pass the string directly to their search API. The quotes are sent as part of the HTTP request -- they are **not** consumed by scrappy's YAML parser.

### Site-specific searches

Set different searches for each site based on what that board does well:

```yaml
sites:
  linkedin:
    search: '"AI Engineer" OR "Machine Learning Engineer" OR "Applied Scientist"'
  remoteok:
    search: ai engineer   # simpler syntax, no OR needed
  findwork:
    search: '"AI Engineer" OR "ML Engineer"'
    location: Remote
```

### Save from interactive mode

After completing a scrape in interactive mode, answer `y` to "Save these settings to ~/.scrappy/config.yaml?" -- the wizard writes your current values as a config file automatically.

The setup wizard now also prompts for:
- **Site selection** -- saved to `config.yaml` as the `sites:` block
- **API key prompts** for all 5 key-required sites: InfoJobs (client ID + secret), Arbeitsagentur (API key), Adzuna (app ID + key), Careerjet (affiliate ID), and Findwork (API key)

See [005-Interactive-Mode.md](005-Interactive-Mode.md) for details.

### `.dockerignore` for Docker builds

When building the Docker image, a `.dockerignore` file excludes `tmp/`, `tests/`,
`docs/`, `*.md`, `config.yaml`, and other non-essential files from the build
context to keep images small and builds fast.
