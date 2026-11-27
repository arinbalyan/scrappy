// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Arin Balyan
// Last modified: 2026-11-27

//! Offline checks that synthetic secrets never reach stdout or stderr.
//!
//! The bridge surfaces are specified in
//! openspec/specs/security-privacy/spec.md.

// Rust guideline compliant 2026-10-09

use std::io::Write;
use std::path::PathBuf;
use std::process::{Command, Output, Stdio};

const SECRETS: [&str; 15] = [
    "LEAK-WIRE-SEARCH-8f7a31c9d2",
    "LEAK-WIRE-SOURCES-6b4e02f7a1",
    "LEAK-WIRE-FORMAT-3c9d85e1f4",
    "LEAK-WIRE-RATES-1e6a7b92d8",
    "LEAK-WIRE-CONCURRENCY-9a2f4c6e10",
    "LEAK-WIRE-TIMEOUT-7d81b3e5c2",
    "LEAK-WIRE-MEMORY-2f5c9a7b41",
    "LEAK-WIRE-LOGLEVEL-4b8e1d6a93",
    "LEAK-WIRE-ISREMOTE-8c3f7a1d52",
    "LEAK-WIRE-JOBTYPE-5e9b2c8f17",
    "LEAK-WIRE-MINSCORE-6d1a4e9b38",
    "LEAK-WIRE-HOURSOLD-1c7f5b2a94",
    "LEAK-WIRE-DATE-9b3e6a2d75",
    "LEAK-WIRE-CHALLENGE-TOKEN-7a4c9e2f61",
    "LEAK-WIRE-REJECTED-5a1d8e3c72",
];

fn binary() -> Command {
    let mut command = Command::new(env!("CARGO_BIN_EXE_scrappy"));
    command
        .env("SCRAPPY_SEARCH", SECRETS[0])
        .env("SCRAPPY_SOURCES", SECRETS[1])
        .env("SCRAPPY_FORMAT", SECRETS[2])
        .env("SCRAPPY_RATES", SECRETS[3])
        .env("SCRAPPY_CONCURRENCY", SECRETS[4])
        .env("SCRAPPY_TIMEOUT", SECRETS[5])
        .env("SCRAPPY_MEMORY_MB", SECRETS[6])
        .env("SCRAPPY_LOG_LEVEL", SECRETS[7])
        .env("SCRAPPY_IS_REMOTE", SECRETS[8])
        .env("SCRAPPY_JOB_TYPE", SECRETS[9])
        .env("SCRAPPY_MIN_SCORE", SECRETS[10])
        .env("SCRAPPY_HOURS_OLD", SECRETS[11])
        .env("SCRAPPY_DATE", SECRETS[12])
        .env("CHALLENGE_TOKEN", SECRETS[13]);
    command
}

fn assert_clean(label: &str, stdout: &[u8], stderr: &[u8]) {
    let out = String::from_utf8_lossy(stdout);
    let err = String::from_utf8_lossy(stderr);
    for secret in SECRETS {
        assert!(!out.contains(secret), "{label} stdout leaked a secret");
        assert!(!err.contains(secret), "{label} stderr leaked a secret");
    }
}

fn run(args: &[&str], input: Option<&str>) -> Output {
    let mut command = binary();
    command.args(args);
    if input.is_some() {
        command.stdin(Stdio::piped());
    }
    let mut child = command.spawn().expect("binary runs");
    if let Some(text) = input {
        child
            .stdin
            .take()
            .expect("stdin piped")
            .write_all(text.as_bytes())
            .expect("input written");
    }
    child.wait_with_output().expect("output collected")
}

fn check_command(label: &str, args: &[&str], input: Option<&str>) -> Output {
    let output = run(args, input);
    assert_clean(label, &output.stdout, &output.stderr);
    output
}

fn temp_dir(name: &str) -> PathBuf {
    let dir = std::env::temp_dir().join(format!("scrappy-wire-{}-{name}", std::process::id()));
    let _ = std::fs::remove_dir_all(&dir);
    std::fs::create_dir_all(&dir).expect("temporary directory created");
    dir
}

#[test]
fn commands_keep_all_synthetic_secrets_off_both_streams() {
    let dir = temp_dir("streams");
    let config = dir.join("config.toml");
    let config = config.to_string_lossy();

    check_command("version", &["version"], None);
    check_command("validate", &["validate"], None);
    check_command("doctor", &["doctor", "--format", "json"], None);
    check_command("list-sources", &["list-sources", "--format", "json"], None);
    check_command("run", &["run", "--source", "remoteok"], None);
    check_command("config init", &["config", "init", "--path", &config], None);
    check_command("config show", &["config", "show", "--path", &config], None);
    check_command(
        "config explain",
        &["config", "explain", "--key", "format"],
        None,
    );

    let input = format!(r#"{{"search":"{}"}}"#, SECRETS[13]);
    check_command("stdin config", &["run", "--stdin-config"], Some(&input));

    std::fs::remove_dir_all(&dir).expect("temporary directory removed");
}

#[test]
fn artifacts_keep_synthetic_secrets_out_of_config_and_checkpoint_files() {
    let dir = temp_dir("artifacts");
    let config = dir.join("config.toml");
    std::fs::write(&config, "format = \"ndjson\"\n").expect("config seeded");

    let config_str = config.to_string_lossy();
    let set_args = [
        "config",
        "set",
        "--path",
        &config_str,
        "--key",
        "log_level",
        "--value",
        r#""debug""#,
    ];
    check_command("config set", &set_args, None);
    check_command(
        "config show",
        &["config", "show", "--path", &config_str],
        None,
    );

    let text = std::fs::read_to_string(&config).expect("config readable");
    for secret in SECRETS {
        assert!(!text.contains(secret), "config file leaked a secret");
    }

    // The runtime has no checkpoint-writing command; verify the resume surface
    // is derived from config values rather than copied from credential input.
    for entry in std::fs::read_dir(&dir).expect("directory readable") {
        let path = entry.expect("entry readable").path();
        if path.is_file() {
            let artifact = std::fs::read_to_string(&path).expect("artifact readable");
            for secret in SECRETS {
                assert!(
                    !artifact.contains(secret),
                    "{} leaked a secret",
                    path.display()
                );
            }
        }
    }

    std::fs::remove_dir_all(&dir).expect("temporary directory removed");
}

#[test]
fn invalid_config_diagnostics_redact_rejected_values() {
    let mut command = binary();
    let output = command
        .env("SCRAPPY_DATE", SECRETS[14])
        .args(["validate"])
        .output()
        .expect("binary runs");
    let stderr = String::from_utf8_lossy(&output.stderr);
    assert!(!output.status.success(), "invalid config must fail");
    assert!(!stderr.contains(SECRETS[14]), "diagnostic leaked a secret");
}
