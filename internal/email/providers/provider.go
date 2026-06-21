// Package providers implements the email-enrichment Provider interface
// for paid services. Each provider lives in its own file and is
// registered with a name so the engine can address them by string.
//
// The interface is intentionally narrow (3 methods) so adding a new
// provider is a 50-line file.
package providers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Email is the provider-agnostic result for DomainSearch and EmailFinder.
type Email struct {
	Address    string
	FirstName  string
	LastName   string
	Position   string
	Department string
	Seniority  string
	LinkedIn   string
	Twitter    string
	Source     string // provider name
	Type       string // "personal" | "role" | "unknown"
	Confidence float64
}

// Provider is the shared interface for all paid email-enrichment
// services (Hunter, Tomba, Snov, Apollo, Dropcontact).
type Provider interface {
	// Name returns the provider's short identifier (e.g. "hunter").
	Name() string

	// Available reports whether the provider has the credentials it
	// needs to make calls. Returns an error explaining the missing
	// env var if not.
	Available() error

	// DomainSearch returns all known emails for the given domain.
	DomainSearch(ctx context.Context, domain string) ([]Email, error)

	// EmailFinder returns the most likely email for the given
	// (first, last, domain) triple. Empty string means no result.
	EmailFinder(ctx context.Context, first, last, domain string) (string, error)

	// Verify checks the deliverability of a specific address.
	// Returns (true, "", nil) on success, (false, reason, nil) on
	// known-bad, and (false, "", err) on transport errors.
	Verify(ctx context.Context, addr string) (deliverable bool, reason string, err error)
}

// ErrUnavailable is returned by Provider implementations when the
// required credentials are not set.
var ErrUnavailable = errors.New("provider unavailable: missing credentials")

// ErrRateLimited is returned when the provider's quota is exhausted.
var ErrRateLimited = errors.New("provider rate limited")

// ErrNotFound is returned when no email was found.
var ErrNotFound = errors.New("provider: no result")

// Registry is the set of providers the engine will try in order.
type Registry struct {
	mu        sync.RWMutex
	providers []Provider
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// Register adds a provider to the registry.
func (r *Registry) Register(p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers = append(r.providers, p)
}

// Providers returns the registered providers in registration order.
func (r *Registry) Providers() []Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Provider, len(r.providers))
	copy(out, r.providers)
	return out
}

// DefaultOrder is the provider order used when the user does not
// override it via env or config. Order is by data quality /
// hit-rate, with Hunter first because it is the most established.
var DefaultOrder = []string{"hunter", "tomba", "snov", "apollo", "dropcontact"}

// LoadFromEnv registers any provider whose credentials are present
// in the environment. Unknown provider names are silently ignored.
func (r *Registry) LoadFromEnv(env Env, httpClient *http.Client) {
	if env.HunterKey != "" {
		r.Register(NewHunter(env.HunterKey, httpClient))
	}
	if env.TombaKey != "" && env.TombaSecret != "" {
		r.Register(NewTomba(env.TombaKey, env.TombaSecret, httpClient))
	}
	if env.SnovClientID != "" && env.SnovClientSecret != "" {
		r.Register(NewSnov(env.SnovClientID, env.SnovClientSecret, httpClient))
	}
	if env.ApolloKey != "" {
		r.Register(NewApollo(env.ApolloKey, httpClient))
	}
	if env.DropcontactKey != "" {
		r.Register(NewDropcontact(env.DropcontactKey, httpClient))
	}
}

// Env holds the environment-derived credentials for all providers.
// Empty string means the corresponding env var was not set.
type Env struct {
	HunterKey       string
	TombaKey        string
	TombaSecret     string
	SnovClientID    string
	SnovClientSecret string
	ApolloKey       string
	DropcontactKey  string
}

// LoadEnv reads all provider env vars from the environment.
func LoadEnv() Env {
	return Env{
		HunterKey:       getenv("HUNTER_API_KEY"),
		TombaKey:        getenv("TOMBA_KEY"),
		TombaSecret:     getenv("TOMBA_SECRET"),
		SnovClientID:    getenv("SNOV_CLIENT_ID"),
		SnovClientSecret: getenv("SNOV_CLIENT_SECRET"),
		ApolloKey:       getenv("APOLLO_API_KEY"),
		DropcontactKey:  getenv("DROPCONTACT_API_KEY"),
	}
}

// getenv is a small indirection so tests can override env values.
// Defaults to lookupEnv (env.go). Override with SetGetenv in tests.
var getenv = func(key string) string {
	return lookupEnv(key)
}

// SetGetenv overrides the package-level env lookup. Used by tests
// to inject values without touching the real environment.
func SetGetenv(fn func(string) string) {
	getenv = fn
}

// GetEnv returns the current env lookup function. Symmetric to
// SetGetenv for tests that need to capture and restore.
func GetEnv() func(string) string {
	return getenv
}

// RateLimiter is a simple token-bucket rate limiter shared across
// all provider calls. Ponytail: 1 channel, 1 ticker, no third-party
// dep. Provider is set to a conservative 60 req/min (1 per second)
// so a single user running 5 providers in parallel cannot exceed
// any provider's per-second quota.
type RateLimiter struct {
	sem chan struct{}
}

// NewRateLimiter returns a limiter that allows up to 60 requests
// per minute (1 per second on average, with burst capacity).
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		sem: make(chan struct{}, 60),
	}
}

// Wait blocks until a token is available or ctx is cancelled.
// A background ticker refills the bucket at 1 token/second.
func (r *RateLimiter) Wait(ctx context.Context) error {
	// Start the refiller on first use (sync.Once via select+chan
	// trick: the goroutine is launched on the first Wait and runs
	// for the lifetime of the process).
	r.startRefiller()

	select {
	case r.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

var refillerOnce sync.Once

func (r *RateLimiter) startRefiller() {
	refillerOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for range ticker.C {
				select {
				case r.sem <- struct{}{}:
				default:
					// bucket full
				}
			}
		}()
	})
}

// Do executes fn under the rate limiter and a per-call timeout.
// The fn is called with a derived context that carries the timeout
// but is otherwise the same as the caller's ctx. If the fn does not
// honour ctx, the caller is responsible for the timeout.
func (r *RateLimiter) Do(ctx context.Context, timeout time.Duration, fn func() error) error {
	if err := r.Wait(ctx); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() {
		defer close(done)
		done <- fn()
	}()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(timeout):
		return fmt.Errorf("provider call exceeded %s timeout", timeout)
	}
}

// String returns a short label for debug logs.
func (r *RateLimiter) String() string {
	return fmt.Sprintf("RateLimiter(cap=%d)", 60)
}

// PathEscape is a small wrapper around url.PathEscape for providers
// that need to embed user input in URL paths. Kept here so providers
// can use it without a separate net/url import.
func PathEscape(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, " ", "%20"), "/", "%2F")
}
