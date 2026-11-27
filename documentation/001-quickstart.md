# Quickstart

Build from the scrappy root with `cargo build --release --locked`. Run a source
with `scrappy run --source remoteok --format ndjson`, and inspect setup with
`scrappy doctor --format json`. The CLI is implemented in src/main.rs and
src/cli/mod.rs; the full command tree is in documentation/003-cli-reference.md.
