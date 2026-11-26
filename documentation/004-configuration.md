# Configuration

Typed configuration and layered resolution live in src/config.rs. Config
values remain optional until resolution, and secrets come from environment
variables or selected files. Field behavior is covered by
openspec/changes/oculus-foundation/specs/cli-config/spec.md.
