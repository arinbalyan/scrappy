# Configuration

Typed configuration and layered resolution live in src/config.rs and
src/config_file.rs. Values resolve from CLI, environment, project file,
user file, and built-in defaults, in that order. Config writes are atomic,
use mode 0600 for files and 0700 for newly created directories, and never
store credentials. See openspec/changes/oculus-foundation/specs/cli-config/spec.md.
