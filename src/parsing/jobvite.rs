// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Arin Balyan
// Last modified: 2026-11-27

//! Parser for the Jobvite company XML feed.
//!
//! See documentation/scrapers/jobvite.md.

// Rust guideline compliant 2026-10-09

use quick_xml::de::from_str;
use serde::Deserialize;

use super::{Parser, Record};
use crate::fetching::Response;

/// Parser for `<result><job>...</job></result>` feeds.
#[derive(Debug, Clone, Copy, Default)]
pub struct JobviteParser;

impl JobviteParser {
    /// Create a Jobvite parser.
    #[must_use]
    pub fn new() -> Self {
        Self
    }
}

#[derive(Debug, Deserialize)]
struct Feed {
    #[serde(rename = "job", default)]
    jobs: Vec<Job>,
}

#[derive(Debug, Deserialize)]
struct Job {
    id: Option<String>,
    title: Option<String>,
    location: Option<String>,
    date: Option<String>,
    description: Option<String>,
    #[serde(rename = "detail-url")]
    detail_url: Option<String>,
    #[serde(rename = "apply-url")]
    apply_url: Option<String>,
}

impl Parser for JobviteParser {
    fn parse(&self, response: &Response) -> Result<Vec<Record>, String> {
        let feed: Feed =
            from_str(response.text()?).map_err(|error| format!("invalid Jobvite XML: {error}"))?;
        let mut records = Vec::with_capacity(feed.jobs.len());
        for job in feed.jobs {
            let Some(url) = job.detail_url.or(job.apply_url) else {
                continue;
            };
            let title = job.title.unwrap_or_default();
            let company = super::url_host(&url).unwrap_or_else(|| "unknown".to_owned());
            let record = Record::new(url, title, company)?
                .with_source_job_id(job.id)
                .with_location(job.location)
                .with_description(job.description)
                .with_date_posted(job.date.and_then(normalize_date));
            records.push(record);
        }
        Ok(records)
    }
}

fn normalize_date(value: String) -> Option<String> {
    let parts: Vec<&str> = value.split('/').collect();
    if parts.len() != 3 {
        return Some(value);
    }
    let month: u32 = parts[0].parse().ok()?;
    let day: u32 = parts[1].parse().ok()?;
    let year = parts[2];
    Some(format!("{year}-{month:02}-{day:02}"))
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::collections::BTreeMap;

    #[test]
    fn parses_jobvite_xml() {
        let body = r#"<?xml version="1.0"?><result><job><id>abc</id><title>Engineer</title><location>Remote</location><date>11/17/2026</date><detail-url>https://app.jobvite.com/job?id=abc</detail-url><description>Build.</description></job></result>"#;
        let response = Response::new(
            "https://app.jobvite.com/xml",
            200,
            BTreeMap::new(),
            body.as_bytes().to_vec(),
        )
        .expect("bounded response");
        let records = JobviteParser.parse(&response).expect("parsed");
        assert_eq!(records.len(), 1);
        assert_eq!(records[0].source_job_id.as_deref(), Some("abc"));
        assert_eq!(records[0].date_posted.as_deref(), Some("2026-11-17"));
    }
}
