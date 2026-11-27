//! Secret handling and outbound target validation.
//!
//! The helpers here keep credentials and personal data out of logs and test
//! fixtures, and narrow outbound targets to addresses that are safe to dial.
//! See documentation/015-security.md.

// Rust guideline compliant 2026-10-09

use std::fmt;
use std::net::{IpAddr, Ipv4Addr, Ipv6Addr, ToSocketAddrs};

/// Marker used in place of credential values and long tokens.
const REDACTED: &str = "[redacted]";
/// Marker used in place of URL query text.
const QUERY_REDACTED: &str = "[query-redacted]";
/// Marker used in place of fixture email addresses.
const EMAIL_PLACEHOLDER: &str = "[email]";
/// Marker used in place of fixture phone numbers.
const PHONE_PLACEHOLDER: &str = "[phone]";
/// Marker used in place of fixture token-like strings.
const TOKEN_PLACEHOLDER: &str = "[token]";

// Matches the challenge evidence threshold; longer runs are treated as secrets.
const TOKEN_THRESHOLD: usize = 24;
// E.164 caps numbers at 15 digits; seven digits covers short local numbers.
const MIN_PHONE_DIGITS: usize = 7;
const MAX_PHONE_DIGITS: usize = 15;

/// A credential whose formatted forms never reveal the value.
///
/// Use `as_str` only at the authentication boundary.
#[derive(Clone, PartialEq, Eq, Hash)]
pub struct Secret(String);

impl From<&str> for Secret {
    fn from(value: &str) -> Self {
        Self(value.to_owned())
    }
}

impl From<String> for Secret {
    fn from(value: String) -> Self {
        Self(value)
    }
}

impl Secret {
    /// Return the credential for authenticated use.
    #[must_use]
    pub fn as_str(&self) -> &str {
        &self.0
    }
}

impl fmt::Debug for Secret {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(REDACTED)
    }
}

impl fmt::Display for Secret {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(REDACTED)
    }
}

/// Mask URL userinfo, query text, and fragments while keeping scheme, host, and path.
///
/// Userinfo is replaced with a redaction marker and any query is replaced with
/// a query marker. Fragments are dropped because they can carry tokens.
///
/// # Examples
///
/// ```
/// use scrappy::security::redact_url;
/// let safe = redact_url("https://user:pass@example.test/jobs?token=abc#frag");
/// assert_eq!(safe, "https://[redacted]@example.test/jobs?[query-redacted]");
/// ```
#[must_use]
pub fn redact_url(url: &str) -> String {
    let head = url.split_once('#').map_or(url, |(before, _)| before);
    let (base, has_query) = match head.split_once('?') {
        Some((before, _)) => (before, true),
        None => (head, false),
    };
    let authority_start = base.find("://").map_or(0, |index| index + 3);
    let (prefix, rest) = base.split_at(authority_start);
    let authority_end = rest.find('/').unwrap_or(rest.len());
    let (authority, tail) = rest.split_at(authority_end);

    let mut out = String::with_capacity(base.len() + QUERY_REDACTED.len());
    out.push_str(prefix);
    match authority.rfind('@') {
        Some(at) => {
            out.push_str(REDACTED);
            out.push('@');
            out.push_str(&authority[at + 1..]);
        }
        None => out.push_str(authority),
    }
    out.push_str(tail);
    if has_query {
        out.push('?');
        out.push_str(QUERY_REDACTED);
    }
    out
}

/// Mask long tokens and query strings in untrusted text.
///
/// This delegates to `crate::engine::challenge::scrub`, which also caps the
/// result at `crate::engine::challenge::MAX_EVIDENCE_CHARS`.
#[must_use]
pub fn redact_text(text: &str) -> String {
    crate::engine::challenge::scrub(text)
}

/// Validate one outbound HTTP target and return the addresses to connect to.
///
/// Hostnames are resolved with `ToSocketAddrs`; literal addresses are checked
/// before any resolution. Callers must connect only to a returned address and
/// must validate each redirect target separately.
///
/// # Errors
///
/// Returns a diagnostic for an unsupported scheme, an invalid host or port,
/// a failed resolution, or an address in a blocked range.
pub fn validate_target(url: &str) -> Result<Vec<IpAddr>, String> {
    let (scheme, authority) = split_target(url)?;
    let (host, port) = split_authority(scheme, authority)?;
    let addresses = resolve_host(host, port)?;
    if addresses.iter().any(|address| is_forbidden(*address)) {
        return Err("target address is blocked".to_owned());
    }
    Ok(addresses)
}

/// Resolve a hostname and return only public addresses.
///
/// HTTP clients use this at connection time so rebinding cannot redirect a
/// validated hostname to a private address.
///
/// # Errors
/// Returns a diagnostic when resolution fails or no public address exists.
pub fn resolve_public_host(host: &str) -> Result<Vec<IpAddr>, String> {
    let target = TargetHost::Name(host);
    let addresses = resolve_host(target, 0)?;
    let allowed: Vec<_> = addresses
        .into_iter()
        .filter(|address| !is_forbidden(*address))
        .collect();
    if allowed.is_empty() {
        return Err("target host resolved only to blocked addresses".to_owned());
    }
    Ok(allowed)
}

/// Host parsed from a target authority.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum TargetHost<'a> {
    /// Literal address that needs no resolution.
    Ip(IpAddr),
    /// Hostname that must be resolved.
    Name(&'a str),
}

fn split_target(url: &str) -> Result<(&str, &str), String> {
    let Some(index) = url.find("://") else {
        return Err("target URL must include a scheme".to_owned());
    };
    let scheme = &url[..index];
    if !scheme.eq_ignore_ascii_case("http") && !scheme.eq_ignore_ascii_case("https") {
        return Err("unsupported target URL scheme".to_owned());
    }
    let rest = &url[index + 3..];
    let authority_end = rest.find(['/', '?', '#']).unwrap_or(rest.len());
    let authority = &rest[..authority_end];
    let authority = authority
        .rsplit_once('@')
        .map_or(authority, |(_, host)| host);
    if authority.is_empty() {
        return Err("target URL has no host".to_owned());
    }
    Ok((scheme, authority))
}

fn split_authority<'a>(scheme: &str, authority: &'a str) -> Result<(TargetHost<'a>, u16), String> {
    let default_port = if scheme.eq_ignore_ascii_case("https") {
        443
    } else {
        80
    };
    let (host_part, port_part) = if let Some(after_open) = authority.strip_prefix('[') {
        let Some(close) = after_open.find(']') else {
            return Err("invalid IPv6 authority".to_owned());
        };
        let host = &after_open[..close];
        let remainder = &after_open[close + 1..];
        let port = match remainder {
            "" => None,
            rest => rest.strip_prefix(':'),
        };
        if port.is_none() && !remainder.is_empty() {
            return Err("invalid characters after IPv6 host".to_owned());
        }
        (host, port)
    } else if authority.matches(':').count() > 1 {
        (authority, None)
    } else {
        match authority.rsplit_once(':') {
            Some((host, port)) => (host, Some(port)),
            None => (authority, None),
        }
    };

    let host = if let Ok(address) = host_part.parse::<IpAddr>() {
        TargetHost::Ip(address)
    } else {
        if host_part.is_empty() || host_part.contains(['[', ']']) {
            return Err("invalid target host".to_owned());
        }
        TargetHost::Name(host_part)
    };
    let port = match port_part {
        Some(raw) => raw.parse::<u16>().map_err(|parse_error| {
            let _ = parse_error;
            "invalid target port".to_owned()
        })?,
        None => default_port,
    };
    if port == 0 {
        return Err("invalid target port".to_owned());
    }
    Ok((host, port))
}

fn resolve_host(host: TargetHost<'_>, port: u16) -> Result<Vec<IpAddr>, String> {
    match host {
        TargetHost::Ip(address) => Ok(vec![address]),
        TargetHost::Name(name) => {
            let resolved = (name, port).to_socket_addrs().map_err(|resolve_error| {
                let _ = resolve_error;
                "target host resolution failed".to_owned()
            })?;
            let mut addresses = Vec::new();
            for socket in resolved {
                let address = socket.ip();
                if !addresses.contains(&address) {
                    addresses.push(address);
                }
            }
            if addresses.is_empty() {
                return Err("target host resolved to no addresses".to_owned());
            }
            Ok(addresses)
        }
    }
}

fn is_forbidden(address: IpAddr) -> bool {
    match address {
        IpAddr::V4(v4) => is_forbidden_v4(v4),
        IpAddr::V6(v6) => is_forbidden_v6(v6),
    }
}

fn is_forbidden_v4(address: Ipv4Addr) -> bool {
    let [a, b, c, _] = address.octets();
    a == 0
        || a == 10
        || a == 127
        || a >= 224
        || (a == 172 && (16..=31).contains(&b))
        || (a == 192 && b == 168)
        || (a == 169 && b == 254)
        || (a == 100 && (64..=127).contains(&b))
        || (a == 192 && b == 0 && (c == 0 || c == 2))
        || (a == 198 && ((18..=19).contains(&b) || (b == 51 && c == 100)))
        || (a == 203 && b == 0 && c == 113)
}

fn is_forbidden_v6(address: Ipv6Addr) -> bool {
    if let Some(v4) = address.to_ipv4_mapped() {
        return is_forbidden_v4(v4);
    }
    if address.is_unspecified() || address == Ipv6Addr::LOCALHOST {
        return true;
    }
    let segments = address.segments();
    let [first, second, ..] = segments;
    if (first & 0xfe00) == 0xfc00 || (first & 0xffc0) == 0xfe80 {
        return true;
    }
    if (first & 0xff00) == 0xff00 || (first == 0x2001 && second == 0x0db8) {
        return true;
    }
    if segments[..6].iter().all(|segment| *segment == 0) {
        let [high, low] = segments[6].to_be_bytes();
        let [high2, low2] = segments[7].to_be_bytes();
        return is_forbidden_v4(Ipv4Addr::new(high, low, high2, low2));
    }
    false
}

/// Replace fixture email addresses, phone numbers, and long tokens.
///
/// The replacement is heuristic: placeholders stand in for email, phone,
/// and token shaped values. Fixtures still need review.
#[must_use]
pub fn scrub_fixture(body: &str) -> String {
    let no_emails = replace_emails(body);
    let no_phones = replace_phones(&no_emails);
    replace_tokens(&no_phones)
}

fn replace_emails(input: &str) -> String {
    let mut out = String::with_capacity(input.len());
    let mut candidate = String::new();
    for ch in input.chars() {
        if ch.is_ascii_alphanumeric() || matches!(ch, '.' | '_' | '%' | '+' | '-' | '@') {
            candidate.push(ch);
        } else {
            flush_email(&mut candidate, &mut out);
            out.push(ch);
        }
    }
    flush_email(&mut candidate, &mut out);
    out
}

fn flush_email(candidate: &mut String, out: &mut String) {
    if let Some((local, domain)) = candidate.split_once('@')
        && !local.is_empty()
        && domain.contains('.')
        && !domain.starts_with('.')
        && !domain.ends_with('.')
    {
        out.push_str(EMAIL_PLACEHOLDER);
        candidate.clear();
        return;
    }
    out.push_str(candidate);
    candidate.clear();
}

fn replace_phones(input: &str) -> String {
    let chars: Vec<char> = input.chars().collect();
    let mut out = String::with_capacity(input.len());
    let mut index = 0;
    while index < chars.len() {
        let ch = chars[index];
        let preceded_by_token = index > 0 && is_token_char(chars[index - 1]);
        let starts_phone = ch.is_ascii_digit()
            || (matches!(ch, '+' | '(') && chars.get(index + 1).is_some_and(char::is_ascii_digit));
        if !starts_phone || preceded_by_token {
            out.push(ch);
            index += 1;
            continue;
        }

        let mut end = index;
        let mut digits = 0;
        while end < chars.len() {
            let current = chars[end];
            if current.is_ascii_digit() {
                digits += 1;
                end += 1;
                continue;
            }
            if matches!(current, '+' | '(' | ')' | '-' | '.' | ' ') {
                let mut lookahead = end;
                while lookahead < chars.len()
                    && matches!(chars[lookahead], '+' | '(' | ')' | '-' | '.' | ' ')
                {
                    lookahead += 1;
                }
                if lookahead < chars.len() && chars[lookahead].is_ascii_digit() {
                    end = lookahead;
                    continue;
                }
            }
            break;
        }

        let followed_by_token = chars.get(end).is_some_and(|next| is_token_char(*next));
        if (MIN_PHONE_DIGITS..=MAX_PHONE_DIGITS).contains(&digits) && !followed_by_token {
            out.push_str(PHONE_PLACEHOLDER);
        } else {
            out.extend(&chars[index..end]);
        }
        index = end;
    }
    out
}

fn is_token_char(ch: char) -> bool {
    ch.is_ascii_alphanumeric() || matches!(ch, '_' | '-' | '%' | '@')
}

fn replace_tokens(input: &str) -> String {
    let mut out = String::with_capacity(input.len());
    let mut candidate = String::new();
    for ch in input.chars() {
        if ch.is_ascii_alphanumeric() || matches!(ch, '_' | '-' | '.') {
            candidate.push(ch);
        } else {
            flush_token(&mut candidate, &mut out);
            out.push(ch);
        }
    }
    flush_token(&mut candidate, &mut out);
    out
}

fn flush_token(candidate: &mut String, out: &mut String) {
    if candidate.len() >= TOKEN_THRESHOLD {
        out.push_str(TOKEN_PLACEHOLDER);
    } else {
        out.push_str(candidate);
    }
    candidate.clear();
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn secret_formatting_never_leaks_value() {
        let value = "synthetic-secret-value-123";
        let secret = Secret::from(value);
        assert_eq!(format!("{secret:?}"), REDACTED);
        assert_eq!(format!("{secret}"), REDACTED);
        assert!(!format!("{secret:?}").contains(value));
        assert!(!format!("{secret}").contains(value));

        let owned = Secret::from(value.to_owned());
        assert_eq!(owned, secret);
        assert_eq!(owned.as_str(), value);
    }

    #[test]
    fn redact_url_masks_userinfo_query_and_fragment() {
        let secret = "abcdefghijklmnopqrstuvwxyz1234";
        let url = format!("https://user:{secret}@example.test/jobs?token={secret}#frag-{secret}");
        let safe = redact_url(&url);
        assert_eq!(
            safe,
            "https://[redacted]@example.test/jobs?[query-redacted]"
        );
        assert!(!safe.contains(secret));
        assert!(!safe.contains("user"));
        assert!(!safe.contains("frag"));
    }

    #[test]
    fn redact_url_keeps_queryless_url_intact() {
        assert_eq!(
            redact_url("https://example.test/jobs/123"),
            "https://example.test/jobs/123"
        );
    }

    #[test]
    fn redact_text_masks_token_and_query() {
        let secret = "abcdefghijklmnopqrstuvwxyz1234";
        let safe = redact_text(&format!("challenge?token={secret} please verify"));
        assert!(!safe.contains(secret));
        assert_ne!(safe.matches(QUERY_REDACTED).count(), 0);
        assert_ne!(safe.matches("verify").count(), 0);
    }

    #[test]
    fn redact_text_masks_standalone_long_identifier() {
        let secret = "abcdefghijklmnopqrstuvwxyz1234";
        let safe = redact_text(&format!("session {secret} ended"));
        assert!(!safe.contains(secret));
        assert_ne!(safe.matches(REDACTED).count(), 0);
    }

    #[test]
    fn blocks_literal_private_ipv4_targets() {
        let literals = [
            "10.1.2.3",
            "172.16.0.1",
            "172.31.255.254",
            "192.168.1.1",
            "127.0.0.1",
            "169.254.169.254",
            "0.0.0.0",
            "100.64.0.1",
            "192.0.0.1",
            "192.0.2.1",
            "198.18.0.1",
            "198.51.100.1",
            "203.0.113.1",
            "224.0.0.1",
            "240.0.0.1",
            "255.255.255.255",
        ];
        for literal in literals {
            let url = format!("http://{literal}/jobs");
            let error = validate_target(&url).unwrap_err();
            assert!(
                error.contains("blocked"),
                "unexpected error for {literal}: {error}"
            );
        }
    }

    #[test]
    fn blocks_literal_private_ipv6_targets() {
        let literals = [
            "::1",
            "fc00::1",
            "fd00::1",
            "fe80::1",
            "::",
            "::ffff:127.0.0.1",
            "::ffff:10.0.0.1",
            "::7f00:1",
            "2001:db8::1",
            "ff02::1",
        ];
        for literal in literals {
            let url = format!("http://[{literal}]:8080/jobs");
            let error = validate_target(&url).unwrap_err();
            assert!(
                error.contains("blocked"),
                "unexpected error for {literal}: {error}"
            );
        }
    }

    #[test]
    fn accepts_public_literals_and_returns_addresses() {
        let v4 = validate_target("https://user:pass@8.8.8.8:8443/jobs?token=abc")
            .expect("public literal accepted");
        assert_eq!(v4, vec![IpAddr::V4(Ipv4Addr::new(8, 8, 8, 8))]);

        let v6 = validate_target("http://[2606:4700::1111]:8080/jobs")
            .expect("public IPv6 literal accepted");
        assert_eq!(
            v6,
            vec![IpAddr::V6(
                "2606:4700::1111".parse().expect("valid literal")
            )]
        );
    }

    #[test]
    fn rejects_malformed_targets() {
        validate_target("ftp://example.test/jobs").unwrap_err();
        validate_target("example.test/jobs").unwrap_err();
        validate_target("https://user@/jobs").unwrap_err();
        validate_target("https://example.test:notaport/jobs").unwrap_err();
        validate_target("https://[::1/jobs").unwrap_err();
    }

    #[test]
    fn resolve_public_host_rejects_names_with_only_private_results() {
        let error = resolve_public_host("localhost").unwrap_err();
        assert!(error.contains("blocked"));
        let error = resolve_public_host("127.0.0.1").unwrap_err();
        assert!(error.contains("blocked"));
    }

    #[test]
    #[ignore = "requires DNS resolution"]
    fn resolve_public_host_accepts_public_name() {
        let addresses = resolve_public_host("example.com").expect("public hostname resolves");
        assert_ne!(addresses, [] as [IpAddr; 0]);
        assert!(addresses.iter().all(|address| !is_forbidden(*address)));
    }

    #[test]
    #[ignore = "requires DNS resolution"]
    fn resolves_public_hostname() {
        let addresses =
            validate_target("https://example.com/jobs").expect("public hostname resolves");
        assert_ne!(addresses, [] as [IpAddr; 0]);
        assert!(addresses.iter().all(|address| !is_forbidden(*address)));
    }

    #[test]
    fn scrub_fixture_replaces_personal_data() {
        let secret = "SYNTHETIC-SECRET-abcdefghijklmnopqrstuvwxyz1234";
        let body = format!("Contact jane.doe@example.test at +1 (555) 010-0199 with {secret}.");
        let scrubbed = scrub_fixture(&body);
        assert!(!scrubbed.contains("jane.doe@example.test"));
        assert!(!scrubbed.contains("555"));
        assert!(!scrubbed.contains(secret));
        assert_ne!(scrubbed.matches(EMAIL_PLACEHOLDER).count(), 0);
        assert_ne!(scrubbed.matches(PHONE_PLACEHOLDER).count(), 0);
        assert_ne!(scrubbed.matches(TOKEN_PLACEHOLDER).count(), 0);
    }

    #[test]
    fn scrub_fixture_keeps_ordinary_text() {
        let body = "Engineer role in Bengaluru";
        assert_eq!(scrub_fixture(body), body);
    }
}
