// Rust guideline compliant 2026-10-09

use std::collections::BTreeSet;
use std::fs;
use std::io::Write;
use std::path::PathBuf;
use std::process::{Command, Output, Stdio};
use std::time::{SystemTime, UNIX_EPOCH};

use scrappy::checkpoint::{self, Checkpoint};
use scrappy::config::Config;
use scrappy::engine::contract::{
    EXIT_CANCELLED, EXIT_CHALLENGE, EXIT_INTERNAL, EXIT_OK, EXIT_PARTIAL, EXIT_USAGE,
    ProgressEvent, write_event,
};
use scrappy::output::record::{
    Email, EmailSource, JobInput, Provenance, SCHEMA_VERSION, from_record,
};
use scrappy::output::writer::{self, Format};
use scrappy::parsing::Record;

struct CountingWriter {
    bytes: Vec<u8>,
    flushes: usize,
}

impl Write for CountingWriter {
    fn write(&mut self, buf: &[u8]) -> std::io::Result<usize> {
        self.bytes.extend_from_slice(buf);
        Ok(buf.len())
    }

    fn flush(&mut self) -> std::io::Result<()> {
        self.flushes += 1;
        Ok(())
    }
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
        let root = std::env::temp_dir().join(format!("scrappy-bridge-{label}-{nanos}"));
        fs::create_dir_all(&root).expect("temp dir created");
        Self { root }
    }
}

impl Drop for Sandbox {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.root);
    }
}

fn binary() -> Command {
    Command::new(env!("CARGO_BIN_EXE_scrappy"))
}

fn run_with_stdin(args: &[&str], input: &str) -> Output {
    let mut child = binary()
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

fn stderr(output: &Output) -> String {
    String::from_utf8(output.stderr.clone()).expect("stderr is UTF-8")
}

fn event(name: &str, source: Option<&str>, emitted: usize, skipped: usize) -> ProgressEvent {
    ProgressEvent {
        event: name.to_owned(),
        run_id: "run-1".to_owned(),
        source_id: source.map(str::to_owned),
        emitted,
        skipped,
        status: "success".to_owned(),
    }
}

fn job(email: &str, company_url: &str) -> scrappy::output::record::Job {
    let record = Record::new(
        "https://jobs.test/1?utm_source=board#top",
        "Engineer",
        "Example Co",
    )
    .expect("valid record")
    .with_description(Some("Build things".to_owned()));
    let input = JobInput {
        source_id: "fixture".to_owned(),
        fetched_at: "2026-11-27T00:00:00Z".to_owned(),
        company_url: Some(company_url.to_owned()),
        emails: vec![Email {
            addr: email.to_owned(),
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
fn schema_prints_enum_values() {
    for (field, values) in [
        ("format", "ndjson, json, csv"),
        ("log_level", "error, warn, info, debug, trace"),
        ("job_type", "fulltime, parttime, contract, internship"),
    ] {
        let output = binary()
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
    let output = binary()
        .args(["config", "schema", "--enum", "nope"])
        .output()
        .expect("binary runs");
    assert_eq!(output.status.code(), Some(2));
}

#[test]
fn valid_stdin_config_produces_no_stdout_and_no_fetch() {
    let output = run_with_stdin(&["run", "--stdin-config"], r#"{"format":"csv"}"#);
    assert!(output.status.success(), "stderr: {}", stderr(&output));
    assert_eq!(output.stdout, [] as [u8; 0]);
    assert!(stderr(&output).contains("source selection is unavailable"));
}

#[test]
fn full_stdin_config_is_accepted() {
    let input = r#"{
        "search": ["rust", "go"],
        "sources": "remoteok",
        "format": "ndjson",
        "rates": {"remoteok": 5},
        "concurrency": 2,
        "timeout": 30,
        "memory_mb": 512,
        "log_level": "debug",
        "is_remote": false,
        "job_type": "fulltime",
        "min_score": 20,
        "hours_old": 48,
        "date": "2026-11-27"
    }"#;
    let output = run_with_stdin(&["run", "--stdin-config"], input);
    assert!(output.status.success(), "stderr: {}", stderr(&output));
    assert_eq!(output.stdout, [] as [u8; 0]);
}

#[test]
fn malformed_stdin_config_exits_two_before_fetch() {
    for input in [
        r#"{"unknown":1}"#,
        r#"{"format":"xml"}"#,
        r#"{"min_score":101}"#,
        r#"{"concurrency":0}"#,
        r#"{"date":"2026/11/27"}"#,
        r#"{"rates":{"":1}}"#,
        "not json",
        "[]",
        r#""text""#,
        r#"{"timeout":"thirty"}"#,
    ] {
        let output = run_with_stdin(&["run", "--stdin-config"], input);
        assert_eq!(output.status.code(), Some(2), "input: {input}");
        assert!(output.stdout.is_empty(), "input emitted stdout: {input}");
        assert!(!stderr(&output).is_empty(), "input: {input}");
    }
}

#[test]
fn invalid_enum_in_stdin_config_lists_values() {
    let output = run_with_stdin(&["run", "--stdin-config"], r#"{"job_type":"unknown"}"#);
    assert_eq!(output.status.code(), Some(2));
    let text = stderr(&output);
    assert!(text.contains("fulltime"));
    assert!(text.contains("job_type"));
}

#[test]
fn unknown_stdin_key_is_named_on_stderr() {
    let output = run_with_stdin(&["run", "--stdin-config"], r#"{"sneaky":1}"#);
    assert_eq!(output.status.code(), Some(2));
    assert!(stderr(&output).contains("sneaky"));
}

#[test]
fn stdin_config_ignores_explicit_config_flags() {
    let sandbox = Sandbox::new("stdin-flags");
    let bad = sandbox.root.join("broken.toml");
    fs::write(&bad, "format = ").expect("config written");
    let output = run_with_stdin(
        &[
            "run",
            "--stdin-config",
            "--config",
            bad.to_str().expect("utf8 path"),
            "--no-default-config",
        ],
        r#"{"format":"csv"}"#,
    );
    assert!(output.status.success(), "stderr: {}", stderr(&output));
    assert_eq!(output.stdout, [] as [u8; 0]);
}

#[test]
fn progress_events_serialize_as_ndjson_with_optional_source() {
    let events = [
        event("run.started", None, 0, 0),
        event("source.started", Some("remoteok"), 0, 0),
        event("run.completed", None, 2, 1),
    ];
    let mut bytes = Vec::new();
    for current in &events {
        write_event(&mut bytes, current).expect("event written");
    }
    let text = String::from_utf8(bytes).expect("utf8");
    let lines: Vec<&str> = text.lines().collect();
    assert_eq!(lines.len(), 3);
    for (line, expected) in lines.iter().zip(&events) {
        let value: serde_json::Value = serde_json::from_str(line).expect("one complete JSON event");
        assert_eq!(value["event"], expected.event);
        assert_eq!(value["run_id"], "run-1");
        assert_eq!(value["emitted"], expected.emitted);
        assert_eq!(value["skipped"], expected.skipped);
        assert_eq!(value["status"], "success");
        match expected.source_id.as_deref() {
            Some(source) => assert_eq!(value["source_id"], source),
            None => assert!(value["source_id"].is_null()),
        }
    }
}

#[test]
fn progress_writer_flushes_each_complete_line() {
    let mut writer = CountingWriter {
        bytes: Vec::new(),
        flushes: 0,
    };
    write_event(&mut writer, &event("run.started", None, 0, 0)).expect("first event");
    assert_eq!(writer.flushes, 1);
    write_event(&mut writer, &event("run.completed", None, 1, 0)).expect("second event");
    assert_eq!(writer.flushes, 2);
    let text = String::from_utf8(writer.bytes).expect("utf8");
    assert_eq!(text.lines().count(), 2);
    for line in text.lines() {
        let _: serde_json::Value = serde_json::from_str(line).expect("complete JSON line");
    }
}

#[test]
fn consumers_separate_structured_progress_from_human_diagnostics() {
    let mut bytes = Vec::new();
    write_event(&mut bytes, &event("run.completed", None, 1, 0)).expect("event written");
    let diagnostics = "scrappy: source selection is unavailable in this build
";
    let mut stream = String::from_utf8(bytes).expect("utf8");
    stream.push_str(diagnostics);
    let mut events = 0;
    let mut human = 0;
    for line in stream.lines() {
        if serde_json::from_str::<serde_json::Value>(line).is_ok() {
            events += 1;
        } else {
            human += 1;
        }
    }
    assert_eq!(events, 1);
    assert_eq!(human, 1);
}

#[test]
fn published_exit_codes_are_distinct_and_usage_is_honored() {
    let codes = [
        EXIT_OK,
        EXIT_INTERNAL,
        EXIT_USAGE,
        EXIT_PARTIAL,
        EXIT_CHALLENGE,
        EXIT_CANCELLED,
    ];
    let mut unique = BTreeSet::new();
    for code in codes {
        assert!(unique.insert(code), "duplicate exit code {code}");
    }
    assert_eq!(codes, [0, 1, 2, 3, 4, 5]);

    let output = run_with_stdin(&["run", "--stdin-config"], r#"{"format":"xml"}"#);
    assert_eq!(output.status.code(), Some(EXIT_USAGE));
}

#[test]
fn format_encoders_share_the_same_record_fields() {
    let record = job("jobs@example.test", "https://example.test");
    let ndjson =
        writer::encode(std::slice::from_ref(&record), Format::Ndjson).expect("ndjson encoded");
    let json = writer::encode(std::slice::from_ref(&record), Format::Json).expect("json encoded");
    let csv = writer::encode(std::slice::from_ref(&record), Format::Csv).expect("csv encoded");

    let line: serde_json::Value =
        serde_json::from_str(ndjson.trim_end()).expect("ndjson line is json");
    let document: serde_json::Value = serde_json::from_str(&json).expect("json document");
    assert_eq!(line, document[0]);
    assert!(
        csv.lines()
            .next()
            .expect("header")
            .contains("company_domain")
    );
    assert!(csv.lines().count() >= 2);

    let streamed: serde_json::Value =
        serde_json::from_str(ndjson.trim_end()).expect("same serialization path");
    assert_eq!(streamed["company_domain"], "example.test");
}

#[test]
fn adversarial_email_does_not_set_company_domain_or_inflate_score() {
    let unrelated = job("worker@unrelated.test", "https://example.test/careers");
    let matching = job("worker@example.test", "https://example.test/careers");
    assert_eq!(unrelated.company_domain.as_deref(), Some("example.test"));
    assert_eq!(
        matching.quality_score,
        unrelated.quality_score + 20,
        "only the matching domain adds score"
    );
}

#[test]
fn checkpoint_preserves_cancellation_records() {
    let sandbox = Sandbox::new("checkpoint");
    let config: Config = serde_json::from_str(r#"{"format":"ndjson","sources":["remoteok"]}"#)
        .expect("config parsed");
    let fingerprint = checkpoint::fingerprint(&config);
    let path = sandbox.root.join("checkpoint.json");

    let mut saved = Checkpoint::new(&fingerprint);
    saved.complete("remoteok", 42);
    checkpoint::save(&path, &saved).expect("checkpoint saved");

    let loaded = checkpoint::load(&path, &config).expect("checkpoint loaded");
    assert_eq!(loaded.version, checkpoint::CHECKPOINT_VERSION);
    assert!(loaded.is_source_complete("remoteok"));
    assert!(loaded.was_emitted(42));
    assert!(!loaded.is_source_complete("remotive"));
    assert!(!path.with_extension("json.tmp").exists());
}
