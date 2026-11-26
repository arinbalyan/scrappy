# Architecture

src/main.rs is a thin entry point. src/lib.rs exposes cli, config, sources,
fetching, parsing, engine, output, security, checkpoint, and metrics modules.
Fixtures live in tests/fixtures/<source_id>/ and behavior docs in
documentation/scrapers/<source_id>.md. Detailed decisions are recorded in
openspec/changes/oculus-foundation/design.md.
