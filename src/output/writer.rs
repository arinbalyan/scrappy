//! Serialization and atomic artifact writes.
//!
//! See openspec/changes/oculus-foundation/specs/output-contract/spec.md.

// Rust guideline compliant 2026-10-09

use std::collections::hash_map::DefaultHasher;
use std::fmt::Write as FmtWrite;
use std::fs;
use std::hash::{Hash, Hasher};
use std::io::{self, Write};
use std::path::{Path, PathBuf};

use serde::{Deserialize, Serialize};

use super::record::Job;

/// Output format selected by the caller.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Format {
    /// Newline-delimited JSON.
    Ndjson,
    /// One JSON array.
    Json,
    /// Comma-separated values with a header.
    Csv,
}

impl Format {
    /// Parse one format name.
    ///
    /// # Errors
    /// Returns a diagnostic listing the accepted names for an unknown value.
    pub fn parse(name: &str) -> Result<Self, String> {
        match name.to_ascii_lowercase().as_str() {
            "ndjson" => Ok(Self::Ndjson),
            "json" => Ok(Self::Json),
            "csv" => Ok(Self::Csv),
            other => Err(format!(
                "unknown output format: {other}; expected ndjson, json, or csv"
            )),
        }
    }
}

/// CSV columns in stable order.
pub const CSV_COLUMNS: &[&str] = &[
    "schema_version",
    "source_id",
    "external_id",
    "canonical_url",
    "title",
    "company",
    "location",
    "description",
    "fetched_at",
    "content_hash",
    "is_remote",
    "date_posted",
    "compensation",
    "apply_method",
    "job_url_direct",
    "company_url",
    "company_domain",
    "company_description",
    "industry",
    "quality_score",
    "source_ids",
    "emails",
];

/// Escape one field for CSV.
#[must_use]
pub fn csv_field(value: &str) -> String {
    if value.contains([',', '"', '\n', '\r']) {
        format!("\"{}\"", value.replace('"', "\"\""))
    } else {
        value.to_owned()
    }
}

fn flatten(value: &serde_json::Value) -> String {
    match value {
        serde_json::Value::Null => String::new(),
        serde_json::Value::Bool(flag) => flag.to_string(),
        serde_json::Value::Number(number) => number.to_string(),
        serde_json::Value::String(text) => text.clone(),
        serde_json::Value::Array(_) | serde_json::Value::Object(_) => {
            serde_json::to_string(value).unwrap_or_default()
        }
    }
}

/// Encode jobs in the selected format.
///
/// # Errors
/// Returns a diagnostic when a record cannot be serialized.
pub fn encode(jobs: &[Job], format: Format) -> Result<String, String> {
    match format {
        Format::Ndjson => {
            let mut out = String::new();
            for job in jobs {
                let line = serde_json::to_string(job)
                    .map_err(|error| format!("cannot encode job: {error}"))?;
                out.push_str(&line);
                out.push('\n');
            }
            Ok(out)
        }
        Format::Json => serde_json::to_string_pretty(jobs)
            .map(|mut text| {
                text.push('\n');
                text
            })
            .map_err(|error| format!("cannot encode jobs: {error}")),
        Format::Csv => {
            let mut out = String::new();
            writeln!(out, "{}", CSV_COLUMNS.join(",")).expect("writing to string");
            for job in jobs {
                let value = serde_json::to_value(job)
                    .map_err(|error| format!("cannot encode job: {error}"))?;
                let cells = CSV_COLUMNS
                    .iter()
                    .map(|column| {
                        let field = value.get(column).map_or(String::new(), flatten);
                        csv_field(&field)
                    })
                    .collect::<Vec<_>>()
                    .join(",");
                writeln!(out, "{cells}").expect("writing to string");
            }
            Ok(out)
        }
    }
}

/// Write one complete NDJSON line and flush it immediately.
///
/// # Errors
/// Returns an I/O error when writing or flushing fails.
pub fn write_line(out: &mut impl Write, job: &Job) -> io::Result<()> {
    let line = serde_json::to_string(job)
        .map_err(|error| io::Error::new(io::ErrorKind::InvalidData, error.to_string()))?;
    out.write_all(line.as_bytes())?;
    out.write_all(b"\n")?;
    out.flush()
}

/// Hash bytes for an artifact manifest.
#[must_use]
pub fn artifact_hash(bytes: &[u8]) -> u64 {
    let mut hasher = DefaultHasher::new();
    bytes.hash(&mut hasher);
    hasher.finish()
}

/// Manifest describing one saved artifact.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct Manifest {
    /// Name of the artifact.
    pub artifact: String,
    /// Byte length of the artifact.
    pub size: usize,
    /// Hash of the artifact bytes.
    pub hash: u64,
}

/// Write an artifact privately and atomically, then write its manifest.
///
/// # Errors
/// Returns a diagnostic when a temporary write, permission change, rename,
/// or manifest write fails. The final path is unchanged on failure.
pub fn save(path: &Path, bytes: &[u8]) -> Result<Manifest, String> {
    let temp = temp_path(path);
    let mut file = fs::File::create(&temp)
        .map_err(|error| format!("cannot create {}: {error}", temp.display()))?;
    file.write_all(bytes)
        .map_err(|error| format!("cannot write {}: {error}", temp.display()))?;
    file.sync_all()
        .map_err(|error| format!("cannot sync {}: {error}", temp.display()))?;
    restrict_file(&temp).map_err(|error| format!("cannot secure {}: {error}", temp.display()))?;
    fs::rename(&temp, path)
        .map_err(|error| format!("cannot replace {}: {error}", path.display()))?;
    let manifest = Manifest {
        artifact: path
            .file_name()
            .and_then(|name| name.to_str())
            .unwrap_or("result")
            .to_owned(),
        size: bytes.len(),
        hash: artifact_hash(bytes),
    };
    let manifest_path = path.with_extension("manifest.json");
    let text = serde_json::to_string_pretty(&manifest)
        .map_err(|error| format!("cannot encode manifest: {error}"))?;
    let manifest_temp = temp_path(&manifest_path);
    fs::write(&manifest_temp, text).map_err(|error| format!("cannot write manifest: {error}"))?;
    restrict_file(&manifest_temp).map_err(|error| format!("cannot secure manifest: {error}"))?;
    fs::rename(&manifest_temp, &manifest_path)
        .map_err(|error| format!("cannot install manifest: {error}"))?;
    Ok(manifest)
}

/// Verify that an artifact still matches its manifest.
///
/// # Errors
/// Returns a diagnostic naming the artifact when it is unreadable or changed.
pub fn verify(path: &Path, manifest: &Manifest) -> Result<(), String> {
    let bytes =
        fs::read(path).map_err(|error| format!("cannot read {}: {error}", path.display()))?;
    if bytes.len() != manifest.size || artifact_hash(&bytes) != manifest.hash {
        return Err(format!("artifact changed: {}", manifest.artifact));
    }
    Ok(())
}

/// Read and verify a saved manifest.
///
/// # Errors
/// Returns a diagnostic when the manifest is unreadable or malformed.
pub fn read_manifest(path: &Path) -> Result<Manifest, String> {
    let text = fs::read_to_string(path)
        .map_err(|error| format!("cannot read {}: {error}", path.display()))?;
    serde_json::from_str(&text)
        .map_err(|error| format!("invalid manifest {}: {error}", path.display()))
}

fn temp_path(path: &Path) -> PathBuf {
    let name = path
        .file_name()
        .and_then(|name| name.to_str())
        .unwrap_or("result");
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

#[cfg(test)]
mod tests {
    use super::*;
    use crate::output::record::{
        Email, EmailSource, JobInput, Provenance, SCHEMA_VERSION, from_record,
    };
    use crate::parsing::Record;

    fn job(description: &str) -> Job {
        let record = Record::new("https://example.test/jobs/1", "Rust Engineer", "Example Co")
            .expect("valid record")
            .with_description(Some(description.to_owned()));
        let input = JobInput {
            source_id: "greenhouse".to_owned(),
            fetched_at: "2026-11-26T00:00:00Z".to_owned(),
            emails: vec![Email {
                addr: "jobs@example.test".to_owned(),
                verified: true,
                source: EmailSource::Description,
                role: None,
                provenance: Provenance::Observed,
            }],
            ..JobInput::default()
        };
        from_record(&record, &input, SCHEMA_VERSION).expect("valid job")
    }

    #[test]
    fn every_format_is_valid() {
        let jobs = vec![job("Build things")];
        let ndjson = encode(&jobs, Format::Ndjson).expect("ndjson");
        let parsed: serde_json::Value = serde_json::from_str(ndjson.trim()).expect("valid ndjson");
        assert_eq!(parsed["title"], "Rust Engineer");
        let json = encode(&jobs, Format::Json).expect("json");
        let parsed: serde_json::Value = serde_json::from_str(&json).expect("valid json");
        assert!(parsed.is_array());
        let csv = encode(&jobs, Format::Csv).expect("csv");
        assert_eq!(csv.lines().count(), 2);
    }

    #[test]
    fn csv_escapes_special_characters() {
        assert_eq!(csv_field("a,b"), "\"a,b\"");
        assert_eq!(csv_field("a\"b"), "\"a\"\"b\"");
        assert_eq!(csv_field("a\nb"), "\"a\nb\"");
        let csv = encode(&[job("line one\nline two, with comma")], Format::Csv).expect("csv");
        assert!(csv.contains("\"line one\nline two, with comma\""));
    }

    #[test]
    fn save_and_verify_artifact() {
        let dir = std::env::temp_dir().join(format!("scrappy-output-{}", std::process::id()));
        fs::create_dir_all(&dir).expect("temp directory");
        let path = dir.join("result.json");
        let bytes = encode(&[job("Build things")], Format::Ndjson).expect("ndjson");
        let manifest = save(&path, bytes.as_bytes()).expect("saved");
        assert_eq!(manifest.hash, artifact_hash(bytes.as_bytes()));
        verify(&path, &manifest).expect("unchanged artifact");
        let loaded = read_manifest(&path.with_extension("manifest.json")).expect("manifest");
        assert_eq!(loaded, manifest);
        fs::write(&path, b"changed").expect("corrupt artifact");
        assert!(verify(&path, &manifest).is_err());
        fs::remove_dir_all(&dir).expect("temp directory removed");
    }
}
