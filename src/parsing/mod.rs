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
    /// ISO timestamp or date the job was posted.
    pub date_posted: Option<String>,
    /// Compensation text or range.
    pub compensation: Option<String>,
    /// How the candidate should apply.
    pub apply_method: Option<String>,
    /// Direct application URL.
    pub job_url_direct: Option<String>,
    /// Company website URL.
    pub company_url: Option<String>,
    /// Industry or category label.
    pub industry: Option<String>,
    /// Remote flag when the source states it.
    pub is_remote: Option<bool>,
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
            date_posted: None,
            compensation: None,
            apply_method: None,
            job_url_direct: None,
            company_url: None,
            industry: None,
            is_remote: None,
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

    /// Attach the posted timestamp or date.
    #[must_use]
    pub fn with_date_posted(mut self, value: Option<String>) -> Self {
        self.date_posted = value.filter(|value| !value.trim().is_empty());
        self
    }

    /// Attach compensation text.
    #[must_use]
    pub fn with_compensation(mut self, value: Option<String>) -> Self {
        self.compensation = value.filter(|value| !value.trim().is_empty());
        self
    }

    /// Attach the application instructions.
    #[must_use]
    pub fn with_apply_method(mut self, value: Option<String>) -> Self {
        self.apply_method = value.filter(|value| !value.trim().is_empty());
        self
    }

    /// Attach a direct application URL.
    #[must_use]
    pub fn with_job_url_direct(mut self, value: Option<String>) -> Self {
        self.job_url_direct = value.filter(|value| !value.trim().is_empty());
        self
    }

    /// Attach the company website.
    #[must_use]
    pub fn with_company_url(mut self, value: Option<String>) -> Self {
        self.company_url = value.filter(|value| !value.trim().is_empty());
        self
    }

    /// Attach an industry label.
    #[must_use]
    pub fn with_industry(mut self, value: Option<String>) -> Self {
        self.industry = value.filter(|value| !value.trim().is_empty());
        self
    }

    /// Attach a remote flag.
    #[must_use]
    pub fn with_is_remote(mut self, value: Option<bool>) -> Self {
        self.is_remote = value;
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
