# Configuration

Typed configuration and layered resolution live in src/config/mod.rs and
src/config/file.rs. Values resolve from CLI, environment, project file,
user file, and built-in defaults, in that order. Config writes are atomic,
use mode 0600 for files and 0700 for newly created directories, and never
store credentials. See openspec/changes/oculus-foundation/specs/cli-config/spec.md.

`email_discovery` is an opt-in boolean, default false. It enables inferred
contact addresses discovered through a dedicated source manifest. Observed
addresses from job descriptions are always emitted with observed provenance.
The environment form is `SCRAPPY_EMAIL_DISCOVERY`. See
documentation/007-email-extraction.md and
openspec/changes/oculus-foundation/specs/security-privacy/spec.md.
