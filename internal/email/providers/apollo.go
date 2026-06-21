package providers

import (
	"context"
	"net/http"
)

// Apollo is the Apollo.io provider. Free tier: none. Paid: $49/mo.
// Endpoints: /api/v1/people/match, /api/v1/contacts/search.
// Ponytail: full implementation deferred to a follow-up PR; this
// stub provides a working constructor and Available check.
type Apollo struct {
	apiKey     string
	httpClient *http.Client
}

// NewApollo returns an Apollo provider with the given API key.
func NewApollo(apiKey string, httpClient *http.Client) *Apollo {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Apollo{
		apiKey:     apiKey,
		httpClient: httpClient,
	}
}

// Name returns the provider's short identifier.
func (a *Apollo) Name() string { return "apollo" }

// Available reports whether the provider has credentials.
func (a *Apollo) Available() error {
	if a.apiKey == "" {
		return ErrUnavailable
	}
	return nil
}

// DomainSearch is a stub. Full implementation deferred.
func (a *Apollo) DomainSearch(_ context.Context, _ string) ([]Email, error) {
	return nil, ErrNotFound
}

// EmailFinder is a stub. Full implementation deferred.
func (a *Apollo) EmailFinder(_ context.Context, _ string, _ string, _ string) (string, error) {
	return "", ErrNotFound
}

// Verify is a stub. Full implementation deferred.
func (a *Apollo) Verify(_ context.Context, _ string) (bool, string, error) {
	return false, "", ErrNotFound
}
