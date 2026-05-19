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
)

// ─── Constants ─────────────────────────────────────────────────────────────────

// RoleEmailSource is the Source value for emails found in a job description.
const RoleEmailSource = "description"

// ─── Patterns ─────────────────────────────────────────────────────────────────

var (
	// mailRegex matches a---b@example.com style addresses (JobSpy pattern) and plain RFC addresses.
	mailRegex = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+(?:---[a-zA-Z0-9._%+\-]+)*@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)

	rolePrefixes = regexp.MustCompile(`(?i)^(info|admin|support|contact|sales|hello|careers|press|marketing|jobs|hr|recruiting|noreply|no-reply|help|enquiries|enquiry|billing)@`)

	disposableDomains = map[string]bool{
		"guerrillamail.com": true, "mailinator.com": true, "trashmail.com": true,
		"tempmail.com": true, "10minutemail.com": true, "yopmail.com": true,
		"sharklasers.com": true, "throwam.com": true, "fakeinbox.com": true,
	}
)

// ─── Helpers ──────────────────────────────────────────────────────────────────

func isDisposableDomain(addr string) bool {
	parts := strings.Split(addr, "@")
	if len(parts) != 2 {
		return true
	}
	return disposableDomains[strings.ToLower(parts[1])]
}

func isValidEmail(addr string) bool {
	_, err := mail.ParseAddress(addr)
	return err == nil
}

func normalizeAddr(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// domainFrom returns the hostname part of addr.
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

// ─── Core types ───────────────────────────────────────────────────────────────

// Email is the canonical email record embedded in a JobPost.
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
			Role:   rolePrefixes.MatchString(addr),
			Source: RoleEmailSource,
		})
	}
	return out
}

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

// IsRole returns true for role-based addresses.
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

// EmailMXVerifier wraps MX lookups so tests can supply a stub.
type EmailMXVerifier struct {
	LookupMX func(domain string) (mxEntries []string, gotMX bool)
}

// NewMXVerifier returns a verifier wired to live MX lookups.
func NewMXVerifier() *EmailMXVerifier {
	return &EmailMXVerifier{LookupMX: lookupMXLive}
}

func lookupMXLive(domain string) (mxEntries []string, gotMX bool) {
	mxs, err := net.LookupMX(domain)
	if err != nil || len(mxs) == 0 {
		return nil, false
	}
	out := make([]string, len(mxs))
	for i, m := range mxs {
		out[i] = m.Host
	}
	return out, true
}

// VerifyWithMX returns true only when the domain has MX records.
// When LookupMX is nil (not wired) it returns true to avoid false drops.
func (v *EmailMXVerifier) VerifyWithMX(e Email) bool {
	if v.LookupMX == nil {
		return true
	}
	d := domainFrom(e.Addr)
	if d == "" {
		return false
	}
	_, ok := v.LookupMX(d)
	return ok
}

// ─── Company-page enrichment ──────────────────────────────────────────────────

// CompanyPageEnricher fetches a company page and extracts emails with bounded
// concurrency via a semaphore.
type CompanyPageEnricher struct {
	HTTPClient  *http.Client
	Concurrency int
	sem         chan struct{}
	PauseMs     int
}

func NewCompanyPageEnricher(client *http.Client, concurrency int, pauseMs int) *CompanyPageEnricher {
	if concurrency < 1 {
		concurrency = 1
	}
	return &CompanyPageEnricher{
		HTTPClient:  client,
		Concurrency: concurrency,
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
		time.Sleep(time.Duration(e.PauseMs) * time.Millisecond)
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
	return mxVerify(Extract(string(b))), nil
}

// mxVerify re-runs MX checks only after EnrichPageMX filtering, which removes duplicates
// and keeps company_page-sourced addresses only.
func mxVerify(candidates []Email) []Email {
	if globalVerifier == nil {
		return candidates
	}
	var out []Email
	seen := make(map[string]bool)
	for _, c := range candidates {
		if seen[c.Addr] {
			continue
		}
		seen[c.Addr] = true
		if globalVerifier.VerifyWithMX(c) {
			out = append(out, c)
		}
	}
	return out
}

// ─── Wiring ───────────────────────────────────────────────────────────────────

var globalVerifier *EmailMXVerifier

// EnrichEmailStage wires the global MX verifier used by mxcEnrichPage.
func EnrichEmailStage(verifier *EmailMXVerifier) {
	globalVerifier = verifier
}

// BuildURLFromDomainAndSite constructs a company URL from a domain and site.
func BuildURLFromDomainAndSite(domain, site string) string {
	if domain == "" {
		return ""
	}
	switch strings.ToLower(site) {
	case "wellfound":
		return "https://wellfound.com/company/" + domain
	default:
		return "https://" + domain
	}
}
