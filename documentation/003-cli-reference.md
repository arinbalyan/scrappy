# CLI reference

The command tree lives in src/cli/mod.rs. Every subcommand supports `--help` and
`--version`, and help output includes a copyable example. The current commands
are `run`, `validate`, `config` with its seven actions, `list-sources`,
`doctor` with `--fix`, `completion`, and `version`.

| Command | Options and behavior |
| --- | --- |
| `run` | `--source <id>` (repeatable), `--format ndjson\|json\|csv\|table`, `--stdin-config`, plus the shared flags. `table` is accepted by the parser but rejected by `run`; the default is `ndjson`. |
| `validate` | Resolve and validate config without fetching; prints `ok`. |
| `config` | Seven actions: `init --path <path> [--force]`, `show --path <path>`, `set --path <path> --key <key> --value <json>`, `unset --path <path> --key <key>`, `edit --path <path>`, `explain --key <key>`, and `schema [--enum <field>]`. |
| `list-sources` | `--format table\|json` and `--generate-status`, which rewrites documentation/005-status.md. |
| `doctor` | `--format text\|json` and `--fix`. |
| `completion` | Generates scripts for bash, zsh, fish, powershell, and elvish. |
| `version` | Prints the package version without loading config. |

Shared flags are `--config <path>` (repeatable; later files win) and
`--no-default-config`, which disables discovered files and environment values.
They apply to `run`, `validate`, `config explain`, `list-sources`, and
`doctor`, but `config explain` reports the default layer because it takes no
config file. With `run --stdin-config`, the stdin JSON is the run-config layer
and explicit `--config` files are ignored.

Config resolution order is user $HOME/.config/scrappy/config.toml, project
`config.toml`, `SCRAPPY_*` environment values, repeated `--config` files in
order, then CLI values. See documentation/004-configuration.md. The command
contract is in
openspec/changes/oculus-foundation/specs/cli-config/spec.md.

## Run progress and exit codes

`run` writes machine-readable progress events as NDJSON lines on stderr. Each
event has an `event` discriminator, a `run_id`, an optional `source_id`, and
`emitted` and `skipped` counters. A terminal event reports the final counts.
Human-readable diagnostics are separate non-JSON stderr lines, so a consumer
can distinguish them by attempting JSON parsing. stdout holds only the result
in the requested format. `run --stdin-config` always emits NDJSON on stdout.

Published exit codes are 0 for clean success, 1 for internal failure, 2 for
invalid usage or configuration, 3 for partial success with per-source
failures, 4 when challenges need attention, and 5 for cancellation. stdout
stays valid NDJSON for codes 0, 3, 4, and 5. The engine applies rate limits,
concurrency caps, result caps, robots policy, challenge backoff, and bounded
async retries before emitting each source.
