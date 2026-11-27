// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Arin Balyan
// Last modified: 2026-11-27

//! Shared parsers for fetched responses.
//!
//! Parsers take a bounded `Response`, return normalized records, and never
//! call the network. See documentation/006-architecture.md.

// Rust guideline compliant 2026-10-09

pub mod html;
pub mod jobvite;
pub mod json;
pub mod jsonld;
pub mod rss;

use serde::{Deserialize, Serialize};

/// Hosts owned by applicant tracking systems or job boards.
///
/// Matched by exact host or subdomain, so `boards.greenhouse.io`
/// matches `greenhouse.io`.
pub const TRACKING_HOSTS: &[&str] = &[
    "ashbyhq.com",
    "bamboohr.com",
    "breezy.hr",
    "builtin.com",
    "careerbuilder.com",
    "dice.com",
    "glassdoor.com",
    "greenhouse.io",
    "himalayas.app",
    "hiring.cafe",
    "icims.com",
    "indeed.com",
    "jobicy.com",
    "jobvite.com",
    "justremote.co",
    "lever.co",
    "linkedin.com",
    "monster.com",
    "news.ycombinator.com",
    "nofluffjobs.com",
    "personio.com",
    "recruitee.com",
    "remote.co",
    "remoteok.com",
    "remotive.com",
    "rippling.com",
    "simplyhired.com",
    "smartrecruiters.com",
    "stackoverflow.com",
    "taleo.net",
    "teamtailor.com",
    "wellfound.com",
    "weworkremotely.com",
    "workable.com",
    "workatastartup.com",
    "workday.com",
    "workingnomads.com",
    "myworkdayjobs.com",
    "ycombinator.com",
    "ziprecruiter.com",
];

/// How a candidate submits an application.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum ApplyRoute {
    /// An HTTP application page.
    Url,
    /// An email address.
    Email,
    /// A third-party applicant tracking system.
    Ats,
    /// No usable route was supplied.
    Unknown,
}

/// Whether a host belongs to an applicant tracking system or job board.
#[must_use]
pub fn is_ats_or_job_board_host(host: &str) -> bool {
    let host = host.trim().trim_end_matches('.').to_ascii_lowercase();
    let host = host.strip_prefix("www.").unwrap_or(&host);
    TRACKING_HOSTS.iter().any(|domain| {
        host == *domain
            || host
                .strip_suffix(domain)
                .is_some_and(|prefix| prefix.ends_with('.'))
    })
}

/// Extract a lower-case host from an absolute URL.
#[must_use]
pub fn url_host(url: &str) -> Option<String> {
    let (_, rest) = url.split_once("://")?;
    let authority = rest.split(['/', '?', '#']).next()?;
    let authority = authority.rsplit('@').next().unwrap_or_default();
    let host = authority
        .strip_prefix('[')
        .and_then(|value| value.split(']').next())
        .unwrap_or_else(|| authority.split(':').next().unwrap_or_default());
    let host = host.trim_end_matches('.').to_ascii_lowercase();
    (!host.is_empty()).then_some(host)
}

/// Classify how a record's application should be submitted.
#[must_use]
pub fn classify_apply_route(method: Option<&str>, direct_url: Option<&str>) -> ApplyRoute {
    let method = method.unwrap_or_default().trim().to_ascii_lowercase();
    let url = direct_url.unwrap_or_default().trim();
    let lower_url = url.to_ascii_lowercase();
    let looks_like_email = url.contains('@') && !url.contains('/') && !url.contains(':');
    if lower_url.starts_with("mailto:") || method.contains("email") || looks_like_email {
        return ApplyRoute::Email;
    }
    if method.contains("ats") {
        return ApplyRoute::Ats;
    }
    if let Some(host) = url_host(url) {
        if is_ats_or_job_board_host(&host) {
            return ApplyRoute::Ats;
        }
        if lower_url.starts_with("http://") || lower_url.starts_with("https://") {
            return ApplyRoute::Url;
        }
    }
    if method.contains("url") || method.contains("direct") || method.contains("web") {
        return ApplyRoute::Url;
    }
    ApplyRoute::Unknown
}

/// Resolve a possibly relative URL against an absolute base URL.
#[must_use]
pub fn resolve_url(base: &str, reference: &str) -> Option<String> {
    let reference = reference.trim();
    if reference.is_empty() {
        return Some(base.to_owned());
    }
    if has_scheme(reference) {
        return Some(reference.to_owned());
    }
    if reference.starts_with("//") {
        let (scheme, _) = base.split_once("://")?;
        return Some(format!("{scheme}:{reference}"));
    }
    let (scheme, rest) = base.split_once("://")?;
    let (authority, base_path) = match rest.split_once('/') {
        Some((authority, path)) => (authority, format!("/{path}")),
        None => (rest, String::new()),
    };
    if authority.is_empty() {
        return None;
    }
    let base_path = base_path.split(['?', '#']).next().unwrap_or_default();
    if let Some(query) = reference.strip_prefix('?') {
        return Some(format!("{scheme}://{authority}{base_path}?{query}"));
    }
    if reference.starts_with('#') {
        return Some(format!("{scheme}://{authority}{base_path}{reference}"));
    }
    let (reference_path, suffix) = split_reference(reference);
    let path = if reference_path.starts_with('/') {
        normalize_path(reference_path)
    } else {
        let directory = base_path.rsplit_once('/').map_or("/", |(head, _)| head);
        normalize_path(&format!("{directory}/{reference_path}"))
    };
    Some(format!("{scheme}://{authority}{path}{suffix}"))
}

/// Normalize a recognized posting time to ISO 8601.
#[must_use]
pub fn normalize_posted_time(raw: &str) -> Option<String> {
    let value = raw.trim();
    if value.is_empty() {
        return None;
    }
    normalize_iso8601(value).or_else(|| normalize_rfc2822(value))
}

fn has_scheme(value: &str) -> bool {
    value.split_once(':').is_some_and(|(scheme, _)| {
        !scheme.is_empty()
            && scheme
                .chars()
                .all(|ch| ch.is_ascii_alphanumeric() || matches!(ch, '+' | '-' | '.'))
    })
}

fn split_reference(reference: &str) -> (&str, &str) {
    match reference.find(['?', '#']) {
        Some(index) => (&reference[..index], &reference[index..]),
        None => (reference, ""),
    }
}

fn normalize_path(path: &str) -> String {
    let trailing = path.ends_with('/');
    let mut segments: Vec<&str> = Vec::new();
    for segment in path.split('/') {
        match segment {
            "" | "." => {}
            ".." => {
                segments.pop();
            }
            other => segments.push(other),
        }
    }
    let mut normalized = format!("/{}", segments.join("/"));
    if trailing && !normalized.ends_with('/') {
        normalized.push('/');
    }
    normalized
}

fn normalize_iso8601(value: &str) -> Option<String> {
    let (date, rest) = value.split_at_checked(10)?;
    let mut parts = date.split('-');
    let year: i32 = parts.next()?.parse().ok()?;
    let month: u32 = parts.next()?.parse().ok()?;
    let day: u32 = parts.next()?.parse().ok()?;
    if parts.next().is_some() || !valid_date(year, month, day) {
        return None;
    }
    let date = format!("{year:04}-{month:02}-{day:02}");
    if rest.is_empty() {
        return Some(date);
    }
    if !rest.starts_with(['T', 't', ' ']) {
        return None;
    }
    let rest = rest[1..].trim();
    if rest.is_empty() {
        return Some(format!("{date}T00:00:00Z"));
    }
    let (clock, zone) = split_time_zone(rest);
    let clock = normalize_clock(clock)?;
    let zone = zone.map_or_else(|| Some("Z".to_owned()), normalize_zone)?;
    Some(format!("{date}T{clock}{zone}"))
}

fn normalize_rfc2822(value: &str) -> Option<String> {
    let tokens: Vec<&str> = value
        .split(|ch: char| ch.is_whitespace() || ch == ',')
        .filter(|token| !token.is_empty())
        .collect();
    let month_index = tokens
        .iter()
        .position(|token| month_number(token).is_some())?;
    let day: u32 = tokens.get(month_index.checked_sub(1)?)?.parse().ok()?;
    let month = month_number(tokens[month_index])?;
    let year: i32 = tokens.get(month_index + 1)?.parse().ok()?;
    if !valid_date(year, month, day) {
        return None;
    }
    let time_index = tokens.iter().position(|token| token.contains(':'));
    let (clock, zone) = time_index.map_or_else(
        || ("00:00:00".to_owned(), Some("Z".to_owned())),
        |index| {
            let clock = normalize_clock(tokens[index]).unwrap_or_else(|| "00:00:00".to_owned());
            let zone = tokens
                .get(index + 1)
                .and_then(|token| normalize_zone(token));
            (clock, zone)
        },
    );
    Some(format!(
        "{year:04}-{month:02}-{day:02}T{clock}{}",
        zone.unwrap_or_else(|| "Z".to_owned())
    ))
}

fn month_number(token: &str) -> Option<u32> {
    const MONTHS: [&str; 12] = [
        "jan", "feb", "mar", "apr", "may", "jun", "jul", "aug", "sep", "oct", "nov", "dec",
    ];
    let token = token.get(..3)?.to_ascii_lowercase();
    MONTHS
        .iter()
        .position(|month| *month == token)
        .and_then(|index| u32::try_from(index + 1).ok())
}

fn valid_date(year: i32, month: u32, day: u32) -> bool {
    if year < 1900 || !(1..=12).contains(&month) || day == 0 {
        return false;
    }
    let days = match month {
        1 | 3 | 5 | 7 | 8 | 10 | 12 => 31,
        4 | 6 | 9 | 11 => 30,
        2 if is_leap_year(year) => 29,
        2 => 28,
        _ => return false,
    };
    day <= days
}

fn is_leap_year(year: i32) -> bool {
    (year % 4 == 0 && year % 100 != 0) || year % 400 == 0
}

fn normalize_clock(clock: &str) -> Option<String> {
    let clock = clock.trim().split('.').next().unwrap_or_default();
    let mut parts = clock.split(':');
    let hour: u32 = parts.next()?.parse().ok()?;
    let minute: u32 = parts.next()?.parse().ok()?;
    let second: u32 = parts.next().unwrap_or("0").parse().ok()?;
    if hour > 23 || minute > 59 || second > 59 {
        return None;
    }
    Some(format!("{hour:02}:{minute:02}:{second:02}"))
}

fn split_time_zone(rest: &str) -> (&str, Option<&str>) {
    let rest = rest.trim();
    if let Some((clock, zone)) = rest.rsplit_once(' ')
        && looks_like_zone(zone)
    {
        return (clock.trim(), Some(zone.trim()));
    }
    if let Some(offset) = rest.rfind(['+', '-'])
        && offset > 0
    {
        return (&rest[..offset], Some(&rest[offset..]));
    }
    if let Some(last) = rest.chars().last()
        && last.eq_ignore_ascii_case(&'z')
    {
        let end = rest.len() - last.len_utf8();
        return (&rest[..end], Some(&rest[end..]));
    }
    (rest, None)
}

fn looks_like_zone(value: &str) -> bool {
    value.eq_ignore_ascii_case("z")
        || value.eq_ignore_ascii_case("utc")
        || value.eq_ignore_ascii_case("gmt")
        || value.eq_ignore_ascii_case("ut")
        || value.starts_with(['+', '-'])
}

fn normalize_zone(zone: &str) -> Option<String> {
    let zone = zone.trim();
    if looks_like_zone(zone) && zone.len() == 1 {
        return Some("Z".to_owned());
    }
    if zone.eq_ignore_ascii_case("utc")
        || zone.eq_ignore_ascii_case("gmt")
        || zone.eq_ignore_ascii_case("ut")
    {
        return Some("Z".to_owned());
    }
    let (sign, digits) = zone.split_at_checked(1)?;
    if sign != "+" && sign != "-" {
        return None;
    }
    match digits.len() {
        4 if digits.chars().all(|ch| ch.is_ascii_digit()) => {
            let (hours, minutes) = digits.split_at(2);
            Some(format!("{sign}{hours}:{minutes}"))
        }
        5 if digits.as_bytes()[2] == b':'
            && digits.chars().filter(char::is_ascii_digit).count() == 4 =>
        {
            Some(zone.to_owned())
        }
        _ => None,
    }
}

/// One normalized job record produced by a parser.
#[derive(Debug, Clone, PartialEq)]
pub struct Record {
    /// Canonical job URL.
    pub url: String,
    /// Source-native job identifier when supplied.
    pub source_job_id: Option<String>,
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
    /// Lower salary bound when supplied as a number.
    pub salary_min: Option<f64>,
    /// Upper salary bound when supplied as a number.
    pub salary_max: Option<f64>,
    /// ISO currency code or symbol for the salary.
    pub salary_currency: Option<String>,
    /// Pay period such as `year`, `month`, or `hour`.
    pub salary_period: Option<String>,
    /// How the candidate should apply.
    pub apply_method: Option<String>,
    /// Typed route used to submit an application.
    pub apply_route: Option<ApplyRoute>,
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
            source_job_id: None,
            title,
            company,
            location: None,
            description: None,
            date_posted: None,
            compensation: None,
            salary_min: None,
            salary_max: None,
            salary_currency: None,
            salary_period: None,
            apply_method: None,
            apply_route: None,
            job_url_direct: None,
            company_url: None,
            industry: None,
            is_remote: None,
        })
    }

    /// Attach the source-native job identifier.
    #[must_use]
    pub fn with_source_job_id(mut self, value: Option<String>) -> Self {
        self.source_job_id = value.filter(|value| !value.trim().is_empty());
        self
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

    /// Attach salary bounds, currency, and pay period.
    #[must_use]
    pub fn with_salary(
        mut self,
        min: Option<f64>,
        max: Option<f64>,
        currency: Option<String>,
        period: Option<String>,
    ) -> Self {
        self.salary_min = min;
        self.salary_max = max;
        self.salary_currency = currency.filter(|value| !value.trim().is_empty());
        self.salary_period = period.filter(|value| !value.trim().is_empty());
        self
    }

    /// Attach the application instructions.
    #[must_use]
    pub fn with_apply_method(mut self, value: Option<String>) -> Self {
        self.apply_method = value.filter(|value| !value.trim().is_empty());
        self
    }

    /// Attach a typed application route.
    #[must_use]
    pub fn with_apply_route(mut self, value: Option<ApplyRoute>) -> Self {
        self.apply_route = value;
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

    /// Resolve relative job, apply, and company URLs against a base URL.
    pub fn resolve_relative_urls(&mut self, base: &str) {
        if let Some(url) = resolve_url(base, &self.url) {
            self.url = url;
        }
        self.job_url_direct = self
            .job_url_direct
            .take()
            .and_then(|url| resolve_url(base, &url));
        self.company_url = self
            .company_url
            .take()
            .and_then(|url| resolve_url(base, &url));
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
