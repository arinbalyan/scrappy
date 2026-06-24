# Configuration

scrappy uses a TOML config file for per-site search terms, locations, and defaults. The config is personal and not committed to git.

## Config file location

1. `config.toml` in the current working directory
2. `~/.scrappy/config.toml` (user-wide defaults)

## Defaults section

```toml
[defaults]
  search = "software engineer"
  location = ["Remote", "United States", "India"]
  is_remote = true
  results_wanted = 100000
  format = "csv"
  out = "./data/jobs.csv"
```

## Per-site configuration

Override search terms and locations per site:

```toml
[sites]
  [sites.indeed]
    search = ["\"Software Engineer\" OR \"Full Stack Developer\""]
    location = ["Remote"]

  [sites.remoteok]
    search = ["software engineer", "full stack", "backend"]
    location = "Remote"

  [sites.linkedin]
    search = ["\"Software Engineer\" OR \"SDE\"", "\"AI Engineer\" OR \"ML Engineer\""]
```

## Environment variables

API keys and secrets go in `.env` (not config.toml):

```bash
SCRAPPY_INDEED_API_KEY=your_key
ADZUNA_APP_ID=your_id
ADZUNA_APP_KEY=your_key
```

## Config precedence

1. CLI flags (highest priority)
2. `config.toml` in current directory
3. `~/.scrappy/config.toml`

Proxy precedence:
1. `--proxy` CLI flag
2. `config.toml` proxy field
3. `SCRAPPY_PROXIES` env var
