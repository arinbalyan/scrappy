// Rust guideline compliant 2026-10-09

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

const NUMBERED_DOCS: [&str; 16] = [
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
    "016-config-migration.md",
];

const MANIFEST_FAMILIES: [&str; 5] = ["feeds_api", "ats", "html_static", "html_rendered", "keyed"];

const FIXTURE_FILES: [&str; 6] = [
    "request.json",
    "request.template.json",
    "response.body",
    "headers.json",
    "frozen.json",
    "golden.ndjson",
];

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
    let docs = documented_docs();
    for name in NUMBERED_DOCS {
        assert!(
            docs.iter()
                .any(|path| path.file_name().is_some_and(|file| file == name)),
            "missing documentation/{name}"
        );
    }
}

#[test]
fn canonical_tree_is_present() {
    let mut expected: Vec<String> = vec![
        "Cargo.toml".to_owned(),
        "Cargo.lock".to_owned(),
        "README.md".to_owned(),
        "LICENSE".to_owned(),
        "NOTICE".to_owned(),
        ".gitignore".to_owned(),
        "openspec/".to_owned(),
        ".codex/skills/ms-rust/SKILL.md".to_owned(),
        ".github/workflows/ci.yml".to_owned(),
        "src/main.rs".to_owned(),
        "src/lib.rs".to_owned(),
        "src/cli/mod.rs".to_owned(),
        "src/cli/doctor.rs".to_owned(),
        "src/config/mod.rs".to_owned(),
        "src/config/file.rs".to_owned(),
        "src/sources/mod.rs".to_owned(),
        "src/sources/manifest.rs".to_owned(),
        "src/sources/registry.rs".to_owned(),
        "src/sources/manifests/mod.rs".to_owned(),
        "src/fetching/mod.rs".to_owned(),
        "src/fetching/plain.rs".to_owned(),
        "src/fetching/emulation.rs".to_owned(),
        "src/fetching/browser.rs".to_owned(),
        "src/fetching/proxy.rs".to_owned(),
        "src/parsing/mod.rs".to_owned(),
        "src/parsing/json.rs".to_owned(),
        "src/parsing/rss.rs".to_owned(),
        "src/parsing/html.rs".to_owned(),
        "src/parsing/jsonld.rs".to_owned(),
        "src/engine/mod.rs".to_owned(),
        "src/engine/contract.rs".to_owned(),
        "src/output/mod.rs".to_owned(),
        "src/security/mod.rs".to_owned(),
        "src/checkpoint/mod.rs".to_owned(),
        "src/metrics/mod.rs".to_owned(),
        "tests/cli_contract/mod.rs".to_owned(),
        "tests/cli_contract/doctor.rs".to_owned(),
        "tests/bridge_contract/mod.rs".to_owned(),
        "tests/config_actions.rs".to_owned(),
        "tests/docs_parity.rs".to_owned(),
        "tests/fixtures/README.md".to_owned(),
        "tests/live/".to_owned(),
        "documentation/scrapers/".to_owned(),
        "openspec/changes/oculus-foundation/design.md".to_owned(),
    ];
    expected.extend(
        NUMBERED_DOCS
            .iter()
            .map(|name| format!("documentation/{name}")),
    );
    expected.extend(
        MANIFEST_FAMILIES
            .iter()
            .map(|family| format!("src/sources/manifests/{family}/")),
    );
    expected.extend(
        MANIFEST_FAMILIES
            .iter()
            .map(|family| format!("src/sources/manifests/{family}/README.md")),
    );
    for source in registered_source_ids() {
        for file in FIXTURE_FILES {
            expected.push(format!("tests/fixtures/{source}/{file}"));
        }
        expected.push(format!("documentation/scrapers/{source}.md"));
    }
    for entry in fs::read_dir(repo_root().join("tests/fixtures"))
        .expect("fixtures folder exists")
        .flatten()
    {
        let path = entry.path();
        if !path.is_dir() {
            continue;
        }
        let Some(name) = path.file_name().and_then(|name| name.to_str()) else {
            continue;
        };
        expected.push(format!("tests/fixtures/{name}/request.template.json"));
        expected.push(format!("tests/fixtures/{name}/frozen.json"));
    }
    let missing: Vec<String> = expected
        .into_iter()
        .filter(|path| !repo_root().join(path).exists())
        .collect();
    assert!(missing.is_empty(), "missing canonical paths: {missing:?}");
}

fn registered_source_ids() -> Vec<String> {
    let root = repo_root();
    let mut ids = Vec::new();
    for family in MANIFEST_FAMILIES {
        let dir = root.join("src/sources/manifests").join(family);
        for entry in fs::read_dir(&dir)
            .expect("manifest family exists")
            .flatten()
        {
            let path = entry.path();
            if path.extension().is_some_and(|ext| ext == "toml") {
                let text = fs::read_to_string(&path).expect("manifest readable");
                let id = text
                    .lines()
                    .find_map(|line| line.strip_prefix("id = "))
                    .map(|line| line.trim().trim_matches('"'))
                    .expect("manifest has an id");
                ids.push(id.to_owned());
            }
        }
    }
    ids.sort();
    ids
}

#[test]
fn manifest_families_are_present() {
    let manifests = repo_root().join("src/sources/manifests");
    for family in MANIFEST_FAMILIES {
        assert!(
            manifests.join(family).is_dir(),
            "missing manifest family directory: src/sources/manifests/{family}"
        );
    }
}

#[test]
fn every_fixture_set_has_the_canonical_files() {
    let root = repo_root();
    let mut sets = 0;
    for source in registered_source_ids() {
        let dir = root.join("tests/fixtures").join(&source);
        for file in FIXTURE_FILES {
            assert!(
                dir.join(file).is_file(),
                "missing tests/fixtures/{source}/{file}"
            );
        }
        sets += 1;
    }
    assert!(sets > 0, "no registered source fixtures found");
}

#[test]
fn registered_sources_have_fixtures_and_docs() {
    let root = repo_root();
    let manifests = root.join("src/sources/manifests");
    for family in MANIFEST_FAMILIES {
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
}

#[test]
fn generated_status_matches_registry() {
    let root = repo_root();
    let manifest_root = root.join("src/sources/manifests");
    let mut registry = scrappy::sources::registry::Registry::new();
    let mut files = Vec::new();
    for family in MANIFEST_FAMILIES {
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
