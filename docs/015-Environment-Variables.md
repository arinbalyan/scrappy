# Environment Variables

`scrappy` loads configuration from environment variables, `.env` files, and `config.yaml` -- in that precedence order.

## Precedence

```
CLI flag > env var > config.yaml > default
```

`.env` files are loaded automatically from two locations (first found wins):

```
config.yaml directory/.env     # beside config.yaml
~/.scrappy/.env                # user-wide defaults
```

The interactive wizard can write API keys to `~/.scrappy/.env` for you. An `.env.example` file is provided in the repository root.

## Reference

### Required for 6 sites

| Variable | Site | Get it at |
|----------|------|-----------|
| `ADZUNA_APP_ID`, `ADZUNA_APP_KEY` | Adzuna | https://developer.adzuna.com/ |
| `CAREERJET_AFFID` | Careerjet | https://www.careerjet.com/partners/ |
| `INFOJOBS_CLIENT_ID`, `INFOJOBS_CLIENT_SECRET` | InfoJobs | https://developer.infojobs.net/ |
| `FINDWORK_API_KEY` | Findwork | https://findwork.dev/developers/ |
| `ARBEITSAGENTUR_API_KEY` | Arbeitsagentur | https://rest.arbeitsagentur.de/ |
| `WEB3CAREER_API_TOKEN` | Web3Career | https://web3.career/web3-jobs-api |

When a required env var is missing, the engine skips that site with a WARN message instead of failing the run.

### Proxy and network

| Variable | Default | Description |
|----------|---------|-------------|
| `SCRAPPY_PROXIES` | -- | Comma-separated SOCKS5/HTTP proxy URLs (lowest priority; overridden by `--proxy` CLI flag and `config.yaml proxy` field) |
| `SCRAPPY_PROXY_ROTATE_EVERY_N` | -- | Rotate to next proxy every N requests (0 = disabled) |
| `SCRAPPY_PROXY_STICKY_WINDOW_N` | 20 | Minimum requests before rotating away from current proxy |
| `SCRAPPY_LOG_LEVEL` | `INFO` | Log verbosity: `DEBUG`, `INFO`, `WARN`, `ERROR` |

**Proxy precedence**: `--proxy` CLI flag > `config.yaml` `proxy:` field > `SCRAPPY_PROXIES` env var

### Site-specific overrides

| Variable | Default | Description |
|----------|---------|-------------|
| `SCRAPPY_GREENHOUSE_SEEDS` | -- | Comma-separated company names for Greenhouse board discovery |
| `SCRAPPY_INDEED_API_KEY` | -- | Indeed partner API key (paid; optional, falls back to public GraphQL) |
| `SCRAPPY_INDEED_CO` | -- | Indeed country code override (e.g. `us`, `gb`, `de`) |

Additional seed variables for job board discovery:

| Variable | Description |
|----------|-------------|
| `SCRAPPY_LEVER_SEEDS` | Comma-separated company names for Lever board discovery |
| `SCRAPPY_WORKABLE_SEEDS` | Comma-separated company names for Workable board discovery |
| `SCRAPPY_WORKDAY_SEEDS` | Comma-separated company names for Workday board discovery |

## `.env` file format

```
ADZUNA_APP_ID=your_id
ADZUNA_APP_KEY=your_key
CAREERJET_AFFID=your_affiliate_id
SCRAPPY_PROXIES=socks5://user:pass@host:1080
SCRAPPY_LOG_LEVEL=DEBUG
```

Lines starting with `#` are ignored. Existing env vars are never overwritten by `.env` files.

## Verification

```bash
# Check which env vars are set
env | grep -E '^(ADZUNA_|CAREERJET_|INFOJOBS_|FINDWORK_|ARBEITSAGENTUR_|WEB3CAREER_|SCRAPPY_)'

# Set a variable for a single run
ADZUNA_APP_ID=xxx scrappy --sites adzuna --search "engineer" --results-wanted 100
```
