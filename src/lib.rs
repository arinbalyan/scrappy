//! Library boundary for the scrappy command-line tool.
//!
//! Command handling, configuration, source manifests, fetching, parsing,
//! and output are organized by subsystem. See documentation/006-architecture.md.

// Rust guideline compliant 2026-10-09

pub mod cli;

#[path = "config/mod.rs"]
pub mod config;

pub mod sources;

pub mod fetching;

pub mod parsing;

pub mod engine;

pub mod output;

pub mod security;

pub mod checkpoint;

pub mod metrics;
