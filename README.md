# scrappy

A small installable Rust CLI that scrapes job sources into deterministic,
machine-readable output. NDJSON is the default stream; CSV is available.

See documentation/001-quickstart.md to begin, documentation/003-cli-reference.md
for commands, and documentation/006-architecture.md for the module layout.

## Quality gate

Run cargo fmt --check, cargo clippy --all-targets -- -D warnings,
cargo test --locked, and cargo doc --no-deps before committing.
