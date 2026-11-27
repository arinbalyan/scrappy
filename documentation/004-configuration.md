# Configuration

Typed configuration and layered resolution live in src/config/mod.rs and
src/config/file.rs. Values resolve in increasing precedence from built-in
defaults, user $HOME/.config/scrappy/config.toml, project config.toml,
`SCRAPPY_*` environment values, repeated `--config` files in command order,
and CLI values. `--no-default-config` disables the discovered files and environment
layer. Config writes are atomic, use mode 0600 for files and 0700 for newly
created directories, and never store credentials. The runtime schema is
printed by `scrappy config schema` and listed in the table below. See
openspec/changes/oculus-foundation/specs/cli-config/spec.md.

| Key | Type | Default |
| --- | --- | --- |
| `search` | string or array | none |
| `sources` | string or array | none |
| `format` | `ndjson`, `json`, `csv` | `ndjson` |
| `rates` | map of source id to requests per minute | none |
| `concurrency` | integer | 4 |
| `timeout` | integer seconds | 30 |
| `memory_mb` | integer megabytes | 512 |
| `log_level` | `error`, `warn`, `info`, `debug`, `trace` | `info` |
| `is_remote` | boolean | unset |
| `job_type` | `fulltime`, `parttime`, `contract`, `internship` | unset |
| `min_score` | integer 0-100 | unset |
| `hours_old` | integer hours | unset |
| `date` | `YYYY-MM-DD` | unset |
| `email_discovery` | boolean | false |

Unknown keys are rejected. Environment names are `SCRAPPY_` plus the upper
case key. `SCRAPPY_SEARCH` and `SCRAPPY_SOURCES` accept a JSON array or a
plain string, and `SCRAPPY_RATES` accepts a JSON object.

`email_discovery` is an opt-in boolean, default false. It enables inferred
contact addresses discovered through a dedicated source manifest. Observed
addresses from job descriptions are always emitted with observed provenance.
The environment form is `SCRAPPY_EMAIL_DISCOVERY`. See
documentation/007-email-extraction.md and
openspec/changes/oculus-foundation/specs/security-privacy/spec.md.

For `run`, a config format is used when set, even if `--format` is also
given; the CLI format applies only when config has no format. The `table`
format is accepted by `--format` for listing but rejected for run output.
With
`run --stdin-config`, the stdin document is the highest-precedence run-config
layer, so its format wins and explicit `--config` flags are ignored.
