use std::fs;
use std::path::PathBuf;

fn repo_root() -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR"))
}

fn documented_docs() -> Vec<PathBuf> {
    let mut docs: Vec<PathBuf> = fs::read_dir(repo_root().join("documentation"))
        .expect("documentation folder exists")
        .filter_map(|entry| entry.ok().map(|entry| entry.path()))
        .filter(|path| path.extension().is_some_and(|ext| ext == "md"))
        .collect();
    docs.sort();
    docs
}

fn backtick_paths(text: &str) -> Vec<String> {
    text.split('`')
        .skip(1)
        .step_by(2)
        .filter(|part| part.contains('/') && !part.contains(' ') && !part.starts_with('<'))
        .map(str::to_owned)
        .collect()
}

#[test]
fn documentation_references_existing_paths() {
    let missing: Vec<String> = documented_docs()
        .iter()
        .flat_map(|doc| {
            let text = fs::read_to_string(doc).expect("documentation is readable");
            backtick_paths(&text)
                .into_iter()
                .filter(|path| {
                    let candidate = path.trim_end_matches('/');
                    !repo_root().join(candidate).exists()
                        && !candidate.contains("<source_id>")
                        && !candidate.starts_with("https://")
                })
                .map(|path| format!("{:?}: {path}", doc.file_name().unwrap()))
        })
        .collect();

    assert!(missing.is_empty(), "missing documented paths: {missing:?}");
}

#[test]
fn every_expected_document_exists() {
    let expected = [
        "001-quickstart.md",
        "002-installation.md",
        "003-cli-reference.md",
        "004-configuration.md",
        "005-status.md",
        "006-architecture.md",
        "007-email-extraction.md",
        "008-quality-scoring.md",
        "009-export-formats.md",
        "010-proxy.md",
        "011-dedup.md",
        "012-browser-automation.md",
        "013-source-registry.md",
        "014-testing.md",
        "015-security.md",
    ];
    let docs = documented_docs();
    for name in expected {
        assert!(
            docs.iter()
                .any(|path| path.file_name().is_some_and(|file| file == name)),
            "missing documentation/{name}"
        );
    }
}
