//! Source manifest registry.
//!
//! See documentation/013-source-registry.md.

// Rust guideline compliant 2026-10-09

use std::collections::BTreeMap;
use std::path::Path;

use crate::sources::manifest::{Manifest, ManifestError, parse};

/// Error produced while loading or validating a registry.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct RegistryError {
    /// Human-readable diagnostic.
    pub message: String,
}

impl std::fmt::Display for RegistryError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(&self.message)
    }
}

impl std::error::Error for RegistryError {}

/// Manifests indexed by stable source identifier.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct Registry {
    entries: BTreeMap<String, Manifest>,
}

impl Registry {
    /// Create an empty registry.
    #[must_use]
    pub fn new() -> Self {
        Self::default()
    }

    /// Return the manifest for a source identifier.
    #[must_use]
    pub fn get(&self, id: &str) -> Option<&Manifest> {
        self.entries.get(id)
    }

    /// Return every loaded manifest in identifier order.
    pub fn iter(&self) -> impl Iterator<Item = &Manifest> {
        self.entries.values()
    }

    /// Return the number of loaded manifests.
    #[must_use]
    pub fn len(&self) -> usize {
        self.entries.len()
    }

    /// Return whether the registry is empty.
    #[must_use]
    pub fn is_empty(&self) -> bool {
        self.entries.is_empty()
    }
}

/// Load manifests from a directory tree.
///
/// # Errors
/// Returns `RegistryError` for unreadable files, invalid manifests, or
/// duplicate identifiers across the tree.
pub fn load(root: &Path) -> Result<Registry, RegistryError> {
    let mut registry = Registry::new();
    let mut files: Vec<_> = std::fs::read_dir(root)
        .map_err(|error| RegistryError {
            message: format!("cannot read {}: {error}", root.display()),
        })?
        .filter_map(Result::ok)
        .map(|entry| entry.path())
        .filter(|path| path.extension().is_some_and(|ext| ext == "toml"))
        .collect();
    files.sort();
    for path in files {
        let text = std::fs::read_to_string(&path).map_err(|error| RegistryError {
            message: format!("cannot read {}: {error}", path.display()),
        })?;
        let manifest = parse(&text).map_err(|error: ManifestError| RegistryError {
            message: format!("{}: {}", path.display(), error.message),
        })?;
        if registry.get(&manifest.id).is_some() {
            return Err(RegistryError {
                message: format!("duplicate source id: {}", manifest.id),
            });
        }
        registry.entries.insert(manifest.id.clone(), manifest);
    }
    Ok(registry)
}

#[cfg(test)]
mod tests {
    use super::*;

    const VALID: &str = r#"
version = 1
id = "sample"
name = "Sample"
family = "feeds_api"
request = "https://example.test/jobs"
parser = "json"
fetcher = "plain"
"#;

    fn dir(name: &str) -> std::path::PathBuf {
        std::env::temp_dir().join(format!("scrappy-registry-{name}-{}", std::process::id()))
    }

    #[test]
    fn loads_manifest_files() {
        let root = dir("loads");
        std::fs::create_dir_all(&root).expect("dir created");
        std::fs::write(root.join("sample.toml"), VALID).expect("manifest written");
        let registry = load(&root).expect("registry loaded");
        assert_eq!(registry.len(), 1);
        assert!(registry.get("sample").is_some());
        std::fs::remove_dir_all(root).expect("dir removed");
    }

    #[test]
    fn rejects_duplicate_ids() {
        let root = dir("duplicates");
        std::fs::create_dir_all(&root).expect("dir created");
        std::fs::write(root.join("a.toml"), VALID).expect("manifest written");
        std::fs::write(root.join("b.toml"), VALID).expect("manifest written");
        let error = load(&root).unwrap_err();
        assert_ne!(error.message, "");
        std::fs::remove_dir_all(root).expect("dir removed");
    }
}
