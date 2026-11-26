# Testing

Normal tests are offline and replay fixtures through real parsers. Live
validation is opt-in and ignored by default. Before writing or reviewing any
.rs file, load .codex/skills/ms-rust/SKILL.md and only the relevant guideline
files in .codex/skills/ms-rust/. Add the compliance marker at the top of every
compliant Rust file. Run cargo fmt, cargo clippy --all-targets -- -D warnings,
and cargo doc --no-deps as the baseline gate. The parity check in
tests/docs_parity.rs verifies that documented paths exist.
