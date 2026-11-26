//! RSS, Atom, and pattern-based feed parsers.
//!
//! See documentation/006-architecture.md.

// Rust guideline compliant 2026-10-09

use quick_xml::de::from_str;
use serde::Deserialize;

use super::{Parser, Record};
use crate::fetching::Response;

/// Parser for RSS 2.0 and Atom feeds.
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
    #[serde(default)]
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

#[derive(Debug, Deserialize)]
struct Feed {
    #[serde(default)]
    entry: Vec<Entry>,
}

#[derive(Debug, Deserialize)]
struct Entry {
    title: Option<String>,
    #[serde(rename = "link", default)]
    links: Vec<Link>,
    summary: Option<String>,
    #[serde(rename = "location", default)]
    locations: Vec<String>,
}

#[derive(Debug, Deserialize)]
struct Link {
    #[serde(rename = "@rel", default)]
    rel: Option<String>,
    #[serde(rename = "@href", default)]
    href: Option<String>,
    #[serde(rename = "$text", default)]
    text: Option<String>,
}

/// Parser that splits a feed title into title, company, and location.
#[derive(Debug, Clone, Default)]
pub struct PatternRssParser {
    separator: String,
    company_first: bool,
}

impl PatternRssParser {
    /// Build a parser using `A - B - C` or `Company: Title` shapes.
    ///
    /// # Errors
    /// Returns a diagnostic when the separator is empty.
    pub fn new(separator: &str, company_first: bool) -> Result<Self, String> {
        if separator.trim().is_empty() {
            return Err("title separator must not be empty".to_owned());
        }
        Ok(Self {
            separator: separator.to_owned(),
            company_first,
        })
    }
}

impl Parser for PatternRssParser {
    fn parse(&self, response: &Response) -> Result<Vec<Record>, String> {
        let text = response.text()?;
        let feed: Rss = from_str(text).map_err(|error| format!("invalid RSS feed: {error}"))?;
        let mut records = Vec::with_capacity(feed.channel.item.len());
        for item in feed.channel.item {
            let raw = item.title.unwrap_or_default();
            let parts = split_title(&raw, &self.separator);
            let (title, company, location) = if self.company_first {
                (
                    parts.get(1).cloned().unwrap_or_default(),
                    parts.first().cloned().unwrap_or_default(),
                    parts.get(2).cloned(),
                )
            } else {
                (
                    parts.first().cloned().unwrap_or_default(),
                    parts.get(1).cloned().unwrap_or_else(|| {
                        page_host(&item.link.clone().unwrap_or_default())
                            .unwrap_or_else(|| "unknown".to_owned())
                    }),
                    parts.get(2).cloned(),
                )
            };
            let url = item.link.unwrap_or_default();
            let record = Record::new(url, title, company)?.with_location(location);
            records.push(record.with_description(item.description));
        }
        Ok(records)
    }
}

impl Parser for RssParser {
    fn parse(&self, response: &Response) -> Result<Vec<Record>, String> {
        let text = response.text()?;
        if text.contains("<feed") {
            return parse_atom(text);
        }
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

fn parse_atom(text: &str) -> Result<Vec<Record>, String> {
    let feed: Feed = from_str(text).map_err(|error| format!("invalid Atom feed: {error}"))?;
    let mut records = Vec::with_capacity(feed.entry.len());
    for entry in feed.entry {
        let url = entry
            .links
            .iter()
            .find(|link| link.rel.as_deref().unwrap_or("alternate") == "alternate")
            .and_then(|link| link.href.clone())
            .or_else(|| entry.links.first().and_then(|link| link.text.clone()))
            .unwrap_or_default();
        let title = entry.title.unwrap_or_default();
        let company = page_host(&url).unwrap_or_else(|| "unknown".to_owned());
        let location = entry.locations.into_iter().next();
        let record = Record::new(url, title, company)?.with_location(location);
        records.push(record.with_description(entry.summary));
    }
    Ok(records)
}

fn split_title(raw: &str, separator: &str) -> Vec<String> {
    raw.split(separator)
        .map(str::trim)
        .filter(|part| !part.is_empty())
        .map(str::to_owned)
        .collect()
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

    fn response(body: &str) -> Response {
        Response::new(
            "https://example.test/feed",
            200,
            BTreeMap::new(),
            body.as_bytes().to_vec(),
        )
        .expect("bounded response")
    }

    #[test]
    fn parses_rss_items() {
        let body = "<rss><channel><item><title>Engineer</title><link>https://example.test/1</link><description>Work</description></item></channel></rss>";
        let records = RssParser.parse(&response(body)).expect("records parsed");
        assert_eq!(records.len(), 1);
        assert_eq!(records[0].company, "example.test");
    }

    #[test]
    fn parses_atom_entries() {
        let body = "<feed xmlns='http://www.w3.org/2005/Atom'><entry><title>Engineer</title><link rel='alternate' href='https://example.test/2'/><summary>Work</summary><location>Remote</location></entry></feed>";
        let records = RssParser.parse(&response(body)).expect("records parsed");
        assert_eq!(records[0].location.as_deref(), Some("Remote"));
    }

    #[test]
    fn parses_title_pattern() {
        let body = "<rss><channel><item><title>Engineer - Acme - Berlin</title><link>https://example.test/3</link></item></channel></rss>";
        let parser = PatternRssParser::new("-", false).expect("parser built");
        let records = parser.parse(&response(body)).expect("records parsed");
        assert_eq!(records[0].company, "Acme");
        assert_eq!(records[0].location.as_deref(), Some("Berlin"));
    }
}
