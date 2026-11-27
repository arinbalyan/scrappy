// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Arin Balyan
// Last modified: 2026-11-27

// Rust guideline compliant 2026-10-09

mod doctor;

use std::fs;
use std::path::{Path, PathBuf};
use std::process::{Command, Output};
use std::time::{SystemTime, UNIX_EPOCH};

use scrappy::config::{CONFIG_SCHEMA, ENV_PREFIX};

const COMMANDS: &[&str] = &[
    "run",
    "validate",
    "config",
    "list-sources",
    "doctor",
    "completion",
    "version",
];

struct Sandbox {
    root: PathBuf,
}

impl Sandbox {
    fn new(label: &str) -> Self {
        let nanos = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .expect("clock after epoch")
            .as_nanos();
        let root = std::env::temp_dir().join(format!("scrappy-cli-{label}-{nanos}"));
        let user = root.join("home").join(".config").join("scrappy");
        fs::create_dir_all(&user).expect("user config dir created");
        fs::create_dir_all(root.join("work")).expect("work dir created");
        Self { root }
    }

    fn user_config(&self) -> PathBuf {
        self.root
            .join("home")
            .join(".config")
            .join("scrappy")
            .join("config.toml")
    }

    fn project_config(&self) -> PathBuf {
        self.root.join("work").join("config.toml")
    }

    fn write(&self, path: &Path, text: &str) {
        let _ = self;
        if let Some(parent) = path.parent() {
            fs::create_dir_all(parent).expect("parent created");
        }
        fs::write(path, text).expect("config written");
    }

    fn command(&self) -> Command {
        let mut command = binary();
        command
            .env("HOME", self.root.join("home"))
            .current_dir(self.root.join("work"));
        command
    }
}

impl Drop for Sandbox {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.root);
    }
}

fn binary() -> Command {
    let mut command = Command::new(env!("CARGO_BIN_EXE_scrappy"));
    for field in CONFIG_SCHEMA {
        command.env_remove(format!("{ENV_PREFIX}{}", field.name.to_uppercase()));
    }
    command
}

fn stdout(output: &Output) -> String {
    String::from_utf8(output.stdout.clone()).expect("stdout is UTF-8")
}

fn stderr(output: &Output) -> String {
    String::from_utf8(output.stderr.clone()).expect("stderr is UTF-8")
}

fn validate_in(sandbox: &Sandbox, args: &[&str]) -> Output {
    let mut command = sandbox.command();
    command.arg("validate").args(args);
    command.output().expect("validate runs")
}

fn invalid_marker() -> &'static str {
    "log_level = 'loud'"
}

fn assert_validated(output: &Output) {
    assert!(output.status.success(), "stderr: {}", stderr(output));
    let mut expected = String::from("ok");
    expected.push(char::from(10));
    assert_eq!(stdout(output), expected);
}

#[test]
fn help_lists_every_subcommand() {
    let output = binary().arg("--help").output().expect("help runs");
    assert!(output.status.success());
    let text = stdout(&output);
    for name in COMMANDS {
        assert!(text.contains(name), "help omitted {name}");
    }
    assert!(text.contains("Example:"));
}

#[test]
fn config_reading_subcommands_expose_global_flags() {
    for name in ["validate", "doctor", "list-sources"] {
        let output = binary()
            .args([name, "--help"])
            .output()
            .expect("subcommand help runs");
        assert!(output.status.success());
        let text = stdout(&output);
        assert!(text.contains("--config"), "{name} has no --config");
        assert!(
            text.contains("--no-default-config"),
            "{name} has no --no-default-config"
        );
    }
}

#[test]
fn every_subcommand_has_help_and_example() {
    for name in COMMANDS {
        let output = binary()
            .args([name, "--help"])
            .output()
            .expect("subcommand help runs");
        assert!(output.status.success(), "{name} --help failed");
        assert!(
            stdout(&output).contains("Example:"),
            "{name} has no example"
        );
    }
}

#[test]
fn version_works_at_root_and_subcommand() {
    for args in [vec!["--version"], vec!["run", "--version"], vec!["version"]] {
        let output = binary().args(&args).output().expect("version runs");
        assert!(output.status.success(), "args: {args:?}");
        assert!(stdout(&output).contains("0.1.0"));
    }
}

#[test]
fn completion_supports_shells_and_rejects_unknown() {
    for shell in ["bash", "zsh", "fish", "powershell", "elvish"] {
        let output = binary()
            .args(["completion", shell])
            .output()
            .expect("completion runs");
        assert!(output.status.success(), "completion {shell} failed");
        assert_ne!(stdout(&output), "");
    }
    let output = binary()
        .args(["completion", "not-a-shell"])
        .output()
        .expect("completion runs");
    assert_eq!(output.status.code(), Some(2));
    assert_ne!(stderr(&output), "");
}

#[test]
fn help_and_version_ignore_broken_config_and_environment() {
    let sandbox = Sandbox::new("help");
    sandbox.write(&sandbox.project_config(), "this is not = toml =");
    sandbox.write(&sandbox.user_config(), "unknown_key = 1");
    for args in [
        vec!["--help"],
        vec!["validate", "--help"],
        vec!["--version"],
        vec!["version"],
        vec!["completion", "bash"],
    ] {
        let output = sandbox
            .command()
            .args(&args)
            .output()
            .expect("command runs");
        assert!(output.status.success(), "args: {args:?}");
        assert_ne!(stdout(&output), "");
    }
}

#[test]
fn version_commands_are_independent_of_bad_environment() {
    let output = binary()
        .env("SCRAPPY_FORMAT", "xml")
        .args(["version"])
        .output()
        .expect("version runs");
    assert!(output.status.success());
    assert!(stdout(&output).contains("scrappy 0.1.0"));
}

#[test]
fn invalid_arguments_exit_two_with_diagnostics() {
    for args in [
        vec!["not-a-command"],
        vec!["validate", "--not-a-flag"],
        vec!["config", "schema", "--enum"],
        vec!["completion"],
    ] {
        let output = binary().args(&args).output().expect("command runs");
        assert_eq!(output.status.code(), Some(2), "args: {args:?}");
        assert!(!stderr(&output).is_empty(), "args: {args:?}");
    }
}

#[test]
fn invalid_enum_lists_accepted_values() {
    let sandbox = Sandbox::new("enum");
    sandbox.write(&sandbox.project_config(), "log_level = 'loud'");
    let output = validate_in(&sandbox, &[]);
    assert_eq!(output.status.code(), Some(2));
    let text = stderr(&output);
    assert!(text.contains("log_level"));
    assert!(text.contains("error, warn, info, debug, trace"));
    assert!(text.contains("config schema --enum log_level"));
}

#[test]
fn unknown_source_exits_two_on_run() {
    let output = binary()
        .args(["run", "--source", "nope"])
        .output()
        .expect("run runs");
    assert_eq!(output.status.code(), Some(2));
    assert!(stderr(&output).contains("unknown source: nope"));
    assert_eq!(stdout(&output), "");
}

#[test]
fn malformed_explicit_config_exits_two_and_names_the_file() {
    let sandbox = Sandbox::new("malformed");
    let path = sandbox.root.join("work").join("broken.toml");
    sandbox.write(&path, "format = ");
    let output = validate_in(&sandbox, &["--config", path.to_str().expect("utf8 path")]);
    assert_eq!(output.status.code(), Some(2));
    assert!(stderr(&output).contains("broken.toml"));
}

#[test]
fn unknown_config_key_exits_two_and_lists_valid_fields() {
    let sandbox = Sandbox::new("unknown");
    sandbox.write(&sandbox.project_config(), "unsupported = 1");
    let output = validate_in(&sandbox, &[]);
    assert_eq!(output.status.code(), Some(2));
    let text = stderr(&output);
    assert!(text.contains("unsupported"));
    assert!(text.contains("format"));
}

#[test]
fn invalid_environment_values_exit_two_with_a_field_remedy() {
    let sandbox = Sandbox::new("env-invalid");
    let name = "SCRAPPY_LOG_LEVEL";
    let output = sandbox
        .command()
        .env(name, "loud")
        .args(["validate"])
        .output()
        .expect("validate runs");
    assert_eq!(output.status.code(), Some(2), "env: {name}");
    assert!(stderr(&output).contains("log_level"), "env: {name}");

    let output = sandbox
        .command()
        .env("SCRAPPY_RATES", r#"{"":"1"}"#)
        .args(["validate"])
        .output()
        .expect("validate runs");
    assert_eq!(output.status.code(), Some(2));
    assert_ne!(stderr(&output), "");
}

#[test]
fn unparsable_environment_values_are_ignored() {
    let sandbox = Sandbox::new("env-ignored");
    for (name, value) in [
        ("SCRAPPY_MIN_SCORE", "101"),
        ("SCRAPPY_IS_REMOTE", "not-a-bool"),
        ("SCRAPPY_TIMEOUT", "thirty"),
        ("SCRAPPY_RATES", "not-json"),
        ("SCRAPPY_DATE", "2026/01/01"),
    ] {
        let output = sandbox
            .command()
            .env(name, value)
            .args(["validate", "--no-default-config"])
            .output()
            .expect("validate runs");
        assert_validated(&output);
    }
}

#[test]
fn project_config_overrides_user_config() {
    let sandbox = Sandbox::new("files");
    sandbox.write(
        &sandbox.user_config(),
        "format = 'csv'
search = 'rust'
",
    );
    sandbox.write(
        &sandbox.project_config(),
        "format = 'json'
",
    );
    let output = validate_in(&sandbox, &[]);
    assert!(output.status.success(), "stderr: {}", stderr(&output));
    assert_eq!(
        stdout(&output),
        "ok
"
    );

    let shown = sandbox
        .command()
        .args(["config", "show", "--path"])
        .arg(sandbox.project_config())
        .output()
        .expect("show runs");
    assert!(shown.status.success());
    let value: serde_json::Value = serde_json::from_str(&stdout(&shown)).expect("show prints JSON");
    assert_eq!(value["format"], "json");
}

#[test]
fn environment_overrides_project_and_explicit_file_overrides_environment() {
    let sandbox = Sandbox::new("env-precedence");
    sandbox.write(&sandbox.project_config(), invalid_marker());
    let explicit = sandbox.root.join("work").join("extra.toml");
    sandbox.write(&explicit, invalid_marker());

    let output = sandbox
        .command()
        .env("SCRAPPY_LOG_LEVEL", "loud")
        .args(["validate"])
        .output()
        .expect("validate runs");
    assert_eq!(output.status.code(), Some(2));
    assert!(stderr(&output).contains("log_level 'loud'"));

    let fixed = sandbox.root.join("work").join("fixed.toml");
    sandbox.write(&fixed, "log_level = 'info'");
    let output = sandbox
        .command()
        .env("SCRAPPY_LOG_LEVEL", "loud")
        .args(["validate", "--config"])
        .arg(&fixed)
        .output()
        .expect("validate runs");
    assert_validated(&output);
}

#[test]
fn hermetic_validation_ignores_discovered_files_and_environment() {
    let sandbox = Sandbox::new("hermetic");
    sandbox.write(&sandbox.user_config(), "format = 'csv'");
    sandbox.write(&sandbox.project_config(), invalid_marker());
    let explicit = sandbox.root.join("work").join("extra.toml");
    sandbox.write(&explicit, "log_level = 'info'");

    let output = sandbox
        .command()
        .env("SCRAPPY_LOG_LEVEL", "loud")
        .args(["validate", "--no-default-config"])
        .output()
        .expect("validate runs");
    assert_validated(&output);

    let output = sandbox
        .command()
        .env("SCRAPPY_LOG_LEVEL", "loud")
        .args(["validate", "--no-default-config", "--config"])
        .arg(&explicit)
        .output()
        .expect("validate runs");
    assert_validated(&output);
}

#[test]
fn later_explicit_files_override_earlier_ones() {
    let sandbox = Sandbox::new("explicit-order");
    let first = sandbox.root.join("work").join("first.toml");
    let second = sandbox.root.join("work").join("second.toml");
    sandbox.write(&first, invalid_marker());
    sandbox.write(&second, "log_level = 'info'");

    let output = validate_in(
        &sandbox,
        &[
            "--no-default-config",
            "--config",
            first.to_str().expect("utf8 path"),
            "--config",
            second.to_str().expect("utf8 path"),
        ],
    );
    assert_validated(&output);

    let reversed = validate_in(
        &sandbox,
        &[
            "--no-default-config",
            "--config",
            second.to_str().expect("utf8 path"),
            "--config",
            first.to_str().expect("utf8 path"),
        ],
    );
    assert_eq!(reversed.status.code(), Some(2));
    assert!(stderr(&reversed).contains("log_level 'loud'"));
}

#[test]
fn scalar_and_array_forms_are_accepted() {
    let sandbox = Sandbox::new("scalar-array");
    let path = sandbox.root.join("work").join("forms.toml");
    sandbox.write(
        &path,
        "search = 'rust'
sources = ['remoteok', 'remotive']
",
    );
    let output = validate_in(
        &sandbox,
        &[
            "--no-default-config",
            "--config",
            path.to_str().expect("utf8"),
        ],
    );
    assert!(output.status.success(), "stderr: {}", stderr(&output));

    sandbox.write(
        &path,
        "search = []
sources = 'remoteok'
",
    );
    let output = validate_in(
        &sandbox,
        &[
            "--no-default-config",
            "--config",
            path.to_str().expect("utf8"),
        ],
    );
    assert!(output.status.success(), "stderr: {}", stderr(&output));

    let output = sandbox
        .command()
        .env("SCRAPPY_SEARCH", r#"["rust","go"]"#)
        .args(["validate", "--no-default-config"])
        .output()
        .expect("validate runs");
    assert!(output.status.success(), "stderr: {}", stderr(&output));
}

#[test]
fn explicit_false_from_environment_and_file_is_preserved() {
    let sandbox = Sandbox::new("false");
    sandbox.write(&sandbox.project_config(), "is_remote = false");
    let output = validate_in(&sandbox, &[]);
    assert!(output.status.success(), "stderr: {}", stderr(&output));

    sandbox.write(&sandbox.project_config(), "is_remote = true");
    let output = sandbox
        .command()
        .env("SCRAPPY_IS_REMOTE", "false")
        .args(["validate"])
        .output()
        .expect("validate runs");
    assert!(output.status.success(), "stderr: {}", stderr(&output));

    let explicit = sandbox.root.join("work").join("truth.toml");
    sandbox.write(&explicit, "is_remote = true");
    let output = sandbox
        .command()
        .env("SCRAPPY_IS_REMOTE", "false")
        .args(["validate", "--config"])
        .arg(&explicit)
        .output()
        .expect("validate runs");
    assert!(output.status.success(), "stderr: {}", stderr(&output));

    let output = sandbox
        .command()
        .env("SCRAPPY_IS_REMOTE", "not-a-bool")
        .args(["validate", "--no-default-config"])
        .output()
        .expect("validate runs");
    assert!(output.status.success(), "stderr: {}", stderr(&output));
}

#[test]
fn output_format_flags_are_accepted() {
    for format in ["ndjson", "json", "csv", "table"] {
        let output = binary()
            .args(["run", "--source", "nope", "--format", format])
            .output()
            .expect("run runs");
        assert_eq!(output.status.code(), Some(2), "format: {format}");
        assert!(stderr(&output).contains("unknown source: nope"));
    }
    let output = binary()
        .args(["run", "--format", "xml"])
        .output()
        .expect("run runs");
    assert_eq!(output.status.code(), Some(2));
    assert!(stderr(&output).contains("xml"));
}

#[test]
fn list_sources_formats_are_distinct() {
    let json = binary()
        .args(["list-sources", "--format", "json"])
        .output()
        .expect("list runs");
    assert!(json.status.success(), "stderr: {}", stderr(&json));
    let value: serde_json::Value =
        serde_json::from_str(&stdout(&json)).expect("list is valid json");
    assert!(value.is_array());
    assert!(value.as_array().expect("array").len() > 1);

    let table = binary()
        .args(["list-sources", "--format", "table"])
        .output()
        .expect("list runs");
    assert!(table.status.success(), "stderr: {}", stderr(&table));
    assert_ne!(stdout(&table), stdout(&json));
}

#[test]
fn config_init_template_documents_every_field() {
    let sandbox = Sandbox::new("init");
    let path = sandbox.root.join("work").join("config.toml");
    let output = sandbox
        .command()
        .args(["config", "init", "--path"])
        .arg(&path)
        .output()
        .expect("init runs");
    assert!(output.status.success(), "stderr: {}", stderr(&output));
    let text = fs::read_to_string(&path).expect("template written");
    assert!(text.starts_with("# scrappy configuration"));
    for field in CONFIG_SCHEMA {
        assert!(text.contains(&format!("# {} (", field.name)));
        assert!(text.contains(&format!("default: {}", field.default)));
        assert!(text.contains(&format!("SCRAPPY_{}", field.name.to_uppercase())));
    }
    assert!(text.contains("No credentials belong in this file"));
    assert!(!text.contains("password"));
    assert!(!text.contains("api_key"));
}

#[test]
fn config_init_refuses_overwrite_unless_forced() {
    let sandbox = Sandbox::new("init-force");
    let path = sandbox.root.join("work").join("config.toml");
    sandbox.write(&path, "format = 'csv'");

    let refused = sandbox
        .command()
        .args(["config", "init", "--path"])
        .arg(&path)
        .output()
        .expect("init runs");
    assert_eq!(refused.status.code(), Some(2));
    assert_eq!(
        fs::read_to_string(&path).expect("file intact"),
        "format = 'csv'"
    );

    let forced = sandbox
        .command()
        .args(["config", "init", "--force", "--path"])
        .arg(&path)
        .output()
        .expect("init runs");
    assert!(forced.status.success(), "stderr: {}", stderr(&forced));
    assert!(
        fs::read_to_string(&path)
            .expect("template written")
            .starts_with("# scrappy configuration")
    );
}

#[test]
fn config_set_unset_show_and_explain_work() {
    let sandbox = Sandbox::new("config-edit");
    let path = sandbox.root.join("work").join("config.toml");
    sandbox.write(
        &path,
        "format = 'ndjson'
sources = ['remoteok']
[remoteok]
slug = 'seed'
",
    );

    let set = sandbox
        .command()
        .args(["config", "set", "--path"])
        .arg(&path)
        .args(["--key", "timeout", "--value", "45"])
        .output()
        .expect("set runs");
    assert!(set.status.success(), "stderr: {}", stderr(&set));

    let shown = sandbox
        .command()
        .args(["config", "show", "--path"])
        .arg(&path)
        .output()
        .expect("show runs");
    assert!(shown.status.success(), "stderr: {}", stderr(&shown));
    let value: serde_json::Value = serde_json::from_str(&stdout(&shown)).expect("show prints json");
    assert_eq!(value["timeout"], 45);
    assert_eq!(value["format"], "ndjson");
    assert_eq!(value["remoteok"]["slug"], "seed");

    let unset = sandbox
        .command()
        .args(["config", "unset", "--path"])
        .arg(&path)
        .args(["--key", "timeout"])
        .output()
        .expect("unset runs");
    assert!(unset.status.success(), "stderr: {}", stderr(&unset));
    let shown = sandbox
        .command()
        .args(["config", "show", "--path"])
        .arg(&path)
        .output()
        .expect("show runs");
    let value: serde_json::Value = serde_json::from_str(&stdout(&shown)).expect("show prints json");
    assert!(value["timeout"].is_null());
    assert_eq!(value["remoteok"]["slug"], "seed");

    let bad = sandbox
        .command()
        .args(["config", "set", "--path"])
        .arg(&path)
        .args(["--key", "format", "--value", "not-json"])
        .output()
        .expect("set runs");
    assert_eq!(bad.status.code(), Some(2));
    assert!(stderr(&bad).contains("format"));

    let explained = sandbox
        .command()
        .args(["config", "explain", "--key", "timeout"])
        .output()
        .expect("explain runs");
    assert!(explained.status.success());
    assert_eq!(
        stdout(&explained),
        "field=timeout layer=default value=<redacted>
"
    );
    let unknown = sandbox
        .command()
        .args(["config", "explain", "--key", "nope"])
        .output()
        .expect("explain runs");
    assert!(stdout(&unknown).starts_with("field=unknown "));
}

#[test]
fn config_schema_matches_runtime_fields_and_enums() {
    let output = binary()
        .args(["config", "schema"])
        .output()
        .expect("schema runs");
    assert!(output.status.success());
    let value: serde_json::Value = serde_json::from_str(&stdout(&output)).expect("schema is json");
    let entries = value.as_array().expect("schema is array");
    let names: Vec<&str> = entries
        .iter()
        .map(|entry| entry["name"].as_str().expect("name"))
        .collect();
    let expected: Vec<&str> = CONFIG_SCHEMA.iter().map(|field| field.name).collect();
    assert_eq!(names, expected);
    for (entry, field) in entries.iter().zip(CONFIG_SCHEMA) {
        assert_eq!(entry["kind"], field.kind);
        assert_eq!(entry["default"], field.default);
    }

    for (name, values) in [
        ("format", "ndjson, json, csv"),
        ("log_level", "error, warn, info, debug, trace"),
        ("job_type", "fulltime, parttime, contract, internship"),
    ] {
        let output = binary()
            .args(["config", "schema", "--enum", name])
            .output()
            .expect("enum runs");
        assert!(output.status.success(), "field: {name}");
        assert_eq!(stdout(&output).trim_end(), values);
    }
    let output = binary()
        .args(["config", "schema", "--enum", "nope"])
        .output()
        .expect("enum runs");
    assert_eq!(output.status.code(), Some(2));
    assert!(stderr(&output).contains("nope"));
}
