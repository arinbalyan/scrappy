//! Custom source manifests from the documented user config directory.
//!
//! The custom-source contract is specified in
//! openspec/changes/oculus-foundation/specs/source-registry/spec.md.

// Rust guideline compliant 2026-10-09

use std::path::{Path, PathBuf};

use crate::sources::manifest::{Manifest, parse};
use crate::sources::registry::{self, Registry};

/// Directory below the user config home that holds custom manifests.
pub const CUSTOM_DIR: &str = "scrappy/sources";

/// Return the custom-source directory for the current user.
#[must_use]
pub fn directory() -> PathBuf {
    if let Some(home) = std::env::var_os("HOME") {
        PathBuf::from(home).join(".config").join(CUSTOM_DIR)
    } else {
        PathBuf::from(CUSTOM_DIR)
    }
}

/// Load custom manifests and merge them into a built-in registry.
///
/// # Errors
/// Returns `RegistryError` for unreadable files, invalid manifests, or a
/// custom source that shadows a built-in identifier.
pub fn load(
    root: &Path,
    custom: &Path,
) -> Result<Registry, crate::sources::registry::RegistryError> {
    let mut registry = crate::sources::registry::load(root)?;
    let Ok(files) = registry::manifest_files(custom) else {
        return Ok(registry);
    };
    for path in &files {
        let text = std::fs::read_to_string(path).map_err(|error| {
            crate::sources::registry::RegistryError {
                message: format!("cannot read {}: {error}", path.display()),
            }
        })?;
        let manifest: Manifest =
            parse(&text).map_err(|error| crate::sources::registry::RegistryError {
                message: format!("{}: {}", path.display(), error.message),
            })?;
        if registry.get(&manifest.id).is_some() {
            return Err(crate::sources::registry::RegistryError {
                message: format!("custom source shadows built-in: {}", manifest.id),
            });
        }
        registry.insert(manifest);
    }
    Ok(registry)
}

#[cfg(test)]
mod tests {
    use super::*;

    const CUSTOM: &str = include_str!("manifest_sample.toml");

    fn temp(name: &str) -> PathBuf {
        std::env::temp_dir().join(format!("scrappy-custom-{name}-{}", std::process::id()))
    }

    #[test]
    fn loads_custom_manifest() {
        let builtins = temp("builtins");
        let custom = temp("custom");
        std::fs::create_dir_all(&builtins).expect("builtins created");
        std::fs::create_dir_all(&custom).expect("custom created");
        std::fs::write(custom.join("sample.toml"), CUSTOM).expect("manifest written");
        let registry = load(&builtins, &custom).expect("registry loaded");
        assert_eq!(registry.len(), 1);
        std::fs::remove_dir_all(builtins).expect("cleanup");
        std::fs::remove_dir_all(custom).expect("cleanup");
    }

    #[test]
    fn rejects_shadowing_builtin() {
        let builtins = temp("shadow-builtins");
        let custom = temp("shadow-custom");
        std::fs::create_dir_all(&builtins).expect("builtins created");
        std::fs::create_dir_all(&custom).expect("custom created");
        std::fs::write(builtins.join("sample.toml"), CUSTOM).expect("manifest written");
        std::fs::write(custom.join("sample.toml"), CUSTOM).expect("manifest written");
        let error = load(&builtins, &custom).unwrap_err();
        assert!(error.message.contains("shadows"));
        std::fs::remove_dir_all(builtins).expect("cleanup");
        std::fs::remove_dir_all(custom).expect("cleanup");
    }
}
