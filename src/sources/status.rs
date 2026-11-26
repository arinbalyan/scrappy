//! Source health status derived from manifests.
//!
//! Health states are specified in
//! openspec/changes/oculus-foundation/specs/source-registry/spec.md.

// Rust guideline compliant 2026-10-09

use serde::Serialize;

use crate::sources::manifest::{Health, Manifest};
use crate::sources::registry::Registry;

/// Availability of a source before a run starts.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum Availability {
    /// Source can be fetched.
    Ready,
    /// Source is missing a declared credential.
    MissingCredentials,
    /// Source is missing a required slug seed.
    MissingSlug,
    /// Source is blocked by policy or a challenge.
    Blocked,
}

/// One row in the source listing.
#[derive(Debug, Clone, PartialEq, Eq, Serialize)]
pub struct SourceStatus {
    /// Stable source identifier.
    pub id: String,
    /// Human-readable source name.
    pub name: String,
    /// Declared health state.
    pub health: Health,
    /// Current availability before a run.
    pub availability: Availability,
    /// Adapter family.
    pub family: String,
    /// Fetcher kind.
    pub fetcher: String,
    /// Emulation profile name.
    pub profile: String,
    /// Credential names, without values.
    pub credentials: Vec<String>,
    /// Fixture identifiers.
    pub fixtures: Vec<String>,
    /// Robots policy decision.
    pub robots: String,
    /// Attribution text.
    pub attribution: String,
}

/// Build statuses for every registered manifest.
#[must_use]
pub fn list(registry: &Registry, fixture_root: &std::path::Path) -> Vec<SourceStatus> {
    registry
        .iter()
        .map(|manifest| status(manifest, fixture_root))
        .collect()
}

fn status(manifest: &Manifest, fixture_root: &std::path::Path) -> SourceStatus {
    let missing_credential = manifest
        .credentials
        .iter()
        .any(|name| std::env::var(name).is_err());
    let missing_fixture = manifest
        .fixtures
        .iter()
        .any(|id| !fixture_root.join(id).is_dir());
    let availability = if manifest.health == Health::Blocked {
        Availability::Blocked
    } else if missing_credential {
        Availability::MissingCredentials
    } else if manifest.health == Health::RequiresSlug && manifest.slugs.is_empty() {
        Availability::MissingSlug
    } else if missing_fixture {
        Availability::Blocked
    } else {
        Availability::Ready
    };
    SourceStatus {
        id: manifest.id.clone(),
        name: manifest.name.clone(),
        health: manifest.health,
        availability,
        family: format!("{:?}", manifest.family),
        fetcher: manifest.fetcher.clone(),
        profile: manifest.profile.clone(),
        credentials: manifest.credentials.clone(),
        fixtures: manifest.fixtures.clone(),
        robots: manifest.robots.clone(),
        attribution: manifest.attribution.clone(),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn fixture_gap_blocks_source() {
        let manifest: Manifest =
            toml::from_str(include_str!("manifest_sample.toml")).expect("sample manifest parses");
        let registry = Registry::new();
        assert_eq!(registry.len(), 0);
        let status = status(&manifest, std::path::Path::new("/nonexistent"));
        assert_eq!(status.availability, Availability::Blocked);
    }
}
