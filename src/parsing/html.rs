//! CSS-selector HTML parsers.
//!
//! See documentation/006-architecture.md.

// Rust guideline compliant 2026-10-09

use scraper::{Html, Selector};

use super::{Parser, Record};
use crate::fetching::Response;

/// Parser for HTML pages driven by declarative selectors.
#[derive(Debug, Clone, Default)]
pub struct HtmlParser {
    item_selector: String,
    title_selector: String,
    company_selector: String,
    url_selector: String,
    url_attribute: String,
}

impl HtmlParser {
    /// Build a parser from CSS selectors.
    ///
    /// # Errors
    /// Returns a diagnostic when a selector is invalid.
    pub fn new(
        item_selector: impl Into<String>,
        title_selector: impl Into<String>,
        company_selector: impl Into<String>,
        url_selector: impl Into<String>,
        url_attribute: impl Into<String>,
    ) -> Result<Self, String> {
        let item_selector = item_selector.into();
        let title_selector = title_selector.into();
        let company_selector = company_selector.into();
        let url_selector = url_selector.into();
        let url_attribute = url_attribute.into();
        for selector in [
            &item_selector,
            &title_selector,
            &company_selector,
            &url_selector,
        ] {
            Selector::parse(selector)
                .map_err(|error| format!("invalid CSS selector {selector}: {error}"))?;
        }
        Ok(Self {
            item_selector,
            title_selector,
            company_selector,
            url_selector,
            url_attribute,
        })
    }
}

impl Parser for HtmlParser {
    fn parse(&self, response: &Response) -> Result<Vec<Record>, String> {
        let document = Html::parse_document(response.text()?);
        let items = Selector::parse(&self.item_selector)
            .map_err(|error| format!("invalid item selector: {error}"))?;
        let titles = Selector::parse(&self.title_selector)
            .map_err(|error| format!("invalid title selector: {error}"))?;
        let companies = Selector::parse(&self.company_selector)
            .map_err(|error| format!("invalid company selector: {error}"))?;
        let urls = Selector::parse(&self.url_selector)
            .map_err(|error| format!("invalid URL selector: {error}"))?;
        let mut records = Vec::new();
        for item in document.select(&items) {
            let title = text_of(item, &titles);
            let company = text_of(item, &companies);
            let url = attribute_of(item, &urls, &self.url_attribute);
            if let Ok(record) = Record::new(url, title, company) {
                records.push(record);
            }
        }
        if records.is_empty() {
            return Err("no records matched the configured selectors".to_owned());
        }
        Ok(records)
    }
}

fn text_of(root: scraper::ElementRef<'_>, selector: &Selector) -> String {
    selected(root, selector)
        .map_or_else(String::new, |node| node.text().collect::<String>())
        .trim()
        .to_owned()
}

fn attribute_of(root: scraper::ElementRef<'_>, selector: &Selector, name: &str) -> String {
    selected(root, selector)
        .and_then(|node| node.value().attr(name))
        .unwrap_or_default()
        .trim()
        .to_owned()
}

/// Prefer the item itself when the field selector matches it, then descendants.
fn selected<'a>(
    root: scraper::ElementRef<'a>,
    selector: &Selector,
) -> Option<scraper::ElementRef<'a>> {
    if selector.matches(&root) {
        Some(root)
    } else {
        root.select(selector).next()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::collections::BTreeMap;

    #[test]
    fn parses_jobs_with_selectors() {
        let body = "<div class='job'><a href='/jobs/1'>Engineer</a><span>Acme</span></div>";
        let response = Response::new(
            "https://example.test/jobs",
            200,
            BTreeMap::new(),
            body.as_bytes().to_vec(),
        )
        .expect("bounded response");
        let parser = HtmlParser::new(".job", "a", "span", "a", "href").expect("valid selectors");
        let records = parser.parse(&response).expect("records parsed");
        assert_eq!(records[0].title, "Engineer");
        assert_eq!(records[0].company, "Acme");
    }

    #[test]
    fn parses_item_that_is_also_the_link() {
        let body = "<a class='job' href='/jobs/1'><span>Engineer</span><b>Acme</b></a>";
        let response = Response::new(
            "https://example.test/jobs",
            200,
            BTreeMap::new(),
            body.as_bytes().to_vec(),
        )
        .expect("bounded response");
        let parser =
            HtmlParser::new(".job", "span", "b", "a.job", "href").expect("valid selectors");
        let records = parser.parse(&response).expect("records parsed");
        assert_eq!(records[0].url, "/jobs/1");
    }

    #[test]
    fn rejects_invalid_selector() {
        assert!(
            HtmlParser::new("[", "a", "span", "a", "href")
                .unwrap_err()
                .contains("invalid CSS selector")
        );
    }
}
