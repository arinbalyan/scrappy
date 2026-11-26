//! Checkpoint and resume state.
//!
//! See documentation/014-testing.md.

// Rust guideline compliant 2026-10-09
//! Versioned checkpoint and resume state.
//!
//! See documentation/014-testing.md.

// Rust guideline compliant 2026-10-09

use std::collections::BTreeSet;
use std::fs;
use std::path::{Path, PathBuf};

use serde::{Deserialize, Serialize};

use crate::config::Config;
use crate::engine::dedup::content_hash;

/// Current checkpoint schema version.
pub const CHECKPOINT_VERSION: u32 = 1;

/// Error produced when checkpoint operations fail.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct CheckpointError {
    /// Human-readable diagnostic.
    pub message: String,
}

impl std::fmt::Display for CheckpointError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(&self.message)
    }
}

impl std::error::Error for CheckpointError {}

/// Stable summary of work completed in one run.
#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct Checkpoint {
    /// Checkpoint schema version.
    pub version: u32,
    /// Fingerprint of the configuration used for this run.
    pub fingerprint: String,
    /// Sources that completed successfully.
    pub completed_sources: BTreeSet<String>,
    /// Content hashes already emitted.
    pub emitted_hashes: BTreeSet<u64>,
}

impl Checkpoint {
    /// Create a checkpoint for one configuration fingerprint.
    #[must_use]
    pub fn new(fingerprint: impl Into<String>) -> Self {
        Self {
            version: CHECKPOINT_VERSION,
            fingerprint: fingerprint.into(),
            completed_sources: BTreeSet::new(),
            emitted_hashes: BTreeSet::new(),
        }
    }

    /// Return true when this source has already completed.
    #[must_use]
    pub fn is_source_complete(&self, source_id: &str) -> bool {
        self.completed_sources.contains(source_id)
    }

    /// Mark a source complete and record one emitted record.
    pub fn complete(&mut self, source_id: &str, hash: u64) {
        self.completed_sources.insert(source_id.to_owned());
        self.emitted_hashes.insert(hash);
    }

    /// Return true when a content hash was already emitted.
    #[must_use]
    pub fn was_emitted(&self, hash: u64) -> bool {
        self.emitted_hashes.contains(&hash)
    }

    /// Validate this checkpoint against the current version and config.
    ///
    /// # Errors
    /// Returns a diagnostic for an unknown version or mismatched fingerprint.
    pub fn validate(&self, fingerprint: &str) -> Result<(), CheckpointError> {
        if self.version != CHECKPOINT_VERSION {
            return Err(CheckpointError {
                message: format!("checkpoint version {} is not supported", self.version),
            });
        }
        if self.fingerprint != fingerprint {
            return Err(CheckpointError {
                message: "checkpoint does not match this configuration".to_owned(),
            });
        }
        Ok(())
    }
}

/// Build a stable fingerprint from resolved config values.
#[must_use]
pub fn fingerprint(config: &Config) -> String {
    let encoded = serde_json::to_string(config).unwrap_or_default();
    let hash = content_hash(&crate::engine::dedup::ContentFields {
        url: "checkpoint",
        title: &encoded,
        company: "",
        description: "",
    });
    format!("{CHECKPOINT_VERSION}-{hash:016x}")
}

/// Read and validate a checkpoint from a file.
///
/// # Errors
/// Returns a diagnostic for unreadable, malformed, or mismatched state.
pub fn load(path: &Path, config: &Config) -> Result<Checkpoint, CheckpointError> {
    let text = fs::read_to_string(path).map_err(|error| CheckpointError {
        message: format!("cannot read checkpoint {}: {error}", path.display()),
    })?;
    let checkpoint: Checkpoint = serde_json::from_str(&text).map_err(|error| CheckpointError {
        message: format!("cannot parse checkpoint: {error}"),
    })?;
    checkpoint.validate(&fingerprint(config))?;
    Ok(checkpoint)
}

/// Write a checkpoint atomically so interrupted runs leave no partial file.
///
/// # Errors
/// Returns a diagnostic for an encoding or write failure.
pub fn save(path: &Path, checkpoint: &Checkpoint) -> Result<(), CheckpointError> {
    let text = serde_json::to_string_pretty(checkpoint).map_err(|error| CheckpointError {
        message: format!("cannot encode checkpoint: {error}"),
    })?;
    let temporary = temporary_path(path);
    fs::write(&temporary, text).map_err(|error| CheckpointError {
        message: format!("cannot write checkpoint: {error}"),
    })?;
    fs::rename(&temporary, path).map_err(|error| CheckpointError {
        message: format!("cannot install checkpoint: {error}"),
    })?;
    Ok(())
}

fn temporary_path(path: &Path) -> PathBuf {
    let mut name = path.file_name().unwrap_or_default().to_os_string();
    name.push(".tmp");
    path.with_file_name(name)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn config() -> Config {
        serde_json::from_str(r#"{"format":"ndjson","sources":["one"]}"#).expect("valid config")
    }

    #[test]
    fn matching_checkpoint_skips_completed_work() {
        let config = config();
        let checkpoint = Checkpoint::new(fingerprint(&config));
        checkpoint
            .validate(&fingerprint(&config))
            .expect("matching config");
        assert!(!checkpoint.is_source_complete("one"));
    }

    #[test]
    fn mismatched_checkpoint_is_rejected() {
        let mut checkpoint = Checkpoint::new(fingerprint(&config()));
        let changed: Config = serde_json::from_str(r#"{"format":"csv"}"#).expect("valid config");
        assert!(
            checkpoint
                .validate(&fingerprint(&changed))
                .expect_err("fingerprint changed")
                .message
                .contains("configuration")
        );
        checkpoint.version = CHECKPOINT_VERSION + 1;
        assert!(
            checkpoint
                .validate(&fingerprint(&config()))
                .expect_err("version changed")
                .message
                .contains("version")
        );
    }

    #[test]
    fn save_and_load_preserve_progress() {
        let dir = std::env::temp_dir().join(format!("scrappy-checkpoint-{}", std::process::id()));
        fs::create_dir_all(&dir).expect("temp directory");
        let path = dir.join("checkpoint.json");
        let config = config();
        let mut checkpoint = Checkpoint::new(fingerprint(&config));
        checkpoint.complete("one", 42);
        save(&path, &checkpoint).expect("checkpoint saved");
        let loaded = load(&path, &config).expect("checkpoint loaded");
        assert!(loaded.is_source_complete("one"));
        assert!(loaded.was_emitted(42));
        assert!(!path.with_extension("json.tmp").exists());
        fs::remove_dir_all(&dir).expect("temp directory removed");
    }
}
