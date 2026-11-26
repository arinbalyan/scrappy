//! Offline parity checks for the bundled source registry.
//!
//! Every built-in manifest must have a behavior document, a declared health
//! state, a family directory that matches its declared family, and a complete
//! fixture set. Manifests must declare credentials by variable name only, so
//! no manifest carries a credential value or secret.

// Rust guideline compliant 2026-10-09

use std::fs;
use std::path::{Path, PathBuf};

use scrappy::sources::manifest::{Family, Health, Manifest};
use scrappy::sources::registry::{self, Registry};

const FIXTURE_FILES: [&str; 6] = [
    "request.json",
    "request.template.json",
    "response.body",
    "headers.json",
    "frozen.json",
    "golden.ndjson",
];

fn root() -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR"))
}

fn manifest_root() -> PathBuf {
    root().join("src/sources/manifests")
}

fn bundled() -> (Vec<PathBuf>, Registry) {
    let files = registry::manifest_files(&manifest_root()).expect("manifest files are readable");
    let registry = registry::load(&manifest_root()).expect("bundled registry loads");
    assert!(!files.is_empty(), "no built-in manifests found");
    assert_eq!(files.len(), registry.len(), "manifest ids are not unique");
    (files, registry)
}

fn family_dir(family: Family) -> &'static str {
    match family {
        Family::FeedsApi => "feeds_api",
        Family::Ats => "ats",
        Family::HtmlStatic => "html_static",
        Family::HtmlRendered => "html_rendered",
        Family::Keyed => "keyed",
    }
}

fn health_label(health: Health) -> &'static str {
    match health {
        Health::Working => "working",
        Health::Keyed => "keyed",
        Health::Blocked => "blocked",
        Health::Stale => "stale",
        Health::RequiresSlug => "requires_slug",
        Health::BrowserOnly => "browser_only",
    }
}

fn assert_known_health(manifest: &Manifest) {
    let label = health_label(manifest.health);
    assert!(
        ALL_HEALTH.contains(&manifest.health),
        "manifest {} has undeclared health {label}",
        manifest.id
    );
}

fn assert_manifest_path(manifest: &Manifest, path: &Path) {
    let expected = manifest_root()
        .join(family_dir(manifest.family))
        .join(format!("{}.toml", manifest.id));
    assert_eq!(
        path,
        expected,
        "manifest {} declares family {:?} but lives at {}",
        manifest.id,
        manifest.family,
        path.display()
    );
}

const ALL_HEALTH: [Health; 6] = [
    Health::Working,
    Health::Keyed,
    Health::Blocked,
    Health::Stale,
    Health::RequiresSlug,
    Health::BrowserOnly,
];

fn assert_behavior_doc(manifest: &Manifest) {
    let path = root()
        .join("documentation/scrapers")
        .join(format!("{}.md", manifest.id));
    assert!(
        path.is_file(),
        "missing behavior document {}",
        path.display()
    );
}

fn assert_fixtures(manifest: &Manifest) {
    let dir = root().join("tests/fixtures").join(&manifest.id);
    assert!(
        dir.is_dir(),
        "missing registered fixture set {}",
        dir.display()
    );
    for name in FIXTURE_FILES {
        let path = dir.join(name);
        assert!(
            path.is_file(),
            "missing canonical fixture {} for {}",
            path.display(),
            manifest.id
        );
    }
}

fn assert_no_secret(path: &Path, text: &str) {
    for line in text.lines() {
        let (key, value) = line.split_once('=').unwrap_or(("", ""));
        let key = key.trim().to_ascii_lowercase();
        if !matches!(key.as_str(), "credentials" | "request" | "body") && !line.contains("{key:") {
            let lower = line.to_ascii_lowercase();
            for marker in ["secret", "password", "passwd", "apikey", "api_key", "token"] {
                assert!(
                    !lower.contains(marker),
                    "manifest {} contains a credential value or secret ({marker})",
                    path.display()
                );
            }
        }
        if key != "credentials" {
            continue;
        }
        let value = value.trim();
        let is_value = !value.is_empty() && !value.starts_with('[');
        assert!(
            !is_value,
            "manifest {} declares a literal credential value",
            path.display()
        );
    }
}

#[test]
fn manifests_match_files_documents_and_health_states() {
    let (files, registry) = bundled();
    for path in files {
        let manifest = registry
            .get(
                path.file_stem()
                    .and_then(|stem| stem.to_str())
                    .expect("manifest file has a name"),
            )
            .expect("registry contains the manifest file");
        assert_manifest_path(manifest, &path);
        assert_known_health(manifest);
        assert_behavior_doc(manifest);
    }
}

#[test]
fn registered_fixture_sets_have_the_six_canonical_files() {
    let (_, registry) = bundled();
    for manifest in registry.iter() {
        assert_fixtures(manifest);
    }
}

#[test]
fn manifests_declare_no_credential_value_or_secret() {
    let (files, _) = bundled();
    for path in files {
        let text = fs::read_to_string(&path).expect("manifest is readable");
        assert_no_secret(&path, &text);
    }
}
