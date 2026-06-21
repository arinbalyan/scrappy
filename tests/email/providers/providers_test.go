package providers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/email/providers"
)

// TestHunter_Name confirms the provider name.
func TestHunter_Name(t *testing.T) {
	h := providers.NewHunter("test-key", nil)
	if h.Name() != "hunter" {
		t.Errorf("expected name=hunter, got %q", h.Name())
	}
}

// TestHunter_Available confirms the credentials check.
func TestHunter_Available(t *testing.T) {
	h := providers.NewHunter("", nil)
	if err := h.Available(); err == nil {
		t.Error("expected error for empty key")
	}
	h2 := providers.NewHunter("test-key", nil)
	if err := h2.Available(); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

// TestHunter_DomainSearch_NoKey confirms graceful failure without a key.
func TestHunter_DomainSearch_NoKey(t *testing.T) {
	h := providers.NewHunter("", nil)
	_, err := h.DomainSearch(context.Background(), "acme.com")
	if err == nil {
		t.Error("expected error for empty key")
	}
}

// TestHunter_DomainSearch_HappyPath confirms the response is parsed.
func TestHunter_DomainSearch_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the API key is included.
		if !strings.Contains(r.URL.RawQuery, "api_key=test-key") {
			http.Error(w, "missing api_key", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{
			"data": {
				"domain": "acme.com",
				"pattern": "{first}.{last}",
				"accept_all": false,
				"emails": [
					{
						"value": "john.doe@acme.com",
						"type": "personal",
						"confidence": 95,
						"first_name": "John",
						"last_name": "Doe",
						"position": "Engineer",
						"department": "Engineering"
					}
				]
			}
		}`)
	}))
	defer srv.Close()

	h := providers.NewHunter("test-key", &http.Client{
		Transport: &rewriteHost{base: srv.URL},
	})
	got, err := h.DomainSearch(context.Background(), "acme.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 email, got %d", len(got))
	}
	if got[0].Address != "john.doe@acme.com" {
		t.Errorf("expected john.doe@acme.com, got %q", got[0].Address)
	}
	if got[0].Source != "hunter" {
		t.Errorf("expected source=hunter, got %q", got[0].Source)
	}
	if got[0].Confidence != 95 {
		t.Errorf("expected confidence=95, got %v", got[0].Confidence)
	}
}

// TestHunter_EmailFinder_HappyPath confirms finder returns the address.
func TestHunter_EmailFinder_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"data": {"email": "ada.lovelace@acme.com", "score": 90}}`)
	}))
	defer srv.Close()

	h := providers.NewHunter("k", &http.Client{Transport: &rewriteHost{base: srv.URL}})
	got, err := h.EmailFinder(context.Background(), "Ada", "Lovelace", "acme.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "ada.lovelace@acme.com" {
		t.Errorf("expected ada.lovelace@acme.com, got %q", got)
	}
}

// TestHunter_EmailFinder_EmptyInputs confirms graceful handling.
func TestHunter_EmailFinder_EmptyInputs(t *testing.T) {
	h := providers.NewHunter("k", nil)
	_, err := h.EmailFinder(context.Background(), "", "", "acme.com")
	if err == nil {
		t.Error("expected error for empty names")
	}
}

// TestHunter_Verify_HappyPath confirms verify parses the result.
func TestHunter_Verify_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"data": {"status": "valid", "result": "deliverable", "score": 98, "accept_all": false}}`)
	}))
	defer srv.Close()

	h := providers.NewHunter("k", &http.Client{Transport: &rewriteHost{base: srv.URL}})
	ok, status, err := h.Verify(context.Background(), "test@acme.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected deliverable=true")
	}
	if status != "valid" {
		t.Errorf("expected status=valid, got %q", status)
	}
}

// TestHunter_RateLimited confirms 429 is mapped to ErrRateLimited.
func TestHunter_RateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limit", http.StatusTooManyRequests)
	}))
	defer srv.Close()

	h := providers.NewHunter("k", &http.Client{Transport: &rewriteHost{base: srv.URL}})
	_, err := h.DomainSearch(context.Background(), "acme.com")
	if err != providers.ErrRateLimited {
		t.Errorf("expected ErrRateLimited, got: %v", err)
	}
}

// TestTomba_Name confirms the provider name.
func TestTomba_Name(t *testing.T) {
	t2 := providers.NewTomba("k", "s", nil)
	if t2.Name() != "tomba" {
		t.Errorf("expected name=tomba, got %q", t2.Name())
	}
}

// TestTomba_Available confirms the credentials check.
func TestTomba_Available(t *testing.T) {
	cases := []struct {
		key, secret string
		wantErr     bool
	}{
		{"", "", true},
		{"k", "", true},
		{"", "s", true},
		{"k", "s", false},
	}
	for _, c := range cases {
		t2 := providers.NewTomba(c.key, c.secret, nil)
		err := t2.Available()
		if (err != nil) != c.wantErr {
			t.Errorf("Tomba(key=%q, secret=%q): wantErr=%v, got: %v", c.key, c.secret, c.wantErr, err)
		}
	}
}

// TestTomba_EmailFinder_HappyPath confirms finder parses the response.
func TestTomba_EmailFinder_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the Tomba auth headers are present.
		if r.Header.Get("X-Tomba-Key") != "k" || r.Header.Get("X-Tomba-Secret") != "s" {
			http.Error(w, "missing tomba auth", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"data": {"email": "ada@acme.com", "score": 85}}`)
	}))
	defer srv.Close()

	t2 := providers.NewTomba("k", "s", &http.Client{Transport: &rewriteHost{base: srv.URL}})
	got, err := t2.EmailFinder(context.Background(), "Ada", "Lovelace", "acme.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "ada@acme.com" {
		t.Errorf("expected ada@acme.com, got %q", got)
	}
}

// TestTomba_Verify_NotSupported confirms Verify returns an error.
func TestTomba_Verify_NotSupported(t *testing.T) {
	t2 := providers.NewTomba("k", "s", nil)
	_, _, err := t2.Verify(context.Background(), "x@x.com")
	if err == nil {
		t.Error("expected Tomba Verify to return an error (not supported)")
	}
}

// TestSnov_Available confirms the credentials check.
func TestSnov_Available(t *testing.T) {
	s := providers.NewSnov("", "", nil)
	if err := s.Available(); err == nil {
		t.Error("expected error for empty credentials")
	}
	s = providers.NewSnov("id", "secret", nil)
	if err := s.Available(); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

// TestSnov_StubMethods confirms the stub methods return ErrNotFound.
func TestSnov_StubMethods(t *testing.T) {
	s := providers.NewSnov("id", "secret", nil)
	if _, err := s.DomainSearch(context.Background(), "acme.com"); err != providers.ErrNotFound {
		t.Errorf("DomainSearch: expected ErrNotFound, got: %v", err)
	}
	if _, err := s.EmailFinder(context.Background(), "a", "b", "c.com"); err != providers.ErrNotFound {
		t.Errorf("EmailFinder: expected ErrNotFound, got: %v", err)
	}
	if _, _, err := s.Verify(context.Background(), "x@x.com"); err != providers.ErrNotFound {
		t.Errorf("Verify: expected ErrNotFound, got: %v", err)
	}
}

// TestApollo_Available confirms the credentials check.
func TestApollo_Available(t *testing.T) {
	a := providers.NewApollo("", nil)
	if err := a.Available(); err == nil {
		t.Error("expected error for empty key")
	}
	a = providers.NewApollo("key", nil)
	if err := a.Available(); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

// TestApollo_StubMethods confirms the stub methods.
func TestApollo_StubMethods(t *testing.T) {
	a := providers.NewApollo("k", nil)
	if _, err := a.DomainSearch(context.Background(), "acme.com"); err != providers.ErrNotFound {
		t.Errorf("DomainSearch: expected ErrNotFound, got: %v", err)
	}
}

// TestDropcontact_Available confirms the credentials check.
func TestDropcontact_Available(t *testing.T) {
	d := providers.NewDropcontact("", nil)
	if err := d.Available(); err == nil {
		t.Error("expected error for empty key")
	}
}

// TestDropcontact_StubMethods confirms the stub methods.
func TestDropcontact_StubMethods(t *testing.T) {
	d := providers.NewDropcontact("k", nil)
	if _, err := d.DomainSearch(context.Background(), "acme.com"); err != providers.ErrNotFound {
		t.Errorf("DomainSearch: expected ErrNotFound, got: %v", err)
	}
}

// TestRegistry_Register_And_List confirms the registry contract.
func TestRegistry_Register_And_List(t *testing.T) {
	r := providers.NewRegistry()
	if len(r.Providers()) != 0 {
		t.Errorf("expected empty registry, got %d", len(r.Providers()))
	}
	r.Register(providers.NewHunter("k", nil))
	r.Register(providers.NewApollo("k", nil))
	if len(r.Providers()) != 2 {
		t.Errorf("expected 2 providers, got %d", len(r.Providers()))
	}
}

// TestRegistry_LoadFromEnv confirms only credentialled providers load.
func TestRegistry_LoadFromEnv(t *testing.T) {
	// Save and restore getenv.
	origGetenv := providers.GetEnv()
	defer providers.SetGetenv(origGetenv)

	providers.SetGetenv(func(key string) string {
		switch key {
		case "HUNTER_API_KEY":
			return "hunter-key"
		case "TOMBA_KEY":
			return "tomba-key"
		case "APOLLO_API_KEY":
			return "apollo-key"
		case "SNOV_CLIENT_ID":
			return ""
		case "TOMBA_SECRET":
			return "tomba-secret"
		default:
			return ""
		}
	})

	r := providers.NewRegistry()
	r.LoadFromEnv(providers.LoadEnv(), nil)
	got := r.Providers()
	if len(got) != 3 {
		t.Errorf("expected 3 providers (hunter, tomba, apollo), got %d", len(got))
	}
	names := make([]string, len(got))
	for i, p := range got {
		names[i] = p.Name()
	}
	// Registration order: Hunter, Tomba, Apollo.
	want := []string{"hunter", "tomba", "apollo"}
	for i, n := range want {
		if i < len(names) && names[i] != n {
			t.Errorf("expected %q at index %d, got %q (full list: %v)", n, i, names[i], names)
		}
	}
}

// TestLoadEnv confirms LoadEnv returns the right keys.
func TestLoadEnv(t *testing.T) {
	// Save and restore.
	origGetenv := providers.GetEnv()
	defer providers.SetGetenv(origGetenv)

	providers.SetGetenv(func(key string) string {
		switch key {
		case "HUNTER_API_KEY":
			return "h"
		case "TOMBA_KEY":
			return "tk"
		case "TOMBA_SECRET":
			return "ts"
		case "SNOV_CLIENT_ID":
			return "si"
		case "SNOV_CLIENT_SECRET":
			return "ss"
		case "APOLLO_API_KEY":
			return "a"
		case "DROPCONTACT_API_KEY":
			return "d"
		}
		return ""
	})

	env := providers.LoadEnv()
	if env.HunterKey != "h" || env.TombaKey != "tk" || env.TombaSecret != "ts" ||
		env.SnovClientID != "si" || env.SnovClientSecret != "ss" ||
		env.ApolloKey != "a" || env.DropcontactKey != "d" {
		t.Errorf("LoadEnv returned wrong values: %+v", env)
	}
}

// TestRateLimiter_ContextCancel confirms cancellation is respected.
// Drains the rate limiter to a known state (bucket empty) so the
// test is deterministic regardless of the refiller timing. Without
// the drain, the bucket is full from the initial 60-token burst and
// Wait always succeeds without ever checking ctx.
func TestRateLimiter_ContextCancel(t *testing.T) {
	rl := providers.NewRateLimiter()
	drainCtx := context.Background()

	// Drain by issuing time-bounded Wait calls until the limiter
	// refuses to grant a token within the timeout. Each successful
	// Wait consumes one token.
	deadline := time.Now().Add(2 * time.Second)
	tokensConsumed := 0
	for time.Now().Before(deadline) {
		c, cancel := context.WithTimeout(drainCtx, 20*time.Millisecond)
		err := rl.Wait(c)
		cancel()
		if err != nil {
			break // bucket is empty (or ctx expired)
		}
		tokensConsumed++
	}
	if tokensConsumed < 1 {
		t.Fatalf("expected to consume at least 1 token; consumed %d", tokensConsumed)
	}

	// With the bucket now empty and ctx cancelled, the select must
	// pick the ctx.Done() case rather than the channel send.
	ctx, cancel := context.WithCancel(drainCtx)
	cancel()
	start := time.Now()
	err := rl.Wait(ctx)
	elapsed := time.Since(start)
	if err == nil {
		t.Error("expected error from cancelled context after bucket drain")
	}
	// Should return promptly (ctx cancellation path), not block on refill.
	if elapsed > 100*time.Millisecond {
		t.Errorf("Wait took %s after ctx cancel; expected fast return", elapsed)
	}
}

// TestRateLimiter_AllowFirst confirms a single call goes through.
func TestRateLimiter_AllowFirst(t *testing.T) {
	rl := providers.NewRateLimiter()
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	if err := rl.Wait(ctx); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

// TestRateLimiter_String confirms the label.
func TestRateLimiter_String(t *testing.T) {
	rl := providers.NewRateLimiter()
	s := rl.String()
	if !strings.Contains(s, "RateLimiter") {
		t.Errorf("expected string to contain RateLimiter, got %q", s)
	}
}

// TestRateLimiter_DoTimeout confirms timeout path.
func TestRateLimiter_DoTimeout(t *testing.T) {
	rl := providers.NewRateLimiter()
	ctx := context.Background()
	err := rl.Do(ctx, 50*time.Millisecond, func() error {
		time.Sleep(200 * time.Millisecond)
		return nil
	})
	if err == nil {
		t.Error("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Errorf("expected timeout in error, got: %v", err)
	}
}

// TestPathEscape_HandlesSpecialChars confirms the helper.
func TestPathEscape_HandlesSpecialChars(t *testing.T) {
	got := providers.PathEscape("hello world/foo")
	if got != "hello%20world%2Ffoo" {
		t.Errorf("expected escaped string, got %q", got)
	}
}

// TestHunter_DomainSearch_APIError confirms error responses are surfaced.
func TestHunter_DomainSearch_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"errors": [{"code": 401, "details": "Invalid API key"}]}`)
	}))
	defer srv.Close()

	h := providers.NewHunter("k", &http.Client{Transport: &rewriteHost{base: srv.URL}})
	_, err := h.DomainSearch(context.Background(), "acme.com")
	if err == nil {
		t.Error("expected error for invalid API key")
	}
	if !strings.Contains(err.Error(), "Invalid API key") {
		t.Errorf("expected error to mention 'Invalid API key', got: %v", err)
	}
}

// Helper: a custom RoundTripper that redirects any request to the
// test server. The provider code builds full URLs against the real
// API host; this Transport rewrites the host to the test server.
type rewriteHost struct {
	base string
}

func (t *rewriteHost) RoundTrip(req *http.Request) (*http.Response, error) {
	testURL := t.base + req.URL.Path
	if req.URL.RawQuery != "" {
		testURL += "?" + req.URL.RawQuery
	}
	newReq, err := http.NewRequest(req.Method, testURL, req.Body)
	if err != nil {
		return nil, err
	}
	for k, v := range req.Header {
		newReq.Header[k] = v
	}
	return http.DefaultClient.Do(newReq)
}

// TestProvider_NamesUnique ensures the 5 provider names are distinct
// strings (catches copy-paste mistakes in Name()).
func TestProvider_NamesUnique(t *testing.T) {
	names := []string{
		providers.NewHunter("k", nil).Name(),
		providers.NewTomba("k", "s", nil).Name(),
		providers.NewSnov("k", "s", nil).Name(),
		providers.NewApollo("k", nil).Name(),
		providers.NewDropcontact("k", nil).Name(),
	}
	seen := make(map[string]bool, len(names))
	for _, n := range names {
		if seen[n] {
			t.Errorf("duplicate provider name: %q", n)
		}
		seen[n] = true
	}
	if len(seen) != 5 {
		t.Errorf("expected 5 unique provider names, got %d", len(seen))
	}
}

// Silence unused warnings for atomics and json in case some
// test bodies get edited.
var (
	_ = atomic.LoadInt32
	_ = json.Marshal
	_ = httptest.NewServer
)
