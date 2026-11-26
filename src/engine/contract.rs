//! Runtime contract for streams, exit codes, and non-interactive input.
//!
//! The contract is specified in
//! openspec/changes/oculus-foundation/specs/cli-config/spec.md.

// Rust guideline compliant 2026-10-09

use std::io::{self, Read, Write};

use serde::de::DeserializeOwned;
use serde_json::Value;

use crate::config::ConfigError;

/// Exit code for a clean run.
pub const EXIT_OK: i32 = 0;
/// Exit code for an internal failure.
pub const EXIT_INTERNAL: i32 = 1;
/// Exit code for invalid usage or configuration.
pub const EXIT_USAGE: i32 = 2;
/// Exit code for a partial run with per-source failures.
pub const EXIT_PARTIAL: i32 = 3;
/// Exit code for a run paused by a challenge.
pub const EXIT_CHALLENGE: i32 = 4;
/// Exit code for a cancelled run.
pub const EXIT_CANCELLED: i32 = 5;

/// A structured progress event written to stderr.
#[derive(Debug, Clone, PartialEq, Eq, serde::Serialize)]
pub struct ProgressEvent {
    /// Discriminator for the event kind.
    pub event: String,
    /// Identifier shared by all events in one run.
    pub run_id: String,
    /// Source associated with the event, when applicable.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub source_id: Option<String>,
    /// Number of records emitted so far.
    pub emitted: usize,
    /// Number of records skipped so far.
    pub skipped: usize,
    /// Status of the event.
    pub status: String,
}

/// Write one NDJSON event to a stream.
///
/// # Errors
/// Returns an io error when writing or flushing fails.
pub fn write_event(out: &mut impl Write, event: &ProgressEvent) -> io::Result<()> {
    let line = serde_json::to_string(event)
        .map_err(|error| io::Error::new(io::ErrorKind::InvalidData, error.to_string()))?;
    out.write_all(line.as_bytes())?;
    out.write_all(b"\n")?;
    out.flush()
}

/// Read and validate one JSON run-config document from stdin.
///
/// # Errors
/// Returns a config error for malformed JSON, a non-object document, or unknown keys.
pub fn stdin_config<T: DeserializeOwned>(input: &mut impl Read) -> Result<T, ConfigError> {
    let mut text = String::new();
    input
        .read_to_string(&mut text)
        .map_err(|error| ConfigError {
            message: format!("cannot read stdin: {error}"),
        })?;
    let value: Value = serde_json::from_str(&text).map_err(|error| ConfigError {
        message: format!("invalid stdin run-config: {error}"),
    })?;
    let object = value.as_object().ok_or_else(|| ConfigError {
        message: "stdin run-config must be a JSON object".to_owned(),
    })?;
    let allowed: Vec<&str> = crate::config::CONFIG_SCHEMA
        .iter()
        .map(|field| field.name)
        .collect();
    if let Some(key) = object.keys().find(|key| !allowed.contains(&key.as_str())) {
        return Err(ConfigError {
            message: format!("unknown stdin run-config key: {key}"),
        });
    }
    serde_json::from_value(Value::Object(object.clone())).map_err(|error| ConfigError {
        message: format!("invalid stdin run-config: {error}"),
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn events_serialize_as_ndjson() {
        let event = ProgressEvent {
            event: "run.completed".to_owned(),
            run_id: "run-1".to_owned(),
            source_id: None,
            emitted: 2,
            skipped: 1,
            status: "success".to_owned(),
        };
        let mut bytes = Vec::new();
        write_event(&mut bytes, &event).expect("event written");
        let text = String::from_utf8(bytes).expect("utf8");
        assert_eq!(text.lines().count(), 1);
        let parsed: Value = serde_json::from_str(text.trim_end()).expect("valid json");
        assert_eq!(parsed["event"], "run.completed");
        assert_eq!(parsed["emitted"], 2);
    }

    #[test]
    fn stdin_config_rejects_unknown_key() {
        let input = br#"{"format":"csv","unknown":1}"#;
        let result: Result<crate::config::Config, _> = stdin_config(&mut &input[..]);
        assert!(result.is_err());
        assert!(result.expect_err("error").message.contains("unknown"));
    }
}
