// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Arin Balyan
// Last modified: 2026-11-27

//! Redacting credential values, the environment boundary, and the secret store.
//!
//! Credentials stay out of committed config files and never appear in
//! formatted diagnostics. See documentation/015-security.md.

// Rust guideline compliant 2026-10-09

use std::collections::BTreeMap;
use std::fmt;
use std::path::{Path, PathBuf};

use crate::config::{Config, ENV_PREFIX, Sources};
use crate::security::Secret;

/// Suffix identifying an environment variable as a credential.
const CREDENTIAL_SUFFIX: &str = "_KEY";
/// Suffix for token-shaped credentials.
const TOKEN_SUFFIX: &str = "_TOKEN";

/// A credential loaded from the environment or a secrets file.
///
/// Both formatted forms render only a redaction marker. Use `as_str` at the
/// authentication boundary only.
#[derive(Clone, PartialEq, Eq, Hash)]
pub struct Credential(Secret);

impl fmt::Debug for Credential {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        fmt::Debug::fmt(&self.0, f)
    }
}

impl fmt::Display for Credential {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        fmt::Display::fmt(&self.0, f)
    }
}

impl Credential {
    /// Return the credential for authenticated use.
    #[must_use]
    pub fn as_str(&self) -> &str {
        self.0.as_str()
    }

    /// Wrap a credential value in a redacting type.
    #[must_use]
    pub fn new(value: impl Into<String>) -> Self {
        credential(value)
    }
}

/// Wrap a credential value in a redacting type.
#[must_use]
pub fn credential(value: impl Into<String>) -> Credential {
    Credential(Secret::from(value.into()))
}

/// Error produced while loading a secrets file.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SecretError {
    /// Human-readable diagnostic with no credential values.
    pub message: String,
}

impl fmt::Display for SecretError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(&self.message)
    }
}

impl std::error::Error for SecretError {}

/// Credential presence for one normalized name.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct CredentialPresence {
    /// Normalized name without the SCRAPPY_ prefix.
    pub name: String,
    /// Whether a non-empty value is available.
    pub present: bool,
}

/// Redacted presence report for declared credential names.
///
/// The report carries names and presence only. Values never enter it.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct SecretReport {
    /// One entry per distinct normalized name, in request order.
    pub entries: Vec<CredentialPresence>,
}

impl SecretReport {
    /// Return true when no name is missing.
    #[must_use]
    pub fn is_complete(&self) -> bool {
        self.entries.iter().all(|entry| entry.present)
    }
}

impl fmt::Display for SecretReport {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        for (index, entry) in self.entries.iter().enumerate() {
            if index > 0 {
                f.write_str(", ")?;
            }
            f.write_str(&entry.name)?;
            f.write_str(if entry.present {
                "=present"
            } else {
                "=missing"
            })?;
        }
        Ok(())
    }
}

/// Credentials read from SCRAPPY_<NAME> and an optional secrets file.
///
/// Names are normalized to bare uppercase form, with the SCRAPPY_ prefix
/// removed. Environment values take precedence over file values. Empty values
/// count as missing.
#[derive(Debug, Clone, Default)]
pub struct SecretStore {
    values: BTreeMap<String, Credential>,
}

impl SecretStore {
    /// Load the store from the process environment and an optional file.
    ///
    /// The file accepts NAME=value, `SCRAPPY_NAME=value`, and
    /// export NAME=value entries with comments and quoted values.
    ///
    /// # Errors
    /// Returns `SecretError` when the file cannot be read or parsed.
    pub fn load(secrets_file: Option<&Path>) -> Result<Self, SecretError> {
        Self::load_with(secrets_file, std::env::vars())
    }

    /// Load a store from a resolved config and the environment.
    ///
    /// A file error falls back to environment values so callers without an
    /// error channel still get a usable presence check.
    #[must_use]
    pub fn from_config(config: &Config) -> Self {
        Self::load(config.secrets_file.as_deref()).unwrap_or_else(|_error| Self::from_environment())
    }

    /// Load a store for the given config files and environment.
    ///
    /// The resolved `secrets_file` path is used when configuration is readable.
    /// Otherwise only environment values are loaded.
    #[must_use]
    pub fn discover(files: &[PathBuf]) -> Self {
        let sources = Sources {
            files: files.to_vec(),
            ..Sources::default()
        };
        match crate::config::resolve(&sources) {
            Ok(config) => Self::from_config(&config),
            Err(_error) => Self::from_environment(),
        }
    }

    /// Load the store from the process environment only.
    #[must_use]
    pub fn from_environment() -> Self {
        Self::from_variables(std::env::vars())
    }

    /// Return the value for a bare or exported credential name.
    #[must_use]
    pub fn get(&self, name: &str) -> Option<&Credential> {
        self.values.get(&canonical_name(name))
    }

    /// Return the canonical names that have no value.
    #[must_use]
    pub fn missing(&self, names: &[String]) -> Vec<String> {
        self.report(names)
            .entries
            .into_iter()
            .filter(|entry| !entry.present)
            .map(|entry| entry.name)
            .collect()
    }

    /// Return a redacted report for the named credentials.
    #[must_use]
    pub fn report(&self, names: &[String]) -> SecretReport {
        let mut entries: Vec<CredentialPresence> = Vec::new();
        for name in names {
            let canonical = canonical_name(name);
            if entries.iter().any(|entry| entry.name == canonical) {
                continue;
            }
            entries.push(CredentialPresence {
                present: self.values.contains_key(&canonical),
                name: canonical,
            });
        }
        SecretReport { entries }
    }

    /// Return values keyed by the requested credential names.
    ///
    /// Requested names keep their original spelling so manifest placeholders
    /// such as {`key:findwork_auth`} continue to resolve.
    #[must_use]
    pub fn values(&self, names: &[String]) -> BTreeMap<String, Credential> {
        names
            .iter()
            .filter_map(|name| self.get(name).map(|value| (name.clone(), value.clone())))
            .collect()
    }

    fn from_variables<I>(variables: I) -> Self
    where
        I: IntoIterator<Item = (String, String)>,
    {
        let mut values = BTreeMap::new();
        for (name, value) in variables {
            let Some(canonical) = environment_name(&name) else {
                continue;
            };
            if value.trim().is_empty() {
                continue;
            }
            values.insert(canonical, credential(value));
        }
        Self { values }
    }

    fn load_with<I>(secrets_file: Option<&Path>, variables: I) -> Result<Self, SecretError>
    where
        I: IntoIterator<Item = (String, String)>,
    {
        let mut store = Self::from_variables(variables);
        if let Some(path) = secrets_file {
            for (name, value) in read_secrets(path)? {
                if value.trim().is_empty() {
                    continue;
                }
                store
                    .values
                    .entry(name)
                    .or_insert_with(|| credential(value));
            }
        }
        Ok(store)
    }
}

/// Load credentials named in source manifests from the environment.
///
/// Credential names form SCRAPPY_<NAME> variables; values are read from the
/// store boundary and never from a config file.
#[must_use]
pub fn from_environment(names: &[String]) -> Vec<(String, Credential)> {
    let store = SecretStore::from_environment();
    names
        .iter()
        .filter_map(|name| store.get(name).map(|value| (name.clone(), value.clone())))
        .collect()
}

/// Return true when an environment variable name denotes a credential.
#[must_use]
pub fn is_credential_name(name: &str) -> bool {
    name.starts_with(ENV_PREFIX)
        && (name.ends_with(CREDENTIAL_SUFFIX) || name.ends_with(TOKEN_SUFFIX))
}

/// Collect credential-shaped variables from the process environment.
#[must_use]
pub fn environment_credentials() -> Vec<(String, Credential)> {
    let store = SecretStore::from_environment();
    store
        .values
        .iter()
        .map(|(name, value)| (format!("{ENV_PREFIX}{name}"), value.clone()))
        .filter(|(name, _value)| is_credential_name(name))
        .collect()
}

fn canonical_name(name: &str) -> String {
    let upper = name.trim().to_ascii_uppercase();
    upper.strip_prefix(ENV_PREFIX).unwrap_or(&upper).to_owned()
}

fn environment_name(name: &str) -> Option<String> {
    let upper = name.to_ascii_uppercase();
    upper
        .strip_prefix(ENV_PREFIX)
        .filter(|bare| !bare.is_empty())
        .map(str::to_owned)
}

fn read_secrets(path: &Path) -> Result<Vec<(String, String)>, SecretError> {
    let text = std::fs::read_to_string(path).map_err(|error| SecretError {
        message: format!("cannot read secrets file {}: {error}", path.display()),
    })?;
    parse_entries(&text).map_err(|reason| SecretError {
        message: format!("cannot parse secrets file {}: {reason}", path.display()),
    })
}

fn parse_entries(text: &str) -> Result<Vec<(String, String)>, String> {
    let mut entries = Vec::new();
    for (index, raw) in text.lines().enumerate() {
        let line = raw.trim();
        if line.is_empty() || line.starts_with('#') {
            continue;
        }
        let (key, raw_value) = line
            .split_once('=')
            .ok_or_else(|| format!("invalid entry on line {}", index + 1))?;
        let key = strip_export(key.trim());
        let name = canonical_name(key);
        if name.is_empty() {
            return Err(format!("invalid entry on line {}", index + 1));
        }
        let value =
            parse_value(raw_value).map_err(|reason| format!("line {}: {reason}", index + 1))?;
        entries.push((name, value));
    }
    Ok(entries)
}

fn strip_export(key: &str) -> &str {
    let mut parts = key.splitn(2, char::is_whitespace);
    let first = parts.next().unwrap_or_default();
    if !first.eq_ignore_ascii_case("export") {
        return key;
    }
    parts.next().map_or("", str::trim_start)
}

fn parse_value(raw: &str) -> Result<String, String> {
    let raw = raw.trim_start();
    if let Some(rest) = raw.strip_prefix('\'') {
        let end = rest
            .find('\'')
            .ok_or_else(|| "unterminated single quote".to_owned())?;
        ensure_trailing(&rest[end + 1..])?;
        Ok(rest[..end].to_owned())
    } else if let Some(rest) = raw.strip_prefix('"') {
        let (value, rest) = parse_double_quoted(rest)?;
        ensure_trailing(rest)?;
        Ok(value)
    } else {
        Ok(strip_inline_comment(raw).trim().to_owned())
    }
}

fn parse_double_quoted(input: &str) -> Result<(String, &str), String> {
    let mut value = String::new();
    let mut chars = input.char_indices();
    while let Some((index, ch)) = chars.next() {
        match ch {
            '"' => return Ok((value, &input[index + ch.len_utf8()..])),
            '\\' => {
                let Some((_escape_index, escaped)) = chars.next() else {
                    return Err("unterminated escape".to_owned());
                };
                match escaped {
                    'n' => value.push('\n'),
                    'r' => value.push('\r'),
                    't' => value.push('\t'),
                    other => value.push(other),
                }
            }
            other => value.push(other),
        }
    }
    Err("unterminated double quote".to_owned())
}

fn ensure_trailing(rest: &str) -> Result<(), String> {
    let rest = rest.trim_start();
    if rest.is_empty() || rest.starts_with('#') {
        Ok(())
    } else {
        Err("unexpected text after value".to_owned())
    }
}

fn strip_inline_comment(value: &str) -> &str {
    let bytes = value.as_bytes();
    for (index, byte) in bytes.iter().enumerate() {
        if *byte == b'#' && (index == 0 || bytes[index - 1].is_ascii_whitespace()) {
            return &value[..index];
        }
    }
    value
}

#[cfg(test)]
mod tests {
    use super::*;

    fn write_temp(name: &str, text: &str) -> PathBuf {
        let path =
            std::env::temp_dir().join(format!("scrappy-secret-{}-{name}", std::process::id()));
        std::fs::write(&path, text).expect("secrets file written");
        path
    }

    #[test]
    fn environment_values_take_precedence_over_the_secrets_file() {
        let path = write_temp("precedence", "SCRAPPY_EXA_API_KEY=file-value");
        let store = SecretStore::load_with(
            Some(&path),
            [("SCRAPPY_EXA_API_KEY".to_owned(), "env-value".to_owned())],
        )
        .expect("store loads");

        assert_eq!(
            store.get("EXA_API_KEY").map(Credential::as_str),
            Some("env-value")
        );
        assert_eq!(
            store.get("SCRAPPY_EXA_API_KEY").map(Credential::as_str),
            Some("env-value")
        );
        let values = store.values(&["exa_api_key".to_owned()]);
        assert_eq!(
            values.get("exa_api_key").map(Credential::as_str),
            Some("env-value")
        );
        std::fs::remove_file(&path).expect("secrets file removed");
    }

    #[test]
    fn secrets_file_supports_comments_quotes_and_export() {
        let path = write_temp(
            "parse",
            r#"# comment line
BARE=bare-value
QUOTED="quoted # keeps hash"
SINGLE='single value'
export EXPORTED=exported-value # trailing comment
export SCRAPPY_PREFIXED=prefixed-value
"#,
        );
        let store = SecretStore::load_with(Some(&path), std::iter::empty()).expect("store loads");

        assert_eq!(
            store.get("BARE").map(Credential::as_str),
            Some("bare-value")
        );
        assert_eq!(
            store.get("QUOTED").map(Credential::as_str),
            Some("quoted # keeps hash")
        );
        assert_eq!(
            store.get("SINGLE").map(Credential::as_str),
            Some("single value")
        );
        assert_eq!(
            store.get("EXPORTED").map(Credential::as_str),
            Some("exported-value")
        );
        assert_eq!(
            store.get("PREFIXED").map(Credential::as_str),
            Some("prefixed-value")
        );
        std::fs::remove_file(&path).expect("secrets file removed");
    }

    #[test]
    fn missing_names_are_normalized_and_deduplicated() {
        let store = SecretStore::from_variables([
            ("SCRAPPY_PRESENT_KEY".to_owned(), "present-value".to_owned()),
            ("SCRAPPY_EMPTY_KEY".to_owned(), "  ".to_owned()),
        ]);

        assert_eq!(
            store.missing(&[
                "PRESENT_KEY".to_owned(),
                "SCRAPPY_MISSING_KEY".to_owned(),
                "missing_key".to_owned(),
                "EMPTY_KEY".to_owned(),
            ]),
            vec!["MISSING_KEY".to_owned(), "EMPTY_KEY".to_owned()]
        );
        assert_eq!(
            store.get("SCRAPPY_PRESENT_KEY").map(Credential::as_str),
            Some("present-value")
        );
    }

    #[test]
    fn reports_list_names_and_presence_without_values() {
        let secret = "SyntheticSecretAbcd1234Efgh5678";
        let store =
            SecretStore::from_variables([("SCRAPPY_PRESENT_KEY".to_owned(), secret.to_owned())]);
        let report = store.report(&["PRESENT_KEY".to_owned(), "ABSENT_KEY".to_owned()]);
        let rendered = report.to_string();

        assert!(rendered.contains("PRESENT_KEY=present"));
        assert!(rendered.contains("ABSENT_KEY=missing"));
        assert!(!rendered.contains(secret));
        assert!(!format!("{store:?}").contains(secret));
        assert!(
            format!("{:?}", store.get("PRESENT_KEY").expect("value present"))
                .contains("[redacted]")
        );
    }
}
