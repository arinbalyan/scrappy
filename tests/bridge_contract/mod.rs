use std::io::Write;
use std::process::{Command, Output, Stdio};

fn run_with_stdin(args: &[&str], input: &str) -> Output {
    let mut child = Command::new(env!("CARGO_BIN_EXE_scrappy"))
        .args(args)
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()
        .expect("binary runs");
    child
        .stdin
        .take()
        .expect("stdin piped")
        .write_all(input.as_bytes())
        .expect("input written");
    child.wait_with_output().expect("output collected")
}

#[test]
fn schema_prints_enum_values() {
    for (field, values) in [
        ("format", "ndjson, json, csv"),
        ("log_level", "error, warn, info, debug, trace"),
    ] {
        let output = Command::new(env!("CARGO_BIN_EXE_scrappy"))
            .args(["config", "schema", "--enum", field])
            .output()
            .expect("binary runs");
        assert!(output.status.success());
        let text = String::from_utf8(output.stdout).expect("utf8");
        assert_eq!(text.trim_end(), values);
    }
}

#[test]
fn unknown_enum_field_exits_two() {
    let output = Command::new(env!("CARGO_BIN_EXE_scrappy"))
        .args(["config", "schema", "--enum", "nope"])
        .output()
        .expect("binary runs");
    assert_eq!(output.status.code(), Some(2));
}

#[test]
fn valid_stdin_config_produces_no_stdout_and_no_fetch() {
    let output = run_with_stdin(&["run", "--stdin-config"], r#"{"format":"csv"}"#);
    assert!(
        output.status.success(),
        "stderr: {}",
        String::from_utf8_lossy(&output.stderr)
    );
    assert_eq!(output.stdout, [] as [u8; 0]);
    assert_ne!(output.stderr, [] as [u8; 0]);
}

#[test]
fn malformed_stdin_config_exits_two_before_fetch() {
    for input in [r#"{"unknown":1}"#, r#"{"format":"xml"}"#, "not json"] {
        let output = run_with_stdin(&["run", "--stdin-config"], input);
        assert_eq!(output.status.code(), Some(2), "input: {input}");
        assert_eq!(
            output.stdout,
            [] as [u8; 0],
            "input emitted stdout: {input}"
        );
        assert_ne!(output.stderr, [] as [u8; 0]);
    }
}
