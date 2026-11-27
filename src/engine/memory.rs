//! Process memory ceiling for scrape runs.
//!
//! See documentation/006-architecture.md.

// Rust guideline compliant 2026-10-09

/// Ceiling in bytes, read from /proc/self/status on Linux.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct MemoryCeiling {
    /// Maximum resident bytes allowed for the process.
    pub max_rss_bytes: u64,
}

impl MemoryCeiling {
    /// Build a ceiling from a configured megabyte value.
    #[must_use]
    pub fn from_mb(megabytes: u64) -> Self {
        Self {
            max_rss_bytes: megabytes.saturating_mul(1024 * 1024),
        }
    }

    /// Return true when the process is still below the ceiling.
    #[must_use]
    pub fn allows_start(&self) -> bool {
        resident_bytes().is_none_or(|rss| rss < self.max_rss_bytes)
    }
}

fn resident_bytes() -> Option<u64> {
    let status = std::fs::read_to_string("/proc/self/status").ok()?;
    let line = status.lines().find(|line| line.starts_with("VmRSS:"))?;
    let value = line.split_whitespace().nth(1)?;
    value.parse::<u64>().ok().map(|kilobytes| kilobytes * 1024)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn ceiling_scales_megabytes() {
        let ceiling = MemoryCeiling::from_mb(512);
        assert_eq!(ceiling.max_rss_bytes, 512 * 1024 * 1024);
    }

    #[test]
    fn process_below_generous_ceiling_can_start() {
        let ceiling = MemoryCeiling::from_mb(1024 * 1024);
        assert!(ceiling.allows_start());
    }
}
