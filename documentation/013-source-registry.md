# Source registry

src/sources/mod.rs exposes manifest.rs, registry.rs, custom.rs, status.rs, and
status_doc.rs. registry::load walks src/sources/manifests/<family>/ recursively,
with one directory each for feeds_api, ats, html_static, html_rendered, and
keyed. list-sources merges manifests from the documented user config directory
$HOME/.config/scrappy/sources, listed by custom::directory. A custom manifest
uses the built-in parser kinds and cannot shadow a built-in source id.

documentation/005-status.md is generated from the registry by
`scrappy list-sources --generate-status`. Each source has a behavior document
in documentation/scrapers/<source_id>.md. Manifest validation and parity rules
are specified in
openspec/changes/oculus-foundation/specs/source-registry/spec.md.
