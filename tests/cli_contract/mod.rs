mod doctor;

use std::process::{Command, Output};

fn run(args: &[&str]) -> Output {
    Command::new(env!("CARGO_BIN_EXE_scrappy"))
        .args(args)
        .output()
        .expect("binary runs")
}

fn stdout(output: &Output) -> String {
    String::from_utf8(output.stdout.clone()).expect("stdout is UTF-8")
}

#[test]
fn help_lists_every_subcommand() {
    let output = run(&["--help"]);
    assert!(output.status.success());
    let text = stdout(&output);
    for name in [
        "run",
        "validate",
        "config",
        "list-sources",
        "doctor",
        "completion",
        "version",
    ] {
        assert!(text.contains(name), "help omitted {name}");
    }
    assert!(text.contains("Example:"));
}

#[test]
fn every_subcommand_has_help_and_example() {
    for name in [
        "run",
        "validate",
        "config",
        "list-sources",
        "doctor",
        "completion",
        "version",
    ] {
        let output = run(&[name, "--help"]);
        assert!(output.status.success(), "{name} --help failed");
        assert!(
            stdout(&output).contains("Example:"),
            "{name} has no example"
        );
    }
}

#[test]
fn version_works_at_root_and_subcommand() {
    for args in [vec!["--version"], vec!["run", "--version"]] {
        let output = run(&args);
        assert!(output.status.success());
        assert!(stdout(&output).contains("0.1.0"));
    }
}

#[test]
fn completion_supports_shells_and_rejects_unknown() {
    for shell in ["bash", "zsh", "fish", "powershell", "elvish"] {
        let output = run(&["completion", shell]);
        assert!(output.status.success(), "completion {shell} failed");
        assert_ne!(stdout(&output), "");
    }
    let output = run(&["completion", "not-a-shell"]);
    assert_eq!(output.status.code(), Some(2));
}

#[test]
fn unknown_subcommand_exits_two() {
    let output = run(&["not-a-command"]);
    assert_eq!(output.status.code(), Some(2));
    assert_ne!(output.stderr, [] as [u8; 0]);
}
