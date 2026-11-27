// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Arin Balyan
// Last modified: 2026-11-27

//! Config file actions and private atomic writes.
//!
//! The editing workflow is specified in
//! openspec/specs/cli-config/spec.md.

// Rust guideline compliant 2026-10-09

use std::collections::BTreeMap;
use std::fmt::Write as FmtWrite;
use std::fs;
use std::io::{self, Write};
use std::path::{Path, PathBuf};

use serde::Serialize;
use serde_json::Value;

use crate::config::{CONFIG_SCHEMA, ConfigError, FieldInfo};

/// Error produced while editing a config file.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct FileError {
    /// Human-readable diagnostic.
    pub message: String,
}

impl std::fmt::Display for FileError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(&self.message)
    }
}

impl std::error::Error for FileError {}

/// Build the commented config template used by `config init`.
#[must_use]
pub fn template() -> String {
    let mut out = String::from(
        "# scrappy configuration\n# Types, defaults, and env vars are documented below.\n",
    );
    for field in CONFIG_SCHEMA {
        let _ = writeln!(
            out,
            "# {} ({}; default: {}; env: SCRAPPY_{})\n",
            field.name,
            field.kind,
            field.default,
            field.name.to_uppercase()
        );
    }
    out.push_str("\n# No credentials belong in this file. Use environment variables.\n");
    out
}

/// Write a config template to `path`, refusing to overwrite unless forced.
///
/// # Errors
/// Returns `FileError` when the path exists and `force` is false, or on I/O failure.
pub fn init(path: &Path, force: bool) -> Result<(), FileError> {
    if path.exists() && !force {
        return Err(FileError {
            message: format!("refusing to overwrite existing file: {}", path.display()),
        });
    }
    write_private(path, template().as_bytes())
}

/// Read a config document as a generic JSON value.
///
/// # Errors
/// Returns `FileError` when the file is unreadable or invalid TOML.
pub fn read(path: &Path) -> Result<BTreeMap<String, Value>, FileError> {
    let text = fs::read_to_string(path).map_err(|error| FileError {
        message: format!("cannot read {}: {error}", path.display()),
    })?;
    toml::from_str(&text).map_err(|error| FileError {
        message: format!("cannot parse {}: {error}", path.display()),
    })
}

/// Set one key while preserving all other sections.
///
/// # Errors
/// Returns `FileError` when the file cannot be read, parsed, or written.
pub fn set(path: &Path, key: &str, value: &str) -> Result<(), FileError> {
    let mut document = read(path)?;
    let parsed = parse_value(value).map_err(|error| FileError {
        message: format!("invalid value for {key}: {error}"),
    })?;
    document.insert(key.to_owned(), parsed);
    write_document(path, &document)
}

/// Remove one key while preserving all other sections.
///
/// # Errors
/// Returns `FileError` when the file cannot be read, parsed, or written.
pub fn unset(path: &Path, key: &str) -> Result<(), FileError> {
    let mut document = read(path)?;
    document.remove(key);
    write_document(path, &document)
}

/// Open `path` in the editor named by the `EDITOR` environment variable.
///
/// # Errors
/// Returns `FileError` when the editor fails.
pub fn edit(path: &Path) -> Result<(), FileError> {
    let editor = std::env::var("EDITOR").unwrap_or_else(|_| "vi".to_owned());
    let status = std::process::Command::new(editor)
        .arg(path)
        .status()
        .map_err(|error| FileError {
            message: format!("cannot start editor: {error}"),
        })?;
    if status.success() {
        Ok(())
    } else {
        Err(FileError {
            message: format!("editor exited with {status}"),
        })
    }
}

/// Explain the winning layer for a field.
#[must_use]
pub fn explain(key: &str) -> String {
    let field = CONFIG_SCHEMA
        .iter()
        .find(|field| field.name == key)
        .map_or("unknown", |field| field.name);
    format!("field={field} layer=default value=<redacted>")
}

/// Return the schema entries used by config actions.
#[must_use]
pub const fn schema() -> &'static [FieldInfo] {
    CONFIG_SCHEMA
}

fn write_document(path: &Path, document: &BTreeMap<String, Value>) -> Result<(), FileError> {
    let text = toml::to_string_pretty(document).map_err(|error| FileError {
        message: format!("cannot encode config: {error}"),
    })?;
    write_private(path, text.as_bytes())
}

fn write_private(path: &Path, bytes: &[u8]) -> Result<(), FileError> {
    if let Some(parent) = path.parent()
        && !parent.as_os_str().is_empty()
        && !parent.exists()
    {
        fs::create_dir_all(parent).map_err(|error| FileError {
            message: format!("cannot create {}: {error}", parent.display()),
        })?;
        restrict_dir(parent).map_err(|error| FileError {
            message: format!("cannot secure {}: {error}", parent.display()),
        })?;
    }
    let temp = temp_path(path);
    let mut file = fs::File::create(&temp).map_err(|error| FileError {
        message: format!("cannot create {}: {error}", temp.display()),
    })?;
    file.write_all(bytes).map_err(|error| FileError {
        message: format!("cannot write {}: {error}", temp.display()),
    })?;
    file.sync_all().map_err(|error| FileError {
        message: format!("cannot sync {}: {error}", temp.display()),
    })?;
    restrict_file(&temp).map_err(|error| FileError {
        message: format!("cannot secure {}: {error}", temp.display()),
    })?;
    fs::rename(&temp, path).map_err(|error| FileError {
        message: format!("cannot replace {}: {error}", path.display()),
    })
}

fn temp_path(path: &Path) -> PathBuf {
    let name = path
        .file_name()
        .and_then(|name| name.to_str())
        .unwrap_or("config");
    path.with_file_name(format!(".{name}.tmp-{}", std::process::id()))
}

#[cfg(unix)]
fn restrict_file(path: &Path) -> io::Result<()> {
    use std::os::unix::fs::PermissionsExt;
    fs::set_permissions(path, fs::Permissions::from_mode(0o600))
}

#[cfg(not(unix))]
fn restrict_file(_path: &Path) -> io::Result<()> {
    Ok(())
}

#[cfg(unix)]
fn restrict_dir(path: &Path) -> io::Result<()> {
    use std::os::unix::fs::PermissionsExt;
    fs::set_permissions(path, fs::Permissions::from_mode(0o700))
}

#[cfg(not(unix))]
fn restrict_dir(_path: &Path) -> io::Result<()> {
    Ok(())
}

fn parse_value(raw: &str) -> Result<Value, ConfigError> {
    serde_json::from_str(raw).map_err(|error| ConfigError {
        message: error.to_string(),
    })
}

/// Serialize a schema entry for machine-readable output.
#[derive(Debug, Serialize)]
struct SchemaView<'a> {
    name: &'a str,
    kind: &'a str,
    default: &'a str,
}

/// Print the runtime config schema as JSON.
///
/// # Errors
/// Returns `FileError` when serialization fails.
pub fn print_schema() -> Result<String, FileError> {
    let views: Vec<SchemaView<'_>> = CONFIG_SCHEMA
        .iter()
        .map(|field| SchemaView {
            name: field.name,
            kind: field.kind,
            default: field.default,
        })
        .collect();
    serde_json::to_string_pretty(&views).map_err(|error| FileError {
        message: format!("cannot encode schema: {error}"),
    })
}
