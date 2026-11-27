// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Arin Balyan
// Last modified: 2026-11-27

// Rust guideline compliant 2026-10-09

use std::fs;
use std::path::PathBuf;
use std::process::{Command, Output};
use std::time::{SystemTime, UNIX_EPOCH};

fn binary() -> Command {
    let mut command = Command::new(env!("CARGO_BIN_EXE_scrappy"));
    for field in scrappy::config::CONFIG_SCHEMA {
        command.env_remove(format!(
            "{}{}",
            scrappy::config::ENV_PREFIX,
            field.name.to_uppercase()
        ));
    }
    command
}

fn stdout(output: &Output) -> String {
    String::from_utf8(output.stdout.clone()).expect("stdout is UTF-8")
}

struct Sandbox {
    root: PathBuf,
}

impl Sandbox {
    fn new(label: &str) -> Self {
        let nanos = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .expect("clock after epoch")
            .as_nanos();
        let root = std::env::temp_dir().join(format!("scrappy-doctor-{label}-{nanos}"));
        fs::create_dir_all(&root).expect("temp dir created");
        Self { root }
    }

    fn write(&self, name: &str, text: &str) -> PathBuf {
        let path = self.root.join(name);
        fs::write(&path, text).expect("file written");
        path
    }
}

impl Drop for Sandbox {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.root);
    }
}

fn doctor_in(sandbox: &Sandbox, args: &[&str]) -> Output {
    let mut command = binary();
    command.current_dir(&sandbox.root);
    command.args(["doctor", "--format", "json"]).args(args);
    command.output().expect("doctor runs")
}

fn json_output(output: &Output) -> serde_json::Value {
    serde_json::from_str(&stdout(output)).expect("doctor emits one JSON object")
}

fn check<'a>(value: &'a serde_json::Value, name: &str) -> &'a serde_json::Value {
    value["checks"]
        .as_array()
        .expect("checks array")
        .iter()
        .find(|entry| entry["name"] == name)
        .unwrap_or_else(|| panic!("missing check {name}"))
}

#[test]
fn doctor_reports_all_named_checks_as_text() {
    let output = binary().arg("doctor").output().expect("doctor runs");
    assert!(output.status.success());
    let text = stdout(&output);
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
        assert!(text.contains(&format!("{name}:")), "missing {name}");
    }
    assert!(text.contains("failures: "));
}

#[test]
fn doctor_json_is_parseable() {
    let output = binary()
        .args(["doctor", "--format", "json"])
        .output()
        .expect("doctor runs");
    assert!(output.status.success());
    let value = json_output(&output);
    assert_eq!(value["checks"].as_array().expect("array").len(), 9);
    assert!(value["failures"].is_u64());
    assert!(value["config_files"].is_array());
}

#[test]
fn doctor_text_and_json_report_the_same_checks_and_failures() {
    let text = binary().arg("doctor").output().expect("doctor runs");
    let json = binary()
        .args(["doctor", "--format", "json"])
        .output()
        .expect("doctor runs");
    assert!(text.status.success() && json.status.success());
    let value = json_output(&json);
    let json_failures = value["failures"].as_u64().expect("failure count");
    let text_failures = stdout(&text)
        .lines()
        .find_map(|line| line.strip_prefix("failures: "))
        .expect("failure line")
        .parse::<u64>()
        .expect("number");
    assert_eq!(text_failures, json_failures);

    let mut failed = 0usize;
    for entry in value["checks"].as_array().expect("checks array") {
        let name = entry["name"].as_str().expect("name");
        let status = entry["status"].as_str().expect("status");
        assert!(
            stdout(&text).contains(&format!("{name}: {status} - ")),
            "text and json differ for {name}"
        );
        if status == "fail" {
            failed += 1;
        }
    }
    assert_eq!(failed as u64, json_failures);
}

#[test]
fn doctor_reports_explicit_config_files_in_order() {
    let sandbox = Sandbox::new("files");
    let first = sandbox.write(
        "first.toml",
        "format = 'csv'
",
    );
    let second = sandbox.write(
        "second.toml",
        "format = 'json'
",
    );
    let output = doctor_in(
        &sandbox,
        &[
            "--config",
            first.to_str().expect("utf8 path"),
            "--config",
            second.to_str().expect("utf8 path"),
        ],
    );
    assert!(output.status.success());
    let value = json_output(&output);
    let files: Vec<&str> = value["config_files"]
        .as_array()
        .expect("array")
        .iter()
        .map(|entry| entry.as_str().expect("path"))
        .collect();
    assert_eq!(
        files,
        vec![
            first.to_str().expect("utf8 path"),
            second.to_str().expect("utf8 path")
        ]
    );
    assert_eq!(check(&value, "config")["status"], "pass");
}

#[test]
fn doctor_warns_on_missing_config_without_stderr() {
    let sandbox = Sandbox::new("missing");
    let missing = sandbox.root.join("absent.toml");
    let output = doctor_in(&sandbox, &["--config", missing.to_str().expect("utf8")]);
    assert!(output.status.success());
    assert!(output.stderr.is_empty(), "stderr must stay machine-clean");
    let value = json_output(&output);
    assert_eq!(check(&value, "config")["status"], "warn");
    assert!(
        check(&value, "config")["reason"]
            .as_str()
            .expect("reason")
            .contains("absent.toml")
    );
}

#[test]
fn doctor_hides_proxy_credentials() {
    let sandbox = Sandbox::new("secrets");
    let path = sandbox.write(
        "config.toml",
        "proxy = 'https://user:s3cr3t-value@proxy.test'
",
    );
    let output = doctor_in(&sandbox, &["--config", path.to_str().expect("utf8")]);
    assert!(output.status.success());
    assert_eq!(check(&json_output(&output), "proxies")["status"], "warn");
    for text in [
        stdout(&output),
        String::from_utf8_lossy(&output.stderr).into_owned(),
    ] {
        assert!(!text.contains("s3cr3t-value"));
        assert!(!text.contains("user:s3cr3t-value"));
    }
}

#[test]
fn doctor_fix_adds_fix_check_and_counts_repairs() {
    let sandbox = Sandbox::new("fix-ok");
    let path = sandbox.write(
        "config.toml",
        "format = 'csv'
",
    );
    let output = doctor_in(
        &sandbox,
        &["--fix", "--config", path.to_str().expect("utf8")],
    );
    assert!(output.status.success());
    let value = json_output(&output);
    assert_eq!(value["checks"].as_array().expect("array").len(), 10);
    let fix = check(&value, "fix");
    assert_eq!(fix["status"], "pass");
    assert_eq!(fix["reason"], "secured 1 config file(s), 0 failure(s)");
}

#[test]
fn doctor_fix_counts_a_failed_repair_as_a_failure() {
    let sandbox = Sandbox::new("fix-fail");
    let path = sandbox.write(
        "config.toml",
        "format = 'csv'
",
    );
    let before = json_output(&doctor_in(
        &sandbox,
        &["--config", path.to_str().expect("utf8")],
    ))["failures"]
        .as_u64()
        .expect("baseline failures");

    let output = doctor_in(
        &sandbox,
        &[
            "--fix",
            "--config",
            "/proc/self/status",
            "--config",
            path.to_str().expect("utf8"),
        ],
    );
    assert!(
        output.status.success(),
        "fix failures do not change exit code"
    );
    let value = json_output(&output);
    let fix = check(&value, "fix");
    assert_eq!(fix["status"], "fail");
    assert!(
        fix["reason"]
            .as_str()
            .expect("reason")
            .contains("1 failure(s)")
    );
    assert_eq!(value["failures"].as_u64().expect("count"), before + 1);
}
