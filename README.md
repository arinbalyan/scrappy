# scrappy

A small installable Rust CLI that scrapes job sources into deterministic,
machine-readable output. NDJSON is the default stream; CSV is available.

See documentation/001-quickstart.md to begin, documentation/003-cli-reference.md
for commands, and documentation/006-architecture.md for the module layout.

## License

scrappy is free software licensed under the GNU General Public License,
version 3 or (at your option) any later version. You may redistribute it and
modify it under those terms. Distributed modifications must keep the copyright
notices intact, state the changes, and remain under the same license. See
LICENSE for the full text and MODIFICATIONS.md for the change-marking policy.

Copyright (c) 2026 Arin Balyan.

## Quality gate

Run cargo fmt --check, cargo clippy --all-targets -- -D warnings,
cargo test --locked, and cargo doc --no-deps before committing.
