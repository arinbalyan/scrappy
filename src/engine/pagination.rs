// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Arin Balyan
// Last modified: 2026-11-27

//! Manifest pagination strategies.
//!
//! Supported forms are `page:N`, `page:start:end`, `offset:step`, and
//! `offset:step:pages`. Other declared strategies stay single-request until
//! an executor exists for them. See documentation/006-architecture.md.

// Rust guideline compliant 2026-10-09

/// Largest page count or offset page count accepted from a manifest.
pub const MAX_PAGES: u32 = 1000;

/// Parsed pagination strategy for one manifest.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Strategy {
    /// One request only.
    None,
    /// Page numbers from `start` through `end`.
    Page {
        /// First page number.
        start: u32,
        /// Last page number.
        end: u32,
    },
    /// Offsets starting at zero with a fixed step.
    Offset {
        /// Offset increment between requests.
        step: u32,
        /// Number of offset requests.
        pages: u32,
    },
    /// Cursor token supplied by the response.
    Cursor,
    /// Bounded fanout over an id array.
    Fanout {
        /// Size of each fanout batch.
        batch: u32,
        /// Maximum ids requested.
        max_ids: u32,
    },
}

impl Strategy {
    /// Query placeholder that selects the next page or offset.
    #[must_use]
    pub fn query_key(self) -> Option<&'static str> {
        match self {
            Self::None | Self::Fanout { .. } => None,
            Self::Page { .. } => Some("page"),
            Self::Offset { .. } => Some("offset"),
            Self::Cursor => Some("cursor"),
        }
    }

    /// Values assigned to the query placeholder in request order.
    #[must_use]
    pub fn values(self) -> Vec<u32> {
        match self {
            Self::Page { start, end } => (start..=end).collect(),
            Self::Offset { step, pages } => (0..pages).map(|index| index * step).collect(),
            Self::None | Self::Cursor | Self::Fanout { .. } => vec![0],
        }
    }
}

/// Parse a manifest pagination string.
#[must_use]
pub fn parse(text: &str) -> Strategy {
    let text = text.trim().to_ascii_lowercase();
    if text == "none" || text.is_empty() {
        return Strategy::None;
    }
    if let Some(rest) = text.strip_prefix("page:") {
        let parts: Vec<u32> = rest
            .split(':')
            .filter_map(|part| part.trim().parse().ok())
            .collect();
        return match parts.as_slice() {
            [end] if *end > 0 => Strategy::Page {
                start: 1,
                end: (*end).min(MAX_PAGES),
            },
            [start, end] if start <= end => Strategy::Page {
                start: *start,
                end: (*end).min(MAX_PAGES),
            },
            _ => Strategy::None,
        };
    }
    if text == "cursor:next" || text.starts_with("cursor:") {
        return Strategy::Cursor;
    }
    if let Some(rest) = text.strip_prefix("fanout:") {
        let parts: Vec<u32> = rest
            .split(':')
            .filter_map(|part| part.trim().parse().ok())
            .collect();
        return match parts.as_slice() {
            [batch, max_ids] if *batch > 0 && *max_ids > 0 => Strategy::Fanout {
                batch: *batch,
                max_ids: (*max_ids).min(MAX_PAGES),
            },
            _ => Strategy::None,
        };
    }
    if let Some(rest) = text.strip_prefix("offset:") {
        let parts: Vec<u32> = rest
            .split(':')
            .filter_map(|part| part.trim().parse().ok())
            .collect();
        return match parts.as_slice() {
            [step] if *step > 0 => Strategy::Offset {
                step: *step,
                pages: 1,
            },
            [step, pages] if *step > 0 && *pages > 0 => Strategy::Offset {
                step: *step,
                pages: (*pages).min(MAX_PAGES),
            },
            _ => Strategy::None,
        };
    }
    Strategy::None
}

/// Return true when the request template can consume the strategy value.
#[must_use]
pub fn template_supports(request: &str, strategy: Strategy) -> bool {
    strategy
        .query_key()
        .is_some_and(|key| request.contains(&format!("{{query:{key}}}")))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_page_ranges() {
        assert_eq!(parse("page:3"), Strategy::Page { start: 1, end: 3 });
        assert_eq!(parse("page:0:6"), Strategy::Page { start: 0, end: 6 });
        assert_eq!(parse("page:1:8"), Strategy::Page { start: 1, end: 8 });
    }

    #[test]
    fn parses_offset_pages() {
        assert_eq!(
            parse("offset:20:10"),
            Strategy::Offset {
                step: 20,
                pages: 10
            }
        );
        assert_eq!(
            Strategy::Offset { step: 20, pages: 3 }.values(),
            vec![0, 20, 40]
        );
    }

    #[test]
    fn unsupported_strategies_stay_single_request() {
        assert_eq!(parse("cursor:next"), Strategy::Cursor);
        assert_eq!(
            parse("fanout:10:45"),
            Strategy::Fanout {
                batch: 10,
                max_ids: 45
            }
        );
        assert_eq!(parse("numbered pages"), Strategy::None);
    }

    #[test]
    fn template_must_use_the_placeholder() {
        assert!(template_supports(
            "https://x/?page={query:page}",
            parse("page:2")
        ));
        assert!(!template_supports("https://x/?page=1", parse("page:2")));
    }
}
