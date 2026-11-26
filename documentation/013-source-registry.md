# Source registry

src/sources/mod.rs exposes manifest.rs and registry.rs. The registry loads
manifests from src/sources/manifests/<family>/, with one directory each for
feeds_api, ats, html_static, html_rendered, and keyed. Each source has a
behavior document in documentation/scrapers/<source_id>.md. Manifest
validation and custom sources are specified in
openspec/changes/oculus-foundation/specs/source-registry/spec.md.
