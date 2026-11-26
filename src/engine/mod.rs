//! Scrape engine and runtime contract.
//!
//! See documentation/006-architecture.md.

// Rust guideline compliant 2026-10-09

pub mod breaker;
pub mod challenge;
pub mod contract;
pub mod dedup;
pub mod governor;
pub mod limits;
pub mod retry;
