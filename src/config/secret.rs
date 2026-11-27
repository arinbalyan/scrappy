//! Redacting credential values and their environment boundary.
//!
//! Credentials stay out of committed config files and never appear in
//! formatted diagnostics. See documentation/015-security.md.

// Rust guideline compliant 2026-10-09

use std::fmt;

use crate::config::ENV_PREFIX;
use crate::security::Secret;

/// Suffix identifying an environment variable as a credential.
const CREDENTIAL_SUFFIX: &str = "_KEY";
/// Suffix for token-shaped credentials.
const TOKEN_SUFFIX: &str = "_TOKEN";

/// A credential loaded from the environment for request authentication.
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

/// Load credentials named in source manifests from the environment.
///
/// Credential names form `SCRAPPY_<NAME>` variables; values are read only
/// from the environment and never from a config file.
#[must_use]
pub fn from_environment(names: &[String]) -> Vec<(String, Credential)> {
    names
        .iter()
        .filter_map(|name| {
            std::env::var(format!("{ENV_PREFIX}{}", name.to_ascii_uppercase()))
                .ok()
                .map(|value| (name.clone(), credential(value)))
        })
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
    std::env::vars()
        .filter(|(name, _)| is_credential_name(name))
        .map(|(name, value)| (name, credential(value)))
        .collect()
}
