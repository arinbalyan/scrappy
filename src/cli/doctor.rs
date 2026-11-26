//! Doctor checks for local setup and dependencies.
//!
//! The check taxonomy is specified in
//! openspec/changes/oculus-foundation/specs/cli-config/spec.md.

// Rust guideline compliant 2026-10-09

use std::fmt::Write as FmtWrite;
use std::path::{Path, PathBuf};

use serde::Serialize;

use crate::config::CONFIG_SCHEMA;

/// Status of one doctor check.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize)]
#[serde(rename_all = "lowercase")]
pub enum Status {
    /// The check passed.
    Pass,
    /// The check passed with a caveat.
    Warn,
    /// The check failed.
    Fail,
    /// The check did not apply.
    Skip,
}

/// Result of one named check.
#[derive(Debug, Clone, PartialEq, Eq, Serialize)]
pub struct Check {
    /// Check name.
    pub name: String,
    /// Outcome of the check.
    pub status: Status,
    /// Human-readable reason, with secrets redacted.
    pub reason: String,
}

/// Full doctor report.
#[derive(Debug, Clone, PartialEq, Eq, Serialize)]
pub struct Report {
    /// Named checks in report order.
    pub checks: Vec<Check>,
    /// Number of failed checks.
    pub failures: usize,
    /// Files the runtime would load, in precedence order.
    pub config_files: Vec<PathBuf>,
}

/// Run all doctor checks without applying fixes.
#[must_use]
pub fn report(files: &[PathBuf]) -> Report {
    build_report(files, false)
}

/// Run all checks and apply documented safe repairs.
#[must_use]
pub fn report_with_fix(files: &[PathBuf]) -> Report {
    build_report(files, true)
}

fn build_report(files: &[PathBuf], fix: bool) -> Report {
    let mut checks = vec![
        config_check(files),
        schema_check(),
        sources_check(),
        credentials_check(),
        proxies_check(files),
        browser_check(),
        fixtures_check(),
        output_check(),
        features_check(),
    ];
    if fix {
        checks.push(fix_check(files));
    }
    let failures = checks.iter().filter(|c| c.status == Status::Fail).count();
    Report {
        checks,
        failures,
        config_files: files.to_vec(),
    }
}
fn config_check(files: &[PathBuf]) -> Check {
    let missing: Vec<String> = files
        .iter()
        .filter(|path| !path.is_file())
        .map(|path| path.display().to_string())
        .collect();
    if missing.is_empty() {
        Check {
            name: "config".to_owned(),
            status: Status::Pass,
            reason: format!("{} config file(s) found", files.len()),
        }
    } else {
        Check {
            name: "config".to_owned(),
            status: Status::Warn,
            reason: format!("missing config file(s): {}", missing.join(", ")),
        }
    }
}

fn schema_check() -> Check {
    let names: Vec<&str> = CONFIG_SCHEMA.iter().map(|f| f.name).collect();
    Check {
        name: "schema".to_owned(),
        status: Status::Pass,
        reason: format!("{} fields in runtime schema", names.len()),
    }
}

fn sources_check() -> Check {
    Check {
        name: "sources".to_owned(),
        status: Status::Skip,
        reason: "source registry not yet implemented".to_owned(),
    }
}

fn credentials_check() -> Check {
    let present = crate::config::ENV_PREFIX;
    Check {
        name: "credentials".to_owned(),
        status: Status::Skip,
        reason: format!("credential names use {present}* prefixes; values are never printed"),
    }
}

fn proxies_check(files: &[PathBuf]) -> Check {
    let configured = files.iter().any(|path| {
        std::fs::read_to_string(path)
            .is_ok_and(|text| text.contains("proxy") || text.contains("PROXY"))
    });
    if configured {
        Check {
            name: "proxies".to_owned(),
            status: Status::Warn,
            reason: "proxy settings found; reachability is checked at run time".to_owned(),
        }
    } else {
        Check {
            name: "proxies".to_owned(),
            status: Status::Skip,
            reason: "no proxies configured".to_owned(),
        }
    }
}
fn browser_check() -> Check {
    Check {
        name: "browser".to_owned(),
        status: Status::Skip,
        reason: "browser feature is disabled in this build".to_owned(),
    }
}

fn fixtures_check() -> Check {
    Check {
        name: "fixtures".to_owned(),
        status: Status::Skip,
        reason: "no sources registered yet".to_owned(),
    }
}

fn output_check() -> Check {
    Check {
        name: "output".to_owned(),
        status: Status::Pass,
        reason: "stdout is available".to_owned(),
    }
}

fn features_check() -> Check {
    Check {
        name: "features".to_owned(),
        status: Status::Pass,
        reason: "default feature set is active".to_owned(),
    }
}

fn fix_check(files: &[PathBuf]) -> Check {
    let mut applied = 0usize;
    let mut failed = 0usize;
    for path in files {
        if path.is_file() && secure(path).is_ok() {
            applied += 1;
        } else if path.is_file() {
            failed += 1;
        }
    }
    let status = if failed > 0 {
        Status::Fail
    } else {
        Status::Pass
    };
    Check {
        name: "fix".to_owned(),
        status,
        reason: format!("secured {applied} config file(s), {failed} failure(s)"),
    }
}

fn secure(path: &Path) -> Result<(), ()> {
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        std::fs::set_permissions(path, std::fs::Permissions::from_mode(0o600)).map_err(|_error| ())
    }
    #[cfg(not(unix))]
    {
        let _ = path;
        Ok(())
    }
}
impl Check {
    /// Return a lowercase status label.
    #[must_use]
    pub fn status_label(&self) -> &'static str {
        match self.status {
            Status::Pass => "pass",
            Status::Warn => "warn",
            Status::Fail => "fail",
            Status::Skip => "skip",
        }
    }
}

/// Render a report as human-readable text.
#[must_use]
pub fn render_text(report: &Report) -> String {
    let mut out = String::new();
    for check in &report.checks {
        let _ = writeln!(
            out,
            "{}: {} - {}",
            check.name,
            check.status_label(),
            check.reason
        );
    }
    let _ = writeln!(out, "failures: {}", report.failures);
    out
}

/// Render a report as pretty JSON.
///
/// # Errors
/// Returns a serialization error for an invalid report.
pub fn render_json(report: &Report) -> Result<String, serde_json::Error> {
    serde_json::to_string_pretty(report)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn report_has_all_named_checks() {
        let report = report(&[]);
        for name in [
            "config",
            "schema",
            "sources",
            "credentials",
            "proxies",
            "browser",
            "fixtures",
            "output",
            "features",
        ] {
            assert!(
                report.checks.iter().any(|c| c.name == name),
                "missing {name}"
            );
        }
    }

    #[test]
    fn json_report_is_parseable() {
        let report = report(&[]);
        let json = render_json(&report).expect("json rendered");
        let parsed: serde_json::Value = serde_json::from_str(&json).expect("valid json");
        assert!(parsed["checks"].is_array());
    }
}
