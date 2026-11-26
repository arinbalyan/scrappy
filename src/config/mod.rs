//! Typed configuration and layered resolution.
//!
//! Field behavior is specified in
//! openspec/changes/oculus-foundation/specs/cli-config/spec.md.
//! Operational notes are in documentation/004-configuration.md.

// Rust guideline compliant 2026-10-09

use std::collections::BTreeMap;
use std::fmt;
use std::path::{Path, PathBuf};

use serde::de::DeserializeOwned;
use serde::{Deserialize, Deserializer, Serialize};

pub mod file;

/// Path for the user-level config file below the home directory.
pub const USER_CONFIG_PATH: &str = "scrappy/config.toml";
/// Path for the project-level config file.
pub const PROJECT_CONFIG_PATH: &str = "config.toml";
/// Prefix for environment variables that overlay config values.
pub const ENV_PREFIX: &str = "SCRAPPY_";

/// Accepted values for the `log_level` field.
pub const LOG_LEVELS: &[&str] = &["error", "warn", "info", "debug", "trace"];

/// Accepted values for the `format` field.
pub const OUTPUT_FORMATS: &[&str] = &["ndjson", "json", "csv"];

/// One documented config field.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct FieldInfo {
    /// Config key.
    pub name: &'static str,
    /// Value type.
    pub kind: &'static str,
    /// Default value.
    pub default: &'static str,
}

/// Metadata for every config field accepted by the runtime.
pub const CONFIG_SCHEMA: &[FieldInfo] = &[
    FieldInfo {
        name: "search",
        kind: "string or array",
        default: "none",
    },
    FieldInfo {
        name: "sources",
        kind: "string or array",
        default: "none",
    },
    FieldInfo {
        name: "format",
        kind: "enum",
        default: "ndjson",
    },
    FieldInfo {
        name: "rates",
        kind: "map of string to integer",
        default: "none",
    },
    FieldInfo {
        name: "concurrency",
        kind: "integer",
        default: "4",
    },
    FieldInfo {
        name: "timeout",
        kind: "integer seconds",
        default: "30",
    },
    FieldInfo {
        name: "memory_mb",
        kind: "integer megabytes",
        default: "512",
    },
    FieldInfo {
        name: "log_level",
        kind: "enum",
        default: "info",
    },
    FieldInfo {
        name: "is_remote",
        kind: "boolean",
        default: "unset",
    },
];

/// Error produced when config discovery or validation fails.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ConfigError {
    /// Human-readable diagnostic.
    pub message: String,
}

impl fmt::Display for ConfigError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(&self.message)
    }
}

impl std::error::Error for ConfigError {}

/// A scalar or array accepted for the same config field.
#[derive(Debug, Clone, PartialEq, Eq, Serialize)]
#[serde(untagged)]
pub enum OneOrMany<T> {
    /// A single value.
    One(T),
    /// Multiple values.
    Many(Vec<T>),
}

impl<T> OneOrMany<T> {
    /// Return the configured values as a vector.
    pub fn into_vec(self) -> Vec<T> {
        match self {
            Self::One(value) => vec![value],
            Self::Many(values) => values,
        }
    }
}

impl<'de, T> Deserialize<'de> for OneOrMany<T>
where
    T: Deserialize<'de>,
{
    fn deserialize<D>(deserializer: D) -> Result<Self, D::Error>
    where
        D: Deserializer<'de>,
    {
        #[derive(Deserialize)]
        #[serde(untagged)]
        enum Wire<U> {
            One(U),
            Many(Vec<U>),
        }

        match Wire::deserialize(deserializer)? {
            Wire::One(value) => Ok(Self::One(value)),
            Wire::Many(values) => Ok(Self::Many(values)),
        }
    }
}

/// Config document before defaults are applied.
#[derive(Debug, Clone, Default, Deserialize, Serialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct Config {
    /// Search terms for job filtering.
    pub search: Option<OneOrMany<String>>,
    /// Source identifiers selected for a run.
    pub sources: Option<OneOrMany<String>>,
    /// Output format name.
    pub format: Option<String>,
    /// Per-source rate map in requests per minute.
    pub rates: Option<BTreeMap<String, usize>>,
    /// Maximum concurrent requests.
    pub concurrency: Option<usize>,
    /// Request timeout in seconds.
    pub timeout: Option<u64>,
    /// Memory ceiling in megabytes.
    pub memory_mb: Option<u64>,
    /// Human-readable log level.
    pub log_level: Option<String>,
    /// Tri-state remote filter.
    pub is_remote: Option<bool>,
}

/// Inputs used to build a layered configuration.
#[derive(Debug, Clone, Default)]
pub struct Sources {
    /// Explicit config paths, in increasing precedence.
    pub files: Vec<PathBuf>,
    /// Whether discovered files and environment values are disabled.
    pub hermetic: bool,
    /// Pre-parsed CLI overrides.
    pub cli: Config,
}

/// Load and resolve configuration from files, environment, and CLI values.
///
/// # Errors
/// Returns `ConfigError` for unreadable files, invalid values, or unknown keys.
pub fn resolve(sources: &Sources) -> Result<Config, ConfigError> {
    let mut merged: BTreeMap<String, serde_json::Value> = BTreeMap::new();
    if !sources.hermetic {
        let user = user_config_path();
        if user.is_file() {
            merge_document(&mut merged, &user)?;
        }
        let project = Path::new(PROJECT_CONFIG_PATH);
        if project.is_file() {
            merge_document(&mut merged, project)?;
        }
        merge_environment(&mut merged)?;
    }
    for file in &sources.files {
        merge_document(&mut merged, file)?;
    }
    let cli = serde_json::to_value(&sources.cli).map_err(|error| ConfigError {
        message: format!("cannot encode cli config: {error}"),
    })?;
    merge_value(&mut merged, cli);
    serde_json::from_value(serde_json::Value::Object(merged.into_iter().collect())).map_err(
        |error| ConfigError {
            message: format!("invalid config: {error}"),
        },
    )
}

fn merge_document(
    merged: &mut BTreeMap<String, serde_json::Value>,
    path: &Path,
) -> Result<(), ConfigError> {
    let text = std::fs::read_to_string(path).map_err(|error| ConfigError {
        message: format!("cannot read {}: {error}", path.display()),
    })?;
    let value: BTreeMap<String, serde_json::Value> =
        toml::from_str(&text).map_err(|error| ConfigError {
            message: format!("cannot parse {}: {error}", path.display()),
        })?;
    for (key, value) in value {
        merged.insert(key, value);
    }
    Ok(())
}

fn merge_environment(merged: &mut BTreeMap<String, serde_json::Value>) -> Result<(), ConfigError> {
    for field in [
        "search",
        "sources",
        "format",
        "rates",
        "concurrency",
        "timeout",
        "memory_mb",
        "log_level",
        "is_remote",
    ] {
        let name = format!("{ENV_PREFIX}{}", field.to_uppercase());
        let Ok(raw) = std::env::var(&name) else {
            continue;
        };
        let value = if field == "search" || field == "sources" {
            parse_list_or_scalar(&raw)
        } else if field == "concurrency" || field == "timeout" || field == "memory_mb" {
            raw.parse::<u64>()
                .map(serde_json::Value::from)
                .map_err(|error| ConfigError {
                    message: format!("invalid {name}: {error}"),
                })?
        } else if field == "is_remote" {
            raw.parse::<bool>()
                .map(serde_json::Value::from)
                .map_err(|error| ConfigError {
                    message: format!("invalid {name}: {error}"),
                })?
        } else if field == "rates" {
            serde_json::from_str(&raw).map_err(|error| ConfigError {
                message: format!("invalid {name}: {error}"),
            })?
        } else {
            serde_json::Value::String(raw)
        };
        merged.insert(field.to_owned(), value);
    }
    Ok(())
}

fn parse_list_or_scalar(raw: &str) -> serde_json::Value {
    if let Ok(values) = serde_json::from_str::<Vec<String>>(raw) {
        serde_json::Value::Array(values.into_iter().map(serde_json::Value::String).collect())
    } else {
        serde_json::Value::String(raw.to_owned())
    }
}

fn merge_value(merged: &mut BTreeMap<String, serde_json::Value>, value: serde_json::Value) {
    let serde_json::Value::Object(object) = value else {
        return;
    };
    for (key, value) in object {
        if !value.is_null() {
            merged.insert(key, value);
        }
    }
}

/// Return the user config path with home directory expansion.
#[must_use]
pub fn user_config_path() -> PathBuf {
    if let Some(home) = std::env::var_os("HOME") {
        PathBuf::from(home).join(".config").join(USER_CONFIG_PATH)
    } else {
        PathBuf::from(USER_CONFIG_PATH)
    }
}

/// Validate resolved values that need cross-field checks.
///
/// # Errors
/// Returns `ConfigError` for out-of-range values or unsupported names.
pub fn validate(config: &Config) -> Result<(), ConfigError> {
    if let Some(level) = &config.log_level
        && !LOG_LEVELS.contains(&level.as_str())
    {
        return Err(ConfigError {
            message: format!(
                "invalid log_level '{level}'; accepted values: {}",
                LOG_LEVELS.join(", ")
            ),
        });
    }
    if let Some(format) = &config.format
        && !OUTPUT_FORMATS.contains(&format.as_str())
    {
        return Err(ConfigError {
            message: format!(
                "invalid format '{format}'; accepted values: {}",
                OUTPUT_FORMATS.join(", ")
            ),
        });
    }
    if config.memory_mb == Some(0) {
        return Err(ConfigError {
            message: "memory_mb must be greater than zero".to_owned(),
        });
    }
    if config.timeout == Some(0) {
        return Err(ConfigError {
            message: "timeout must be greater than zero".to_owned(),
        });
    }
    if let Some(rates) = &config.rates {
        for (source, rate) in rates {
            if source.is_empty() || *rate == 0 {
                return Err(ConfigError {
                    message: format!("invalid rate for source '{source}': {rate}"),
                });
            }
        }
    }
    Ok(())
}

/// Return accepted values for a named enum field.
#[must_use]
pub fn enum_values(name: &str) -> Option<&'static [&'static str]> {
    match name {
        "format" => Some(OUTPUT_FORMATS),
        "log_level" => Some(LOG_LEVELS),
        _ => None,
    }
}

/// Parse a config document from a string.
///
/// # Errors
/// Returns `ConfigError` when the document is malformed.
pub fn parse<T: DeserializeOwned>(text: &str) -> Result<T, ConfigError> {
    toml::from_str(text).map_err(|error| ConfigError {
        message: format!("cannot parse config: {error}"),
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn scalar_and_array_forms_load() {
        let scalar: Config = parse("search = 'rust'").expect("scalar loads");
        let array: Config = parse("search = ['rust', 'go']").expect("array loads");
        assert_eq!(
            scalar.search.expect("scalar present").into_vec(),
            vec!["rust".to_owned()]
        );
        assert_eq!(
            array.search.expect("array present").into_vec(),
            vec!["rust".to_owned(), "go".to_owned()]
        );
    }

    #[test]
    fn empty_list_is_preserved() {
        let config: Config = parse("search = []").expect("empty array loads");
        assert_eq!(
            config.search.expect("empty present").into_vec(),
            Vec::<String>::new()
        );
    }

    #[test]
    fn explicit_false_is_preserved() {
        let config: Config = parse("is_remote = false").expect("false loads");
        assert_eq!(config.is_remote, Some(false));
    }

    #[test]
    fn unknown_key_fails() {
        let error = parse::<Config>("unknown_key = 1").expect_err("unknown fails");
        assert!(error.message.contains("unknown field"));
    }

    #[test]
    fn validation_rejects_bad_values() {
        let config: Config = parse("log_level = 'loud'").expect("loads");
        assert!(validate(&config).is_err());
        let config: Config = parse("memory_mb = 0").expect("loads");
        assert!(validate(&config).is_err());
    }
}

#[cfg(test)]
mod precedence_tests {
    use super::*;

    #[test]
    fn cli_overrides_lower_layers() {
        let cli = Config {
            format: Some("csv".to_owned()),
            ..Config::default()
        };
        let config = resolve(&Sources {
            hermetic: true,
            cli,
            ..Sources::default()
        })
        .expect("resolution succeeds");
        assert_eq!(config.format.as_deref(), Some("csv"));
    }

    #[test]
    fn hermetic_ignores_files() {
        let dir = std::env::temp_dir().join(format!("scrappy-test-{}", std::process::id()));
        std::fs::create_dir_all(&dir).expect("temp dir created");
        let path = dir.join("config.toml");
        std::fs::write(
            &path,
            "format = 'json'
",
        )
        .expect("config written");
        let config = resolve(&Sources {
            hermetic: true,
            ..Sources::default()
        })
        .expect("resolution succeeds");
        assert_eq!(config.format, None);
        std::fs::remove_file(path).expect("config removed");
    }
}
