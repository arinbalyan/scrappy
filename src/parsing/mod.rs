//! Shared parsers for fetched responses.
//!
//! Parsers take a bounded `Response`, return normalized records, and never
//! call the network. See documentation/006-architecture.md.

// Rust guideline compliant 2026-10-09

pub mod html;
pub mod json;
pub mod jsonld;
pub mod rss;

/// One normalized job record produced by a parser.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Record {
    /// Canonical job URL.
    pub url: String,
    /// Job title.
    pub title: String,
    /// Hiring company.
    pub company: String,
    /// Optional location text.
    pub location: Option<String>,
    /// Optional description text.
    pub description: Option<String>,
}

impl Record {
    /// Build a record after validating required fields.
    ///
    /// # Errors
    /// Returns a diagnostic when a required field is empty.
    pub fn new(
        url: impl Into<String>,
        title: impl Into<String>,
        company: impl Into<String>,
    ) -> Result<Self, String> {
        let url = url.into();
        let title = title.into();
        let company = company.into();
        if url.trim().is_empty() {
            return Err("record url must not be empty".to_owned());
        }
        if title.trim().is_empty() {
            return Err("record title must not be empty".to_owned());
        }
        if company.trim().is_empty() {
            return Err("record company must not be empty".to_owned());
        }
        Ok(Self {
            url,
            title,
            company,
            location: None,
            description: None,
        })
    }

    /// Attach optional location text.
    #[must_use]
    pub fn with_location(mut self, location: Option<String>) -> Self {
        self.location = location.filter(|value| !value.trim().is_empty());
        self
    }

    /// Attach optional description text.
    #[must_use]
    pub fn with_description(mut self, description: Option<String>) -> Self {
        self.description = description.filter(|value| !value.trim().is_empty());
        self
    }
}

/// Parse a bounded response into normalized records.
pub trait Parser {
    /// Parse one response.
    ///
    /// # Errors
    /// Returns a diagnostic when the payload has an unsupported shape.
    fn parse(&self, response: &crate::fetching::Response) -> Result<Vec<Record>, String>;
}
