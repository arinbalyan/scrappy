# CLI reference

The command tree lives in src/cli/mod.rs. Every subcommand supports `--help` and
`--version`, and help output includes a copyable example. The current commands
are `run`, `validate`, `config` with its seven actions, `list-sources`,
`doctor` with `--fix`, `completion`, and `version`.

`completion` generates scripts for bash, zsh, fish, powershell, and elvish.
The full contract is in
openspec/changes/oculus-foundation/specs/cli-config/spec.md.
