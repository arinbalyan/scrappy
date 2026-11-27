// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Arin Balyan
// Last modified: 2026-11-27

//! Machine-readable attention events for challenged sources.
//!
//! See documentation/015-security.md.

// Rust guideline compliant 2026-10-09

use serde::Serialize;

use crate::config::Config;
use crate::engine::challenge::{Challenge, ChallengeKind};

/// A challenge event that asks a human or downstream consumer to intervene.
#[derive(Debug, Clone, PartialEq, Eq, Serialize)]
#[serde(deny_unknown_fields)]
pub struct AttentionEvent {
    /// Discriminator for the event kind.
    pub event: &'static str,
    /// Identifier of the challenged source.
    pub source_id: String,
    /// Classification of the challenge.
    pub kind: ChallengeKind,
    /// HTTP status that carried the challenge.
    pub status: u16,
    /// Sanitized evidence safe to display.
    pub evidence: String,
    /// Opaque resume token bound to one configuration fingerprint.
    pub resume_state: String,
}

/// Build an attention event from a challenge using the active configuration.
#[must_use]
pub fn attention_event(source_id: &str, challenge: &Challenge, config: &Config) -> AttentionEvent {
    AttentionEvent {
        event: "source.attention_required",
        source_id: source_id.to_owned(),
        kind: challenge.kind,
        status: challenge.status,
        evidence: challenge.evidence.clone(),
        resume_state: crate::checkpoint::fingerprint(config),
    }
}

/// Return true when a resume token belongs to the given configuration.
#[must_use]
pub fn resume_matches(event: &AttentionEvent, config: &Config) -> bool {
    event.resume_state == crate::checkpoint::fingerprint(config)
}

/// Return true when the event only pauses its own source.
#[must_use]
pub fn only_source_paused(event: &AttentionEvent, source_id: &str) -> bool {
    event.source_id == source_id
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::collections::BTreeMap;

    fn config(format: &str) -> Config {
        let json = format!(r#"{{"format":"{format}"}}"#);
        serde_json::from_str(&json).expect("valid config")
    }

    fn challenge() -> Challenge {
        let headers = BTreeMap::new();
        crate::engine::challenge::detect(
            403,
            &headers,
            "cf-browser-verification?token=abcdefghijklmnopqrstuvwxyz1234",
        )
        .expect("challenge detected")
    }

    #[test]
    fn event_is_machine_readable() {
        let event = attention_event("greenhouse", &challenge(), &config("ndjson"));
        let json = serde_json::to_string(&event).expect("serialized event");
        let value: serde_json::Value = serde_json::from_str(&json).expect("valid json");
        assert_eq!(value["event"], "source.attention_required");
        assert_eq!(value["source_id"], "greenhouse");
        assert_eq!(value["kind"], "Cloudflare");
        assert!(value["resume_state"].is_string());
        assert!(!json.contains("abcdefghijklmnopqrstuvwxyz1234"));
    }

    #[test]
    fn resume_state_rejects_other_config() {
        let event = attention_event("greenhouse", &challenge(), &config("ndjson"));
        assert!(resume_matches(&event, &config("ndjson")));
        assert!(!resume_matches(&event, &config("csv")));
    }

    #[test]
    fn challenge_pauses_only_its_source() {
        let event = attention_event("greenhouse", &challenge(), &config("ndjson"));
        assert!(only_source_paused(&event, "greenhouse"));
        assert!(!only_source_paused(&event, "lever"));
    }
}
