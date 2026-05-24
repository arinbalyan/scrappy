# Interactive Mode

Run `scrappy` with no arguments on a terminal to enter the interactive wizard. It guides you through setting up a scrape job step by step -- ideal for first-time users or one-off searches.

## How it starts

When you run:

```bash
scrappy
```

1. scrappy checks whether stdin is a TTY (a real terminal).
2. If yes, and no flags were passed, interactive mode activates automatically.
3. If stdin is piped (e.g. `echo "" | scrappy`), interactive mode is skipped even when `--interactive` is set.
4. A logo and version print, followed by an interactive prompt with box-drawn sections.

## Config auto-loading

Before the wizard starts, scrappy loads an existing config file to pre-fill defaults:

| Check Order | Path | Purpose |
|-------------|------|---------|
| 1st | `./config.yaml` (current directory) | Project-level config |
| 2nd | `~/.scrappy/config.yaml` | User-wide defaults |

If found, the `defaults:` block populates wizard prompts (search, location, results-wanted, format, out, etc.). If not found, empty defaults are used.

A `.env` file beside the config is also loaded automatically for API keys.

## Wizard steps

Each prompt shows a **default value in brackets**. Press Enter to accept the default or type a new value.

### Step 1 -- Search term

```
? Search term (e.g. "software engineer") [AI Engineer]:
```

Accepts comma-separated values for multi-value cartesian product:

```
AI Engineer,Software Engineer,Rust Developer
```

### Step 2 -- Location

```
? Location (e.g. "San Francisco, CA" or "Remote") [Remote]:
```

Also accepts comma-separated:

```
Remote,New York,Hyderabad
```

### Step 3 -- Sites

```
? Sites (comma-separated, empty=all) [linkedin,indeed]:
```

Leave empty to scrape all 62+ sites. Enter specific names to limit.

### Step 4 -- Results wanted

```
? Results wanted [500]:
```

Maximum total results across all sites.

### Step 5 -- Output format

```
? Format (jsonl/csv/xlsx/parquet) [jsonl]:
```

### Step 6 -- Output path

```
? Output path (empty = stdout) [/data/jobs.jsonl]:
```

Empty = results printed as pretty-printed JSON to stdout.

### Step 7 -- Remote filter

```
? Only show remote jobs? (y/n) [n]:
```

### Step 8 -- Job type

```
? Job type (fulltime/parttime/contract/internship/any) [any]:
```

### Step 9 -- Memory cap

```
? Memory cap (e.g. 512MB, 1GB, 0=unlimited) [0]:
```

Caps internal concurrency to stay within budget. See [016-Memory-Management.md](016-Memory-Management.md).

### Step 10 -- Minimum quality score

```
? Minimum quality score (0-100, 0=off) [0]:
```

Jobs below this threshold are dropped before export. See [011-Quality.md](011-Quality.md).

## Post-scrape flow

### Progress output

During scraping, scrappy writes status to stderr:

```
[OK] Scraped 142 jobs in 34s
```

### Save config prompt

After results are written, scrappy asks:

```
? Save these settings to ~/.scrappy/config.yaml? (y/N):
```

If you answer `y` or `yes`, the current wizard values are written to `~/.scrappy/config.yaml`. This makes them the defaults for future runs. See [008-Configuration.md](008-Configuration.md).

### API key wizard

Next, scrappy checks which of the 5 key-required sites are missing credentials:

```
Some sites require API keys to function:
[ERROR] adzuna          ADZUNA_APP_ID, ADZUNA_APP_KEY
[OK]    careerjet       CAREERJET_AFFID
[ERROR] infojobs        INFOJOBS_CLIENT_ID, INFOJOBS_CLIENT_SECRET
[ERROR] findwork        FINDWORK_API_KEY
[ERROR] arbeitsagentur  ARBEITSAGENTUR_API_KEY

? Configure API keys now? (y/N):
```

Answer `y` to enter each key interactively. Values are saved to `~/.scrappy/.env` (with `0600` permissions) and loaded automatically on subsequent runs.

> ⚠️ **Security**: Config files (including config.yaml and .env) are saved with `0600` permissions to protect proxy credentials and API keys. Review your config before sharing output or committing to version control.

## Tips for first use

1. **Start simple**: run `scrappy`, pick one site (e.g. `remoteok`) and a broad search term.
2. **Save your config** when prompted -- future runs will remember your settings.
3. **Check the API key wizard** -- many sites work without keys, but 5 require them for full results.
4. **Use `--non-interactive` in scripts** -- the wizard blocks on stdin and will hang in cron.
5. **Memory cap** -- start with `0` (unlimited), add `512MB` only if you see OOM kills.

## Force interactive / non-interactive

```bash
# Force interactive even when piping
scrappy --interactive

# Force non-interactive on a TTY (for scripts)
scrappy --non-interactive

# Auto-detect (default): enable when TTY and no flags
scrappy
```

The auto-detect logic:

| stdin is a TTY | Flags provided | Mode |
|:---:|:---:|:---:|
| Yes | Yes | Non-interactive (flags take precedence) |
| Yes | No | Interactive (wizard shown) |
| No | Any | Non-interactive (piped input) |

See [006-NonInteractive-Mode.md](006-NonInteractive-Mode.md) for details on non-interactive mode.
