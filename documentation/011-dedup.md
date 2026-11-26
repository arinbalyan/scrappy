# Deduplication

src/engine.rs performs canonical URL and content-hash deduplication, with URL
dedup separate from title-company dedup. The configuration contract is in
openspec/changes/oculus-foundation/specs/scrape-engine/spec.md.
