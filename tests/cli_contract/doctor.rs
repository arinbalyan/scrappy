use std::process::Command;

fn run(args: &[&str]) -> std::process::Output {
    Command::new(env!("CARGO_BIN_EXE_scrappy"))
        .args(args)
        .output()
        .expect("binary runs")
}

#[test]
fn doctor_reports_all_named_checks_as_text() {
    let output = run(&["doctor"]);
    assert!(output.status.success());
    let text = String::from_utf8(output.stdout).expect("utf8");
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
}

#[test]
fn doctor_json_is_parseable() {
    let output = run(&["doctor", "--format", "json"]);
    assert!(output.status.success());
    let text = String::from_utf8(output.stdout).expect("utf8");
    let value: serde_json::Value = serde_json::from_str(&text).expect("valid json");
    assert!(value["checks"].is_array());
    assert_eq!(value["checks"].as_array().expect("array").len(), 9);
}

#[test]
fn doctor_fix_adds_fix_check() {
    let output = run(&["doctor", "--fix", "--format", "json"]);
    assert!(output.status.success());
    let text = String::from_utf8(output.stdout).expect("utf8");
    let value: serde_json::Value = serde_json::from_str(&text).expect("valid json");
    let names: Vec<&str> = value["checks"]
        .as_array()
        .expect("array")
        .iter()
        .map(|c| c["name"].as_str().expect("name"))
        .collect();
    assert!(names.contains(&"fix"));
}
