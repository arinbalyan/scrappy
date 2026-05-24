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
	mailRegex = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+(?:---[a-zA-Z0-9._%+\-]+)*@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)
)

// ─── Internal helpers ─────────────────────────────────────────────────────────

func isDisposableDomain(addr string) bool {
	parts := strings.Split(addr, "@")
	if len(parts) != 2 {
		return true
	}
	switch strings.ToLower(parts[1]) {
	case "guerrillamail.com", "mailinator.com", "trashmail.com",
		"tempmail.com", "10minutemail.com", "yopmail.com",
		"sharklasers.com", "throwam.com", "fakeinbox.com",
		"maildrop.cc", "getnada.com", "burnermail.io",
		"emailondeck.com", "mohmal.com", "temp-mail.org":
		return true
	}
	return false
}

func isValidEmail(addr string) bool {
	_, err := mail.ParseAddress(addr)
	return err == nil
}

func normalizeAddr(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func domainFrom(addr string) string {
	parts := strings.Split(strings.ToLower(addr), "@")
	if len(parts) != 2 {
		return ""
	}
	return parts[1]
}

func isRoleAddr(addr string) bool {
	local := strings.Split(addr, "@")[0]
	rolePrefixes := map[string]bool{
		"info": true, "admin": true, "support": true, "contact": true,
		"sales": true, "hello": true, "careers": true, "press": true,
		"marketing": true, "jobs": true, "hr": true, "recruiting": true,
		"noreply": true, "no-reply": true, "help": true,
		"enquiries": true, "enquiry": true, "billing": true,
	}
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
func Extract(text string) []Email {
	matches := mailRegex.FindAllString(text, -1)
	seen := make(map[string]bool)
	var out []Email
	for _, m := range matches {
		addr := normalizeAddr(m)
		if seen[addr] {
			continue
		}
		seen[addr] = true
		if !isValidEmail(addr) {
			continue
		}
		if isDisposableDomain(addr) {
			continue
		}
		out = append(out, Email{
			Addr:   addr,
			Role:   isRoleAddr(addr),
			Source: RoleEmailSource,
		})
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
		if !isRoleAddr(e.Addr) {
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
		return true
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

	// Nil resolver with no stub = safe mode.
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
		return nil, err
	}
	req.Header.Set("User-Agent", "scrappy/1.0 (+https://github.com/arinbalyan/scrappy)")

	resp, err := e.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	defer io.Copy(io.Discard, resp.Body)

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return e.filterVerified(Extract(string(b))), nil
}

// filterVerified runs MX verification on candidates and keeps only those that pass.
func (e *CompanyPageEnricher) filterVerified(candidates []Email) []Email {
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
		if e.Verifier.Verify(context.Background(), c.Addr) {
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
