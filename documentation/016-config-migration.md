# Config migration

The new CLI separates scraping from inspection and repair. Old flags that
described one large invocation become config keys, source selection, or
explicit subcommands. The runtime schema is available through `scrappy config
schema`, and every command in this document was checked against the current
binary.

## Command mapping

| Old invocation | New invocation | Notes |
| --- | --- | --- |
| `scrappy --search 'rust' --sites remoteok --format jsonl` | `scrappy run --source remoteok --format ndjson` with `search = "rust"` in config | `jsonl` is now `ndjson`. Search belongs in config or `SCRAPPY_SEARCH`. |
| `scrappy --results-wanted 100` | `scrappy run --source <id>` with `rates` and `concurrency` config | There is no global result cap; manifests define per-source rate and concurrency budgets. |
| `scrappy --location 'Berlin'` | `scrappy config set --key search --value '"rust Berlin"'` | Location is part of the search string for sources that support it. |
| `scrappy --out jobs.json` | Shell redirection, for example `scrappy run --format json > jobs.json` | Output stays on stdout and warnings stay on stderr so redirection is safe. |
| `scrappy --format csv` | `scrappy run --format csv` | CSV uses a stable header and the same field names as NDJSON. |
| `scrappy --proxy ...` | `SCRAPPY_PROXIES=... scrappy run` | Proxy health and selection run inside the fetchers. |
| `scrappy --config config.toml ...` | `scrappy run --config config.toml` | Repeat `--config` to layer multiple files; later files win. |
| `scrappy --version-json` | `scrappy version` | Prints a plain `scrappy 0.1.0` line, not JSON. See `scrappy version --help`. |
| `scrappy doctor --fix` | `scrappy doctor --fix` | Repairs are limited to config files and generated artifacts. |
| `scrappy --log-level DEBUG ...` | `SCRAPPY_LOG_LEVEL=debug scrappy run` | Levels are `error`, `warn`, `info`, `debug`, and `trace`. |
| Interactive wizard | `scrappy config init --path config.toml` | The template is commented and never overwrites without `--force`. |

## Config key mapping

| Old config or flag | New key | Type and default |
| --- | --- | --- |
| `defaults.search` or `--search` | `search` | string or array, default none |
| `defaults.sites` or `--sites` | `sources` | string or array, default none |
| `defaults.format` or `--format` | `format` | enum `ndjson`, `json`, or `csv`; default `ndjson` |
| `site_rps` or `--site-rps` | `rates` | map of source id to requests per minute |
| `--max-rps` | `rates` with a per-source entry | The global value is expressed through each source limit. |
| `--is-remote` | `is_remote` | tri-state boolean, default unset |
| `--job-type` | `job_type` | enum, default unset |
| `--min-score` | `min_score` | integer from 0 to 100, default unset |
| `--hours-old` | `hours_old` | integer hours, default unset |
| `--since` | `date` | `YYYY-MM-DD`, default unset |
| `--timeout` | `timeout` | integer seconds, default 30 |
| `--concurrency` | `concurrency` | integer, default 4 |
| `--memory-cap` | `memory_mb` | integer megabytes, default 512 |
| `log_level` or `--log-level` | `log_level` | enum, default info |

Keys absent from this table have no equivalent in the current runtime. The
old email verification, XLSX and Parquet exports, interactive wizard, GitHub
email discovery, dedup-by-company, salary normalization, and JSON pretty or
minify switches are outside the current contract. A source-specific setting
that the old binary read from a `[sources.<id>]` section is preserved by
`config set`, which leaves unknown sections untouched.

## Verify a migration

Run this sequence after moving an old config:

1. `scrappy config init --path config.toml` writes a commented template.
2. `scrappy config show --path config.toml` prints the resolved values with
   secrets redacted.
3. `scrappy config explain --key search` reports the field and a redacted
   value. It takes no `--config` or `--path` flag and reports the default
   layer, so use `scrappy config show --path config.toml` to check file values.
4. `scrappy validate --config config.toml` checks the configuration without
   fetching.
5. `scrappy doctor --format json` reports local setup as machine-readable
   output.
6. `scrappy list-sources --format json` lists registered sources and health.
7. `scrappy run --config config.toml --format ndjson` performs the scrape.

Every listed command exits before any request except `run`. A malformed
config exits with status 2 and a diagnostic on stderr.


## Secrets and schema migration

- Move credential values from committed config files to `SCRAPPY_<NAME>` variables or a local `secrets_file`.
- Remove credential values from old configs before sharing or committing them.
- Output schema version 2 is additive. Version 1 records remain readable, but new records carry source ids, salary fields, and typed apply routes.
