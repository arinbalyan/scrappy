// Package email provides extraction, normalization, MX verification,
// and company-page enrichment for email addresses embedded in job postings.
package email

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/util"
)

// ─── Constants ─────────────────────────────────────────────────────────────────

const RoleEmailSource = "description"

// ─── Patterns ─────────────────────────────────────────────────────────────────

var (
	// mailRegex matches standard email-like strings.
	mailRegex = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+(?:---[a-zA-Z0-9._%+\-]+)*@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)

	// obfuscatedRegex matches common obfuscation patterns used in job postings:
	//   name [at] domain [dot] com
	//   name (at) domain (dot) com
	//   name AT domain DOT com
	//   name(at)domain.com
	obfuscatedRegex = regexp.MustCompile(`(?i)([a-zA-Z0-9._%+\-]+)\s*(?:\[at\]|\(at\)|\bat\b)\s*([a-zA-Z0-9.\-]+)\s*(?:\[dot\]|\(dot\)|\bdot\b)\s*([a-zA-Z]{2,})`)

	// htmlMailtoRegex extracts mailto: hrefs in HTML.
	htmlMailtoRegex = regexp.MustCompile(`href\s*=\s*"mailto:([^"]+)"`)
)

// ─── Internal helpers ─────────────────────────────────────────────────────────

// blockedDomains are always discarded regardless of MX records.
var blockedDomains = map[string]bool{
	// Disposable / throwaway mail providers.
	"guerrillamail.com": true, "mailinator.com": true, "trashmail.com": true,
	"tempmail.com": true, "10minutemail.com": true, "yopmail.com": true,
	"sharklasers.com": true, "throwam.com": true, "fakeinbox.com": true,
	"maildrop.cc": true, "getnada.com": true, "burnermail.io": true,
	"emailondeck.com": true, "mohmal.com": true, "temp-mail.org": true,
	"tempemail.co": true, "guerrillamail.org": true, "guerrillamail.net": true,
	"grr.la": true, "mailmetrash.com": true, "throwaway.email": true,
	"dispostable.com": true, "mailexpire.com": true, "spambox.us": true,
	"mailetc.com": true, "spamgourmet.com": true, "spamhole.com": true,
	"thankyou2010.com": true, "trash2009.com": true, "trashymail.com": true,
	"tyldd.com": true, "uggsrock.com": true, "wegwerfmail.de": true,
	"wegwerfmail.net": true, "wegwerfmail.org": true, "wh4f.org": true,
	"whyspam.me": true, "willselfdestruct.com": true, "winemaven.info": true,
	"wronghead.com": true, "yopmail.fr": true, "yopmail.net": true,
	"z1p.biz": true, "zero-mail.net": true, "zoaxe.com": true,
	"nomail.xl.cx": true, "nospam4.us": true, "nowmymail.com": true,
	// Platform routing addresses — never direct candidate contacts.
	"indeed.com": true, "linkedin.com": true, "glassdoor.com": true,
	"monster.com": true, "ziprecruiter.com": true, "careerbuilder.com": true,
}

// invalidDomainsRegex matches TLDs and patterns that are never valid email domains.
var invalidDomainSuffixes = []string{
	".local", ".arpa", ".invalid", ".test", ".example",
	".onion", ".i2p", ".bitnet", ".uucp",
}

func isBlockedDomain(addr string) bool {
	domain := domainFrom(addr)
	if domain == "" {
		return true
	}
	dl := strings.ToLower(domain)
	if blockedDomains[dl] {
		return true
	}
	// Block IP-address domains (e.g., user@[192.168.1.1]).
	if strings.HasPrefix(dl, "[") || strings.HasPrefix(dl, "192.") ||
		strings.HasPrefix(dl, "10.") || strings.HasPrefix(dl, "172.") ||
		strings.Count(dl, ".") == 1 && regexp.MustCompile(`^\d+\.\d+$`).MatchString(dl) {
		return true
	}
	for _, suffix := range invalidDomainSuffixes {
		if strings.HasSuffix(dl, suffix) {
			return true
		}
	}
	return false
}

// strictlyValidEmail performs RFC 5321-style validation beyond mail.ParseAddress.
// Rejects addresses with consecutive dots, leading dots, empty local parts, etc.
func strictlyValidEmail(addr string) bool {
	// mail.ParseAddress already does basic RFC 5322 validation.
	_, err := mail.ParseAddress(addr)
	if err != nil {
		return false
	}
	parts := strings.SplitN(addr, "@", 2)
	if len(parts) != 2 {
		return false
	}
	local := parts[0]
	domain := parts[1]

	// Reject empty or too-long local parts.
	if local == "" || len(local) > 64 {
		return false
	}
	// Reject consecutive dots in local part.
	if strings.Contains(local, "..") {
		return false
	}
	// Reject leading/trailing dots in local part.
	if strings.HasPrefix(local, ".") || strings.HasSuffix(local, ".") {
		return false
	}
	// Reject comments in local part (RFC 5321 says no, but ParseAddress may allow).
	if strings.Contains(local, "(") || strings.Contains(local, ")") {
		return false
	}
	// Domain must have at least one dot (no bare TLDs like user@com).
	if !strings.Contains(domain, ".") {
		return false
	}
	// Domain part max length.
	if len(domain) > 255 {
		return false
	}
	return true
}

func normalizeAddr(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

// deobfuscate attempts to reconstruct an email from common obfuscation patterns
// (e.g., "name [at] domain [dot] com"). Returns "" if not obfuscated.
func deobfuscate(text string) string {
	matches := obfuscatedRegex.FindStringSubmatch(text)
	if len(matches) < 4 {
		return ""
	}
	// Reconstruct: name@domain.tld
	return strings.ToLower(strings.TrimSpace(matches[1])) + "@" +
		strings.ToLower(strings.TrimSpace(matches[2])) + "." +
		strings.ToLower(strings.TrimSpace(matches[3]))
}

func domainFrom(addr string) string {
	parts := strings.Split(strings.ToLower(addr), "@")
	if len(parts) != 2 {
		return ""
	}
	return parts[1]
}

var rolePrefixes = map[string]bool{
	"info": true, "admin": true, "support": true, "contact": true,
	"sales": true, "hello": true, "careers": true, "press": true,
	"marketing": true, "jobs": true, "hr": true, "recruiting": true,
	"noreply": true, "no-reply": true, "help": true,
	"enquiries": true, "enquiry": true, "billing": true,
	"team": true, "newsletter": true, "subscribe": true,
}

func isRoleAddr(addr string) bool {
	local := strings.Split(addr, "@")[0]
	return rolePrefixes[strings.ToLower(local)]
}

// ─── Core type ────────────────────────────────────────────────────────────────

// Email is a single extracted email address with metadata.
type Email struct {
	Addr   string `json:"addr"`
	Role   bool   `json:"role,omitempty"`
	Source string `json:"source"`
}

// ─── Extraction ───────────────────────────────────────────────────────────────

// Extract scans text for email-like strings, discarding disposables and
// invalid RFC addresses. Source is set to RoleEmailSource (description).
// It also detects common obfuscation patterns like "name [at] domain [dot] com"
// and HTML entity-encoded addresses (e.g. &#64; for @, &#46; for .).
func Extract(text string) []Email {
	// Normalize HTML entities first so regex can match decoded forms.
	clean := normalizeHTMLEntities(text)

	seen := make(map[string]bool)
	var out []Email

	collect := func(addr string) {
		a := normalizeAddr(addr)
		if a == "" || seen[a] {
			return
		}
		seen[a] = true
		if !strictlyValidEmail(a) {
			return
		}
		if isBlockedDomain(a) {
			return
		}
		out = append(out, Email{
			Addr:   a,
			Role:   isRoleAddr(a),
			Source: RoleEmailSource,
		})
	}

	// Standard regex match (now matches entity-decoded text).
	for _, m := range mailRegex.FindAllString(clean, -1) {
		collect(m)
	}

	// Obfuscated pattern detection.
	for _, m := range obfuscatedRegex.FindAllStringSubmatch(clean, -1) {
		if len(m) >= 4 {
			reconstructed := strings.ToLower(strings.TrimSpace(m[1])) + "@" +
				strings.ToLower(strings.TrimSpace(m[2])) + "." +
				strings.ToLower(strings.TrimSpace(m[3]))
			collect(reconstructed)
		}
	}

	return out
}

// normalizeHTMLEntities decodes common HTML entities that may hide email
// addresses in job descriptions, e.g. &#64; -> @, &#46; -> ., etc.
func normalizeHTMLEntities(text string) string {
	s := text
	s = strings.ReplaceAll(s, "&#64;", "@")
	s = strings.ReplaceAll(s, "&#x40;", "@")
	s = strings.ReplaceAll(s, "&#46;", ".")
	s = strings.ReplaceAll(s, "&#x2E;", ".")
	s = strings.ReplaceAll(s, "&#x2e;", ".")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	return s
}

// ExtractFromHTML scans HTML text for email addresses in both standard text
// and mailto:href attributes. Returns a deduplicated list.
func ExtractFromHTML(html string) []Email {
	seen := make(map[string]bool)
	var out []Email

	collect := func(addr string) {
		a := normalizeAddr(addr)
		if a == "" || seen[a] {
			return
		}
		seen[a] = true
		if !strictlyValidEmail(a) {
			return
		}
		if isBlockedDomain(a) {
			return
		}
		out = append(out, Email{
			Addr:   a,
			Role:   isRoleAddr(a),
			Source: "mailto",
		})
	}

	// mailto: href extraction.
	for _, m := range htmlMailtoRegex.FindAllStringSubmatch(html, -1) {
		if len(m) >= 2 {
			// mailto: URLs may have ?subject=... — strip query params.
			raw := strings.SplitN(m[1], "?", 2)[0]
			collect(raw)
		}
	}

	// Also run standard Extact on the HTML body for free-text emails.
	for _, e := range Extract(html) {
		a := normalizeAddr(e.Addr)
		if !seen[a] {
			seen[a] = true
			e.Source = "description"
			out = append(out, e)
		}
	}

	return out
}

// ─── Dedup and filter ─────────────────────────────────────────────────────────

// Deduplicate removes exact-address duplicates.
func Deduplicate(emails []Email) []Email {
	seen := make(map[string]bool)
	var out []Email
	for _, e := range emails {
		if !seen[e.Addr] {
			seen[e.Addr] = true
			out = append(out, e)
		}
	}
	return out
}

// DomainFrom extracts the hostname from an email address.
func DomainFrom(addr string) string {
	return domainFrom(addr)
}

// IsRole returns true for role-based addresses (info@, admin@, etc.).
func IsRole(addr string) bool {
	return isRoleAddr(addr)
}

// FilterRole removes role-based addresses from the list.
func FilterRole(emails []Email) []Email {
	var out []Email
	for _, e := range emails {
		if !e.Role {
			out = append(out, e)
		}
	}
	return out
}

// ─── MX verification ──────────────────────────────────────────────────────────

// MXVerifier performs DNS MX lookups with configurable timeout.
type MXVerifier struct {
	// Resolver is the DNS resolver used for MX lookups. Defaults to net.DefaultResolver.
	Resolver *net.Resolver

	// Timeout is the per-lookup timeout. Defaults to 10s when zero.
	Timeout time.Duration

	// LookupMX is an optional stub for tests. When set, it is used instead of Resolver.
	// The function receives the domain and returns MX host strings and whether any exist.
	LookupMX func(domain string) (hosts []string, ok bool)
}

// NewMXVerifier returns an MXVerifier with default settings.
func NewMXVerifier() *MXVerifier {
	return &MXVerifier{
		Resolver: net.DefaultResolver,
		Timeout:  10 * time.Second,
	}
}

// Verify checks whether the domain of addr has MX records.
//
// When ctx is nil, a background context with DefaultTimeout is used.
// Nil Resolver with no LookupMX stub returns true (safe mode for tests/offline).
// A LookupMX stub takes precedence over Resolver when set.
func (v *MXVerifier) Verify(ctx context.Context, addr string) bool {
	if v == nil {
		return false
	}
	d := domainFrom(addr)
	if d == "" {
		return false
	}

	// Use test stub when provided.
	if v.LookupMX != nil {
		_, ok := v.LookupMX(d)
		return ok
	}

	// Nil resolver with no stub = safe mode (offline / test environment).
	if v.Resolver == nil {
		return true
	}

	if ctx == nil {
		ctx = context.Background()
	}
	lookupCtx, cancel := context.WithTimeout(ctx, v.Timeout)
	defer cancel()
	mxs, err := v.Resolver.LookupMX(lookupCtx, d)
	return err == nil && len(mxs) > 0
}

// VerifyEmail returns (verified, reason) for diagnostics.
// reason is "mx_ok" / "mx_no_records" / "mx_error:..." / "invalid_domain".
func (v *MXVerifier) VerifyEmail(ctx context.Context, addr string) (verified bool, reason string) {
	if v == nil {
		return false, "nil_verifier"
	}
	d := domainFrom(addr)
	if d == "" {
		return false, "no_domain"
	}
	if isBlockedDomain(addr) {
		return false, "blocked_domain"
	}
	if !strictlyValidEmail(addr) {
		return false, "invalid_format"
	}

	if v.LookupMX != nil {
		_, ok := v.LookupMX(d)
		if ok {
			return true, "mx_ok"
		}
		return false, "mx_no_records"
	}

	if v.Resolver == nil {
		return true, "safe_mode"
	}

	if ctx == nil {
		ctx = context.Background()
	}
	lookupCtx, cancel := context.WithTimeout(ctx, v.Timeout)
	defer cancel()
	mxs, err := v.Resolver.LookupMX(lookupCtx, d)
	if err != nil {
		return false, "mx_error:" + err.Error()
	}
	if len(mxs) > 0 {
		return true, "mx_ok"
	}
	return false, "mx_no_records"
}

// ─── Company-page enrichment ──────────────────────────────────────────────────

// CompanyPageEnricher fetches a company page and extracts emails with bounded
// concurrency via a semaphore.
type CompanyPageEnricher struct {
	HTTPClient  *http.Client
	Concurrency int
	Verifier    *MXVerifier
	sem         chan struct{}
	PauseMs     int
}

// NewCompanyPageEnricher returns an enricher with the given concurrency and pause.
func NewCompanyPageEnricher(client *http.Client, concurrency int, pauseMs int) *CompanyPageEnricher {
	if concurrency < 1 {
		concurrency = 1
	}
	return &CompanyPageEnricher{
		HTTPClient:  client,
		Concurrency: concurrency,
		Verifier:    NewMXVerifier(),
		sem:         make(chan struct{}, concurrency),
		PauseMs:     pauseMs,
	}
}

// Enrich fetches companyURL and extracts emails from the page, returning those
// that pass MX verification. Source is set to "company_page".
// Non-fatal errors (fetch failure, no MX records) are logged via util.Debug
// and do not prevent partial results from being returned.
func (e *CompanyPageEnricher) Enrich(ctx context.Context, companyURL string) ([]Email, error) {
	if e.HTTPClient == nil || companyURL == "" {
		return nil, nil
	}
	e.sem <- struct{}{}
	defer func() { <-e.sem }()

	if e.PauseMs > 0 {
		if err := util.SleepWithContext(ctx, time.Duration(e.PauseMs)*time.Millisecond); err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, companyURL, nil)
	if err != nil {
		util.Debug("email_enrich_request_err", map[string]any{"url": companyURL, "err": err.Error()})
		return nil, nil // non-fatal
	}
	req.Header.Set("User-Agent", "scrappy/1.0 (+https://github.com/arinbalyan/scrappy)")

	resp, err := e.HTTPClient.Do(req)
	if err != nil {
		util.Debug("email_enrich_fetch_err", map[string]any{"url": companyURL, "err": err.Error()})
		return nil, nil // non-fatal
	}
	defer io.Copy(io.Discard, resp.Body)
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		util.Debug("email_enrich_read_err", map[string]any{"url": companyURL, "err": err.Error()})
		return nil, nil // non-fatal
	}

	candidates := Extract(string(b))
	if len(candidates) == 0 {
		return nil, nil
	}
	return e.filterVerified(ctx, candidates), nil
}

// filterVerified runs MX verification on candidates and keeps only those that pass.
func (e *CompanyPageEnricher) filterVerified(ctx context.Context, candidates []Email) []Email {
	if e.Verifier == nil {
		return candidates
	}
	seen := make(map[string]bool)
	var out []Email
	for _, c := range candidates {
		if seen[c.Addr] {
			continue
		}
		seen[c.Addr] = true
		if e.Verifier.Verify(ctx, c.Addr) {
			out = append(out, c)
		}
	}
	return out
}

// BuildURLFromDomainAndSite constructs a best-guess company URL from a domain.
func BuildURLFromDomainAndSite(domain, site string) string {
	if domain == "" {
		return ""
	}
	return "https://" + domain
}
