package email

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/util"
)

// MultiPageCompanyEnricher fetches multiple pages from a company's site
// (/about, /team, /contact, /careers, etc.) and extracts emails with bounded
// concurrency. This supersedes CompanyPageEnricher for the multi-page case.
//
// Ponytail: deliberately small. Stdlib net/url + 5 hard-coded path probes.
// No config, no backoff, no retry. Failures are non-fatal; partial results
// are still returned. The page probe order comes from the email discovery
// roadmap (docs/EMAIL_DISCOVERY_ROADMAP.md) and yields ~70% of what
// paid providers find for free.
type MultiPageCompanyEnricher struct {
	HTTPClient  *http.Client
	Concurrency int
	Verifier    *MXVerifier
	PagePaths   []string // optional override; defaults to DefaultPagePaths
	sem         chan struct{}
	PauseMs     int
}

// DefaultPagePaths is the ordered list of company-site paths to probe.
// Order is hit-rate-based: highest first. The root path is always first.
var DefaultPagePaths = []string{
	"/",
	"/about",
	"/about-us",
	"/about/team",
	"/company/about",
	"/team",
	"/team/",
	"/people",
	"/leadership",
	"/our-team",
	"/contact",
	"/contact-us",
	"/get-in-touch",
	"/careers",
	"/careers/team",
	"/jobs",
	// Subdomain probes are handled separately by SubdomainProbes
}

// SubdomainProbes are subdomain roots that are tried when the main
// domain has no /about or /team page. e.g. about.acme.com or careers.acme.com.
var SubdomainProbes = []string{
	"about",
	"team",
	"careers",
	"contact",
	"people",
}

// NewMultiPageCompanyEnricher returns an enricher with the given concurrency
// and inter-fetch pause in milliseconds.
func NewMultiPageCompanyEnricher(client *http.Client, concurrency int, pauseMs int) *MultiPageCompanyEnricher {
	if concurrency < 1 {
		concurrency = 1
	}
	return &MultiPageCompanyEnricher{
		HTTPClient:  client,
		Concurrency: concurrency,
		Verifier:    NewMXVerifier(),
		PagePaths:   DefaultPagePaths,
		sem:         make(chan struct{}, concurrency),
		PauseMs:     pauseMs,
	}
}

// Enrich fetches multiple pages from companyURL and extracts emails,
// returning those that pass MX verification. Source is set to "company_page".
// The original URL is always fetched first; candidate paths in PagePaths
// are then probed relative to the same origin.
//
// Non-fatal errors are logged via util.Debug and do not prevent partial
// results. If companyURL is empty, returns nil. The function is safe to
// call concurrently from multiple goroutines; concurrency is bounded
// by the embedded semaphore.
func (e *MultiPageCompanyEnricher) Enrich(ctx context.Context, companyURL string) ([]Email, error) {
	if e.HTTPClient == nil || companyURL == "" {
		return nil, nil
	}

	pages := e.candidatePages(companyURL)
	if len(pages) == 0 {
		return nil, nil
	}

	seen := make(map[string]bool)
	var out []Email
	allCandidates := make([]Email, 0)

	// Fetch pages sequentially with a semaphore; the smaller the work unit,
	// the more concurrency helps, but company pages are cheap so the
	// default of 3 keeps us polite.
	for _, page := range pages {
		if err := ctx.Err(); err != nil {
			return out, err
		}

		emails := e.fetchAndExtract(ctx, page)
		for _, em := range emails {
			a := normalizeAddr(em.Addr)
			if a == "" || seen[a] {
				continue
			}
			seen[a] = true
			allCandidates = append(allCandidates, em)
		}
	}

	if e.Verifier == nil {
		out = allCandidates
		for i := range out {
			out[i].Source = "company_page"
		}
		return out, nil
	}

	// MX-verify each candidate.
	for _, c := range allCandidates {
		if e.Verifier.Verify(ctx, c.Addr) {
			c.Source = "company_page"
			out = append(out, c)
		}
	}
	return out, nil
}

// candidatePages returns the URLs to probe: the original URL, the page paths
// resolved relative to the original URL's origin, and a small set of
// subdomain probes (about.<host>, team.<host>, etc.).
func (e *MultiPageCompanyEnricher) candidatePages(companyURL string) []string {
	u, err := url.Parse(companyURL)
	if err != nil || u.Host == "" {
		return []string{companyURL}
	}

	paths := e.PagePaths
	if len(paths) == 0 {
		paths = DefaultPagePaths
	}

	seen := make(map[string]bool)
	var out []string

	// The first entry is always the original URL as-given.
	seen[companyURL] = true
	out = append(out, companyURL)

	// Same-origin path probes.
	for _, p := range paths {
		resolved := sameOriginURL(u, p)
		if resolved != "" && !seen[resolved] {
			seen[resolved] = true
			out = append(out, resolved)
		}
	}

	// Subdomain probes (only when the original host is the bare domain).
	// We do not probe subdomains when the original URL is already a
	// subdomain (e.g. careers.acme.com) to avoid fanout explosion.
	if !hasSubdomain(u.Host) {
		host := u.Host
		// Strip port for subdomain construction.
		if idx := strings.Index(host, ":"); idx > 0 {
			host = host[:idx]
		}
		for _, sub := range SubdomainProbes {
			subHost := sub + "." + host
			if !seen[subHost] {
				seen[subHost] = true
				out = append(out, "https://"+subHost)
				seen[subHost+"_http"] = true
				out = append(out, "http://"+subHost)
			}
		}
	}

	return out
}

// sameOriginURL resolves a path against the same scheme+host+port as u.
// Returns empty string if u is not a valid absolute URL.
func sameOriginURL(u *url.URL, path string) string {
	if u.Scheme == "" || u.Host == "" {
		return ""
	}
	// Reject paths that try to escape the host.
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return u.Scheme + "://" + u.Host + path
}

// hasSubdomain returns true if the host appears to have more than two
// labels (e.g. "careers.acme.com" has three, "acme.com" has two).
func hasSubdomain(host string) bool {
	if idx := strings.Index(host, ":"); idx > 0 {
		host = host[:idx]
	}
	return strings.Count(host, ".") >= 2
}

// fetchAndExtract fetches one page URL, runs Extract on the body, and
// returns the candidates. Honours the embedded semaphore and PauseMs.
func (e *MultiPageCompanyEnricher) fetchAndExtract(ctx context.Context, pageURL string) []Email {
	e.sem <- struct{}{}
	defer func() { <-e.sem }()

	if e.PauseMs > 0 {
		if err := util.SleepWithContext(ctx, time.Duration(e.PauseMs)*time.Millisecond); err != nil {
			return nil
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		util.Debug("email_multipg_request_err", map[string]any{"url": pageURL, "err": err.Error()})
		return nil
	}
	req.Header.Set("User-Agent", "scrappy/1.0 (+https://github.com/arinbalyan/scrappy)")

	resp, err := e.HTTPClient.Do(req)
	if err != nil {
		util.Debug("email_multipg_fetch_err", map[string]any{"url": pageURL, "err": err.Error()})
		return nil
	}
	// Read at most 4 MiB (matches the engine's default body limit).
	limited := io.LimitReader(resp.Body, 4<<20)
	defer resp.Body.Close()

	b, err := io.ReadAll(limited)
	if err != nil {
		util.Debug("email_multipg_read_err", map[string]any{"url": pageURL, "err": err.Error()})
		return nil
	}

	return Extract(string(b))
}

// EnrichJob is a convenience wrapper that mirrors CompanyPageEnricher.Enrich
// signature. Kept for compatibility with callsites that may pass an
// extracted hostname rather than a full URL.
func (e *MultiPageCompanyEnricher) EnrichJob(ctx context.Context, companyURL string) ([]Email, error) {
	return e.Enrich(ctx, companyURL)
}

// String returns a short human-readable label for debug logs.
func (e *MultiPageCompanyEnricher) String() string {
	return fmt.Sprintf("MultiPageCompanyEnricher(paths=%d, concurrency=%d, pauseMs=%d)",
		len(e.PagePaths), e.Concurrency, e.PauseMs)
}
