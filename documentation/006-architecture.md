# Architecture

src/main.rs is a thin entry point. src/lib.rs exposes cli, config, sources,
fetching, parsing, engine, output, security, checkpoint, and metrics modules.
Each subsystem is a directory with a mod.rs root. src/config holds schema and
resolution in mod.rs and file editing in file.rs. src/engine/contract.rs owns
the runtime bridge contract for streams, exit codes, and stdin run config.
Integration tests use directory targets under tests/cli_contract and
tests/bridge_contract, declared in Cargo.toml.
Fixtures live in tests/fixtures/<source_id>/ and behavior docs in
documentation/scrapers/<source_id>.md. Detailed decisions are recorded in
openspec/changes/oculus-foundation/design.md.
