# Deduplication

src/engine/dedup.rs performs canonical URL and content-hash deduplication.
Canonical URLs use a lower-case scheme and host, drop URL fragments and common
tracking parameters, and sort remaining query keys. A content hash is computed
from the canonical URL plus normalized title, company, and description fields.
Dedup keeps the first record for each key and can be disabled by callers.
URL dedup stays separate from title-company dedup. The configuration contract
is in openspec/changes/oculus-foundation/specs/scrape-engine/spec.md.
