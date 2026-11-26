//! RSS and Atom feed parsers.
//!
//! See documentation/006-architecture.md.

// Rust guideline compliant 2026-10-09

use quick_xml::de::from_str;
use serde::Deserialize;

use super::{Parser, Record};
use crate::fetching::Response;

/// Parser for RSS 2.0 item elements.
#[derive(Debug, Clone, Copy, Default)]
pub struct RssParser;

impl RssParser {
    /// Create an RSS parser.
    #[must_use]
    pub fn new() -> Self {
        Self
    }
}

#[derive(Debug, Deserialize)]
struct Channel {
    item: Vec<Item>,
}

#[derive(Debug, Deserialize)]
struct Item {
    title: Option<String>,
    link: Option<String>,
    description: Option<String>,
}

#[derive(Debug, Deserialize)]
struct Rss {
    channel: Channel,
}

impl Parser for RssParser {
    fn parse(&self, response: &Response) -> Result<Vec<Record>, String> {
        let text = response.text()?;
        let feed: Rss = from_str(text).map_err(|error| format!("invalid RSS feed: {error}"))?;
        let mut records = Vec::with_capacity(feed.channel.item.len());
        for item in feed.channel.item {
            let title = item.title.unwrap_or_default();
            let url = item.link.unwrap_or_default();
            let company = page_host(&url).unwrap_or_else(|| "unknown".to_owned());
            let record = Record::new(url, title, company)?;
            records.push(record.with_description(item.description));
        }
        Ok(records)
    }
}

fn page_host(url: &str) -> Option<String> {
    let rest = url.split_once("://")?.1;
    let host = rest.split('/').next().unwrap_or_default();
    if host.is_empty() {
        None
    } else {
        Some(host.to_owned())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::collections::BTreeMap;

    #[test]
    fn parses_rss_items() {
        let body = "<rss><channel><item><title>Engineer</title><link>https://example.test/1</link><description>Work</description></item></channel></rss>";
        let response = Response::new(
            "https://example.test/feed",
            200,
            BTreeMap::new(),
            body.as_bytes().to_vec(),
        )
        .expect("bounded response");
        let records = RssParser.parse(&response).expect("records parsed");
        assert_eq!(records.len(), 1);
        assert_eq!(records[0].company, "example.test");
    }
}
