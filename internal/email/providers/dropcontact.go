package providers

import (
	"context"
	"net/http"
)

// Dropcontact is the Dropcontact.io provider. Free tier: none.
// Paid: 100 credits/month starting. Endpoints: /v1/enrich/all.
// Ponytail: full implementation deferred to a follow-up PR; this
// stub provides a working constructor and Available check.
type Dropcontact struct {
	apiKey     string
	httpClient *http.Client
}

// NewDropcontact returns a Dropcontact provider with the given API key.
func NewDropcontact(apiKey string, httpClient *http.Client) *Dropcontact {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Dropcontact{
		apiKey:     apiKey,
		httpClient: httpClient,
	}
}

// Name returns the provider's short identifier.
func (d *Dropcontact) Name() string { return "dropcontact" }

// Available reports whether the provider has credentials.
func (d *Dropcontact) Available() error {
	if d.apiKey == "" {
		return ErrUnavailable
	}
	return nil
}

// DomainSearch is a stub. Full implementation deferred.
func (d *Dropcontact) DomainSearch(_ context.Context, _ string) ([]Email, error) {
	return nil, ErrNotFound
}

// EmailFinder is a stub. Full implementation deferred.
func (d *Dropcontact) EmailFinder(_ context.Context, _ string, _ string, _ string) (string, error) {
	return "", ErrNotFound
}

// Verify is a stub. Full implementation deferred.
func (d *Dropcontact) Verify(_ context.Context, _ string) (bool, string, error) {
	return false, "", ErrNotFound
}
