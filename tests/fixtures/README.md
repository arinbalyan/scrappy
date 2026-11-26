# Fixtures

Each source has one fixture set at `tests/fixtures/<source_id>/`. A complete set contains six files:

- `tests/fixtures/<source_id>/request.json`: frozen request derived from the template.
- `tests/fixtures/<source_id>/request.template.json`: `method`, `url` template, and `headers`.
- `tests/fixtures/<source_id>/response.body`: saved upstream response body.
- `tests/fixtures/<source_id>/headers.json`: saved response headers and status.
- `tests/fixtures/<source_id>/frozen.json`: `fetched_at` timestamp and `substitutions` values.
- `tests/fixtures/<source_id>/golden.ndjson`: expected normalized output.

Each `{name}` token in the template URL or body is replaced by the `name` entry in `frozen.json` `substitutions`. Literal braces use doubled braces, so `{{` becomes `{` and `}}` becomes `}`; this lets bodies carry JSON objects such as GraphQL payloads. `request.json` stores the rendered URL and body and keeps the template method and headers. The contract was recorded on 2026-11-26. See `documentation/014-testing.md`.
