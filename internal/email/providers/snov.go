package providers

import (
	"context"
	"net/http"
)

// Snov is the Snov.io provider. Free tier: 50 credits/month.
// Endpoints: /v2/domain-search, /v2/get-emails-from-name, /v2/email-verifier.
// Ponytail: full implementation deferred to a follow-up PR; this
// stub provides a working constructor and Available check so the
// registry and env-loading paths exercise it.
type Snov struct {
	clientID    string
	clientSecret string
	httpClient  *http.Client
}

// NewSnov returns a Snov provider with the given credentials.
func NewSnov(clientID, clientSecret string, httpClient *http.Client) *Snov {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Snov{
		clientID:    clientID,
		clientSecret: clientSecret,
		httpClient:  httpClient,
	}
}

// Name returns the provider's short identifier.
func (s *Snov) Name() string { return "snov" }

// Available reports whether the provider has credentials.
func (s *Snov) Available() error {
	if s.clientID == "" || s.clientSecret == "" {
		return ErrUnavailable
	}
	return nil
}

// DomainSearch is a stub. Full implementation deferred.
func (s *Snov) DomainSearch(_ context.Context, _ string) ([]Email, error) {
	return nil, ErrNotFound
}

// EmailFinder is a stub. Full implementation deferred.
func (s *Snov) EmailFinder(_ context.Context, _ string, _ string, _ string) (string, error) {
	return "", ErrNotFound
}

// Verify is a stub. Full implementation deferred.
func (s *Snov) Verify(_ context.Context, _ string) (bool, string, error) {
	return false, "", ErrNotFound
}
