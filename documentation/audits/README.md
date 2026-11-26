# Audit fixtures

This folder records how the dependency audit gate is verified without adding
a vulnerable crate to `Cargo.lock`. The CI audit step runs `cargo audit` and
`cargo deny check`. To verify failure behavior, run `cargo audit` against a
throwaway manifest that depends on a crate with a published advisory. Keep the
throwaway manifest outside the repository tree and delete it after the check.
