//! Browser fetcher placeholder behind the browser feature.
//!
//! See documentation/012-browser-automation.md.

// Rust guideline compliant 2026-10-09

/// Reports whether this build includes browser automation.
#[must_use]
pub fn available() -> bool {
    cfg!(feature = "browser")
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn reports_build_feature() {
        assert_eq!(available(), cfg!(feature = "browser"));
    }
}
