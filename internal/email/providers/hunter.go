package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Hunter is the Hunter.io email-finder provider.
// Free tier: 25 searches/month. Paid: $34/mo Starter, $99/mo Growth.
// Endpoints: /v2/domain-search, /v2/email-finder, /v2/email-verifier.
//
// Ponytail: one struct, three methods, stdlib only.
type Hunter struct {
	key        string
	httpClient *http.Client
	monthlyCap int // requests per month; 25 for free tier
}

// NewHunter returns a Hunter provider with the given API key.
func NewHunter(key string, httpClient *http.Client) *Hunter {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &Hunter{
		key:        key,
		httpClient: httpClient,
		monthlyCap: 25,
	}
}

// Name returns the provider's short identifier.
func (h *Hunter) Name() string { return "hunter" }

// Available reports whether the provider has credentials.
func (h *Hunter) Available() error {
	if h.key == "" {
		return ErrUnavailable
	}
	return nil
}

// DomainSearch returns all known emails for the given domain via
// the /v2/domain-search endpoint.
func (h *Hunter) DomainSearch(ctx context.Context, domain string) ([]Email, error) {
	if err := h.Available(); err != nil {
		return nil, err
	}
	u := fmt.Sprintf("https://api.hunter.io/v2/domain-search?domain=%s&limit=10&api_key=%s",
		url.QueryEscape(domain), url.QueryEscape(h.key))
	body, err := h.get(ctx, u)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data struct {
			Domain     string `json:"domain"`
			Pattern    string `json:"pattern"`
			AcceptAll  bool   `json:"accept_all"`
			Emails     []struct {
				Value      string `json:"value"`
				Type       string `json:"type"`
				Confidence int    `json:"confidence"`
				FirstName  string `json:"first_name"`
				LastName   string `json:"last_name"`
				Position   string `json:"position"`
				Department string `json:"department"`
				Seniority  string `json:"seniority"`
				LinkedIn   string `json:"linkedin"`
				Twitter    string `json:"twitter"`
			} `json:"emails"`
		} `json:"data"`
		Errors []struct {
			Code    int    `json:"code"`
			Details string `json:"details"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("hunter_decode: %w", err)
	}
	if len(resp.Errors) > 0 {
		return nil, fmt.Errorf("hunter_api_error: %s (code %d)", resp.Errors[0].Details, resp.Errors[0].Code)
	}

	out := make([]Email, 0, len(resp.Data.Emails))
	for _, e := range resp.Data.Emails {
		out = append(out, Email{
			Address:    e.Value,
			FirstName:  e.FirstName,
			LastName:   e.LastName,
			Position:   e.Position,
			Department: e.Department,
			Seniority:  e.Seniority,
			LinkedIn:   e.LinkedIn,
			Twitter:    e.Twitter,
			Source:     h.Name(),
			Type:       e.Type,
			Confidence: float64(e.Confidence),
		})
	}
	return out, nil
}

// EmailFinder returns the most likely email for the given triple
// via the /v2/email-finder endpoint.
func (h *Hunter) EmailFinder(ctx context.Context, first, last, domain string) (string, error) {
	if err := h.Available(); err != nil {
		return "", err
	}
	if first == "" || last == "" || domain == "" {
		return "", ErrNotFound
	}
	u := fmt.Sprintf("https://api.hunter.io/v2/email-finder?domain=%s&first_name=%s&last_name=%s&api_key=%s",
		url.QueryEscape(domain), url.QueryEscape(first), url.QueryEscape(last), url.QueryEscape(h.key))
	body, err := h.get(ctx, u)
	if err != nil {
		return "", err
	}

	var resp struct {
		Data struct {
			Email      string `json:"email"`
			Score      int    `json:"score"`
			Position   string `json:"position"`
			Department string `json:"department"`
		} `json:"data"`
		Errors []struct {
			Code    int    `json:"code"`
			Details string `json:"details"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("hunter_finder_decode: %w", err)
	}
	if len(resp.Errors) > 0 {
		// Code 400 / 404 means no result; not an error to bubble up.
		if resp.Errors[0].Code == 400 || resp.Errors[0].Code == 404 {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("hunter_finder_error: %s", resp.Errors[0].Details)
	}
	return resp.Data.Email, nil
}

// Verify checks deliverability via /v2/email-verifier.
func (h *Hunter) Verify(ctx context.Context, addr string) (bool, string, error) {
	if err := h.Available(); err != nil {
		return false, "", err
	}
	if addr == "" {
		return false, "empty_address", nil
	}
	u := fmt.Sprintf("https://api.hunter.io/v2/email-verifier?email=%s&api_key=%s",
		url.QueryEscape(addr), url.QueryEscape(h.key))
	body, err := h.get(ctx, u)
	if err != nil {
		return false, "", err
	}

	var resp struct {
		Data struct {
			Status     string `json:"status"`     // "valid" | "invalid" | "accept_all" | "unknown" | "disposable" | "webmail"
			Result     string `json:"result"`     // "deliverable" | "undeliverable" | "risky" | "unknown"
			Score      int    `json:"score"`      // 0-100
			AcceptAll  bool   `json:"accept_all"`
		} `json:"data"`
		Errors []struct {
			Code    int    `json:"code"`
			Details string `json:"details"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return false, "", fmt.Errorf("hunter_verify_decode: %w", err)
	}
	if len(resp.Errors) > 0 {
		return false, "", fmt.Errorf("hunter_verify_error: %s", resp.Errors[0].Details)
	}
	return resp.Data.Result == "deliverable", resp.Data.Status, nil
}

// get performs a GET and returns the response body. Honours ctx.
func (h *Hunter) get(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "scrappy/1.0 (+https://github.com/arinbalyan/scrappy)")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, ErrRateLimited
	}
	if resp.StatusCode == http.StatusPaymentRequired {
		// Free tier quota exceeded.
		return nil, ErrRateLimited
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("hunter_http_%d", resp.StatusCode)
	}

	limited := io.LimitReader(resp.Body, 1<<20)
	buf := make([]byte, 0, 16<<10)
	tmp := make([]byte, 4096)
	for {
		n, err := limited.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			break
		}
	}
	return buf, nil
}
