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

#[test]
fn canonical_tree_is_present() {
    let expected = [
        "README.md",
        "LICENSE",
        "NOTICE",
        "src/main.rs",
        "src/lib.rs",
        "src/cli/mod.rs",
        "src/cli/doctor.rs",
        "src/config/mod.rs",
        "src/config/file.rs",
        "src/sources/mod.rs",
        "src/sources/manifest.rs",
        "src/sources/registry.rs",
        "src/sources/manifests/mod.rs",
        "src/sources/manifests/feeds_api/README.md",
        "src/sources/manifests/ats/README.md",
        "src/sources/manifests/html_static/README.md",
        "src/sources/manifests/html_rendered/README.md",
        "src/sources/manifests/keyed/README.md",
        "src/fetching/mod.rs",
        "src/fetching/plain.rs",
        "src/fetching/emulation.rs",
        "src/fetching/browser.rs",
        "src/fetching/proxy.rs",
        "src/parsing/mod.rs",
        "src/parsing/json.rs",
        "src/parsing/rss.rs",
        "src/parsing/html.rs",
        "src/parsing/jsonld.rs",
        "src/engine/mod.rs",
        "src/engine/contract.rs",
        "src/output/mod.rs",
        "src/security/mod.rs",
        "src/checkpoint/mod.rs",
        "src/metrics/mod.rs",
        "tests/cli_contract/mod.rs",
        "tests/cli_contract/doctor.rs",
        "tests/bridge_contract/mod.rs",
        "tests/config_actions.rs",
        "tests/docs_parity.rs",
        "tests/fixtures/README.md",
        "tests/live/README.md",
        "documentation/scrapers/README.md",
        ".github/workflows/ci.yml",
        "openspec/changes/oculus-foundation/design.md",
    ];
    let missing: Vec<&str> = expected
        .into_iter()
        .filter(|path| !repo_root().join(path).exists())
        .collect();
    assert!(missing.is_empty(), "missing canonical paths: {missing:?}");
}

#[test]
fn registered_sources_have_fixtures_and_docs() {
    let root = repo_root();
    let manifests = root.join("src/sources/manifests");
    let mut found = 0;
    for family in ["feeds_api", "ats", "html_static", "html_rendered", "keyed"] {
        let dir = manifests.join(family);
        let Ok(entries) = std::fs::read_dir(&dir) else {
            continue;
        };
        for entry in entries.flatten() {
            let path = entry.path();
            if path.extension().is_some_and(|ext| ext == "toml") {
                let text = std::fs::read_to_string(&path).expect("manifest readable");
                let id = text
                    .lines()
                    .find_map(|line| line.strip_prefix("id = "))
                    .map(|line| line.trim().trim_matches('"'))
                    .expect("manifest has an id");
                found += 1;
                assert!(
                    root.join("tests/fixtures").join(id).is_dir(),
                    "missing fixture set for {id}"
                );
                assert!(
                    root.join("documentation/scrapers")
                        .join(format!("{id}.md"))
                        .is_file(),
                    "missing behavior doc for {id}"
                );
            }
        }
    }
    let _ = found;
}

#[test]
fn generated_status_matches_registry() {
    let root = repo_root();
    let manifest_root = root.join("src/sources/manifests");
    let mut registry = scrappy::sources::registry::Registry::new();
    let mut files = Vec::new();
    for family in ["feeds_api", "ats", "html_static", "html_rendered", "keyed"] {
        for entry in fs::read_dir(manifest_root.join(family))
            .expect("family folder exists")
            .flatten()
        {
            if entry.path().extension().is_some_and(|ext| ext == "toml") {
                files.push(entry.path());
            }
        }
    }
    files.sort();
    for path in files {
        let text = fs::read_to_string(path).expect("manifest readable");
        let manifest = scrappy::sources::manifest::parse(&text).expect("manifest parses");
        registry.insert(manifest);
    }
    let expected = scrappy::sources::status_doc::render(&registry, &root.join("tests/fixtures"));
    let actual =
        fs::read_to_string(root.join("documentation/005-status.md")).expect("status doc exists");
    assert_eq!(
        actual, expected,
        "status doc is stale; run list-sources --generate-status"
    );
}
