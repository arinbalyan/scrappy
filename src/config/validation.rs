//! Actionable validation diagnostics for resolved configuration.
//!
//! The diagnostic contract is specified in
//! openspec/changes/oculus-foundation/specs/cli-config/spec.md.

// Rust guideline compliant 2026-10-09

use crate::config::{Config, ConfigError, LOG_LEVELS, OUTPUT_FORMATS};

/// Accepted values for the `job_type` field.
pub const JOB_TYPES: &[&str] = &["fulltime", "parttime", "contract", "internship"];

/// Validate a resolved config with field-specific remedies.
///
/// # Errors
/// Returns `ConfigError` naming the field, accepted values or range, and a
/// remedy for each unsupported value.
pub fn validate(config: &Config) -> Result<(), ConfigError> {
    enum_field("format", config.format.as_deref(), OUTPUT_FORMATS)?;
    enum_field("log_level", config.log_level.as_deref(), LOG_LEVELS)?;
    enum_field("job_type", config.job_type.as_deref(), JOB_TYPES)?;

    if config.min_score.is_some_and(|score| score > 100) {
        return Err(error(
            "min_score",
            "must be between 0 and 100",
            "use a value in the documented range",
        ));
    }
    if config.concurrency.is_some_and(|value| value == 0) {
        return Err(error(
            "concurrency",
            "must be greater than zero",
            "use a positive integer",
        ));
    }
    if config.timeout.is_some_and(|value| value == 0) {
        return Err(error(
            "timeout",
            "must be greater than zero",
            "use a positive number of seconds",
        ));
    }
    if config.memory_mb.is_some_and(|value| value == 0) {
        return Err(error(
            "memory_mb",
            "must be greater than zero",
            "use a positive number of megabytes",
        ));
    }
    if config.hours_old.is_some_and(|value| value == 0) {
        return Err(error(
            "hours_old",
            "must be greater than zero",
            "use a positive number of hours",
        ));
    }
    if let Some(date) = &config.date
        && !valid_date(date)
    {
        return Err(error(
            "date",
            &format!("'{date}' is not YYYY-MM-DD"),
            "use a calendar date in YYYY-MM-DD form",
        ));
    }
    if let Some(rates) = &config.rates
        && let Some((source, rate)) = rates
            .iter()
            .find(|(source, rate)| source.is_empty() || **rate == 0)
    {
        return Err(error(
            "rates",
            &format!("invalid rate for source '{source}': {rate}"),
            "use a positive requests-per-minute value for a named source",
        ));
    }
    Ok(())
}

fn enum_field(field: &str, value: Option<&str>, accepted: &[&str]) -> Result<(), ConfigError> {
    if let Some(value) = value
        && !accepted.contains(&value)
    {
        return Err(ConfigError {
            message: format!(
                "invalid {field} '{value}'; accepted values: {}; see `config schema --enum {field}`",
                accepted.join(", ")
            ),
        });
    }
    Ok(())
}

fn error(field: &str, problem: &str, remedy: &str) -> ConfigError {
    ConfigError {
        message: format!("invalid {field}: {problem}; remedy: {remedy}"),
    }
}

fn valid_date(value: &str) -> bool {
    let bytes = value.as_bytes();
    bytes.len() == 10
        && bytes[4] == b'-'
        && bytes[7] == b'-'
        && bytes
            .iter()
            .enumerate()
            .all(|(index, byte)| index == 4 || index == 7 || byte.is_ascii_digit())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn enum_error_lists_values() {
        let config = Config {
            job_type: Some("unknown".to_owned()),
            ..Config::default()
        };
        let error = validate(&config).unwrap_err();
        assert!(
            error
                .message
                .contains("fulltime, parttime, contract, internship")
        );
    }

    #[test]
    fn range_error_names_remedy() {
        let config = Config {
            min_score: Some(101),
            ..Config::default()
        };
        let error = validate(&config).unwrap_err();
        assert!(error.message.contains("remedy"));
    }

    #[test]
    fn date_requires_iso_form() {
        let config = Config {
            date: Some("2026/01/01".to_owned()),
            ..Config::default()
        };
        assert!(validate(&config).is_err());
        let config = Config {
            date: Some("2026-01-01".to_owned()),
            ..Config::default()
        };
        validate(&config).unwrap();
    }
}
