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

// Tomba is the Tomba.io email-finder provider.
// Free tier: 25 searches/month. Paid: $89/mo for 5000.
// Endpoints: /v1/email-finder, /v1/domain-search, /v1/linkedin, /v1/email-format.
//
// Ponytail: same shape as Hunter; one struct, three methods, stdlib only.
type Tomba struct {
	key        string
	secret     string
	httpClient *http.Client
	monthlyCap int
}

// NewTomba returns a Tomba provider with the given key and secret.
func NewTomba(key, secret string, httpClient *http.Client) *Tomba {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &Tomba{
		key:        key,
		secret:     secret,
		httpClient: httpClient,
		monthlyCap: 25,
	}
}

// Name returns the provider's short identifier.
func (t *Tomba) Name() string { return "tomba" }

// Available reports whether the provider has credentials.
func (t *Tomba) Available() error {
	if t.key == "" || t.secret == "" {
		return ErrUnavailable
	}
	return nil
}

// DomainSearch returns all known emails for the given domain.
func (t *Tomba) DomainSearch(ctx context.Context, domain string) ([]Email, error) {
	if err := t.Available(); err != nil {
		return nil, err
	}
	u := fmt.Sprintf("https://api.tomba.io/v1/domain-search?domain=%s", url.QueryEscape(domain))
	body, err := t.get(ctx, u)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data struct {
			Emails []struct {
				Email      string  `json:"email"`
				FirstName  string  `json:"first_name"`
				LastName   string  `json:"last_name"`
				Position   string  `json:"position"`
				Department string  `json:"department"`
				Seniority  string  `json:"seniority"`
				LinkedIn   string  `json:"linkedin"`
				Twitter    string  `json:"twitter"`
				Confidence float64 `json:"score"`
				Type       string  `json:"type"`
			} `json:"emails"`
		} `json:"data"`
		Error  json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("tomba_decode: %w", err)
	}

	out := make([]Email, 0, len(resp.Data.Emails))
	for _, e := range resp.Data.Emails {
		out = append(out, Email{
			Address:    e.Email,
			FirstName:  e.FirstName,
			LastName:   e.LastName,
			Position:   e.Position,
			Department: e.Department,
			Seniority:  e.Seniority,
			LinkedIn:   e.LinkedIn,
			Twitter:    e.Twitter,
			Source:     t.Name(),
			Type:       e.Type,
			Confidence: e.Confidence,
		})
	}
	return out, nil
}

// EmailFinder returns the most likely email for the given triple.
func (t *Tomba) EmailFinder(ctx context.Context, first, last, domain string) (string, error) {
	if err := t.Available(); err != nil {
		return "", err
	}
	if first == "" || last == "" || domain == "" {
		return "", ErrNotFound
	}
	u := fmt.Sprintf("https://api.tomba.io/v1/email-finder?domain=%s&full_name=%s&first_name=%s&last_name=%s",
		url.QueryEscape(domain), url.QueryEscape(first+" "+last), url.QueryEscape(first), url.QueryEscape(last))
	body, err := t.get(ctx, u)
	if err != nil {
		return "", err
	}

	var resp struct {
		Data struct {
			Email string `json:"email"`
			Score int    `json:"score"`
		} `json:"data"`
		Error json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("tomba_finder_decode: %w", err)
	}
	return resp.Data.Email, nil
}

// Verify is not supported by Tomba's public API; returns ErrNotFound.
func (t *Tomba) Verify(_ context.Context, _ string) (bool, string, error) {
	return false, "", fmt.Errorf("tomba: verify not supported")
}

// get performs a GET with Tomba's required auth headers.
func (t *Tomba) get(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "scrappy/1.0 (+https://github.com/arinbalyan/scrappy)")
	req.Header.Set("X-Tomba-Key", t.key)
	req.Header.Set("X-Tomba-Secret", t.secret)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, ErrRateLimited
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("tomba_http_%d", resp.StatusCode)
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
