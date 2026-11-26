use std::os::unix::fs::PermissionsExt;
use std::path::PathBuf;
use std::process::{Command, Output};

fn run(args: &[&str]) -> Output {
    Command::new(env!("CARGO_BIN_EXE_scrappy"))
        .args(args)
        .output()
        .expect("binary runs")
}

fn temp_config(name: &str) -> PathBuf {
    std::env::temp_dir().join(format!("scrappy-config-{}-{name}", std::process::id()))
}

#[test]
fn init_writes_template_and_refuses_overwrite() {
    let path = temp_config("init.toml");
    let _ = std::fs::remove_file(&path);
    let args = ["config", "init", "--path"];
    let path_str = path.to_string_lossy();
    let output = run(&[args[0], args[1], args[2], &path_str]);
    assert!(
        output.status.success(),
        "stderr: {}",
        String::from_utf8_lossy(&output.stderr)
    );
    let text = std::fs::read_to_string(&path).expect("template written");
    assert!(text.contains("log_level"));
    assert!(text.contains("SCRAPPY_"));
    let output = run(&[args[0], args[1], args[2], &path_str]);
    assert_eq!(output.status.code(), Some(2));
    assert_eq!(std::fs::read_to_string(&path).expect("original kept"), text);
    std::fs::remove_file(&path).expect("cleanup");
}

#[test]
fn set_and_unset_preserve_sections_and_permissions() {
    let path = temp_config("edit.toml");
    let path_str = path.to_string_lossy();
    std::fs::write(
        &path,
        "timeout = 30\n\n[sources.remoteok]\nenabled = true\n",
    )
    .expect("seed written");
    let output = run(&[
        "config",
        "set",
        "--path",
        &path_str,
        "--key",
        "log_level",
        "--value",
        "\"debug\"",
    ]);
    assert!(
        output.status.success(),
        "stderr: {}",
        String::from_utf8_lossy(&output.stderr)
    );
    let text = std::fs::read_to_string(&path).expect("config readable");
    assert!(text.contains("remoteok"));
    assert!(text.contains("debug"));
    let mode = std::fs::metadata(&path)
        .expect("metadata available")
        .permissions()
        .mode();
    assert_eq!(mode & 0o777, 0o600);
    let output = run(&["config", "unset", "--path", &path_str, "--key", "log_level"]);
    assert!(
        output.status.success(),
        "stderr: {}",
        String::from_utf8_lossy(&output.stderr)
    );
    let text = std::fs::read_to_string(&path).expect("config readable");
    assert!(!text.contains("debug"));
    assert!(text.contains("remoteok"));
    std::fs::remove_file(&path).expect("cleanup");
}

#[test]
fn schema_lists_every_field() {
    let output = run(&["config", "schema"]);
    assert!(output.status.success());
    let text = String::from_utf8(output.stdout).expect("schema is UTF-8");
    for field in [
        "search",
        "sources",
        "format",
        "rates",
        "concurrency",
        "timeout",
        "memory_mb",
        "log_level",
        "is_remote",
    ] {
        assert!(text.contains(field), "schema omitted {field}");
    }
}
