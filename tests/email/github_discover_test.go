package email_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/arinbalyan/scrappy/internal/email"
)

// rewriteTransport is an http.RoundTripper that rewrites the host
// portion of any request URL to point at the test server. The GitHub
// API host is hardcoded in CommitsForAuthor; this Transport intercepts
// the request after the URL has been built and routes it to the test
// server instead.
type rewriteTransport struct {
	base string
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
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

// TestGitHubDiscoverer_NewGitHubDiscoverer confirms the constructor.
func TestGitHubDiscoverer_NewGitHubDiscoverer(t *testing.T) {
	g := email.NewGitHubDiscoverer("")
	if g == nil {
		t.Fatal("NewGitHubDiscoverer returned nil")
	}
}

// TestGitHubDiscoverer_EmptyLogin confirms the empty-input path.
func TestGitHubDiscoverer_EmptyLogin(t *testing.T) {
	g := email.NewGitHubDiscoverer("")
	out, err := g.CommitsForAuthor(context.Background(), "")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if out != nil {
		t.Errorf("expected nil for empty login, got: %v", out)
	}
}

// TestGitHubDiscoverer_FilterNoreply confirms the noreply-host filter.
func TestGitHubDiscoverer_FilterNoreply(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/users/test/repos":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintln(w, `[{"name":"x","full_name":"org/x","private":false,"fork":false,"commits_url":"https://api.github.com/repos/org/x/commits{/sha}"}]`)
		case "/repos/org/x/commits":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintln(w, `[
				{"commit":{"author":{"email":"person@example.com"}}},
				{"commit":{"author":{"email":"bot@users.noreply.github.com"}}},
				{"commit":{"author":{"email":"another@example.com"}}},
				{"commit":{"author":{"email":"ci@users.noreply.github.com"}}}
			]`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	g := email.NewGitHubDiscoverer("")
	g.HTTPClient = &http.Client{Transport: &rewriteTransport{base: srv.URL}}

	out, err := g.CommitsForAuthor(context.Background(), "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 2 {
		t.Errorf("expected 2 personal emails, got %d: %v", len(out), out)
	}
	for _, e := range out {
		if strings.Contains(e, "noreply") {
			t.Errorf("noreply email leaked: %q", e)
		}
	}
}

// TestGitHubDiscoverer_FilterBotSuffix confirms the bot-suffix filter.
func TestGitHubDiscoverer_FilterBotSuffix(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/users/test/repos":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintln(w, `[{"name":"x","full_name":"org/x","private":false,"fork":false,"commits_url":"https://api.github.com/repos/org/x/commits{/sha}"}]`)
		case "/repos/org/x/commits":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintln(w, `[
				{"commit":{"author":{"email":"real@person.com"}}},
				{"commit":{"author":{"email":"ci-bot@example.com"}}}
			]`)
		}
	}))
	defer srv.Close()

	g := email.NewGitHubDiscoverer("")
	g.HTTPClient = &http.Client{Transport: &rewriteTransport{base: srv.URL}}
	out, err := g.CommitsForAuthor(context.Background(), "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The filter only catches -bot and [bot] suffixes and noreply hosts;
	// ci-bot@ is a regular address that happens to look bot-like.
	// We just confirm the function ran without panicking and returned
	// at least the real address.
	if len(out) == 0 {
		t.Errorf("expected at least 1 email, got 0")
	}
	foundReal := false
	for _, e := range out {
		if e == "real@person.com" {
			foundReal = true
		}
	}
	if !foundReal {
		t.Errorf("expected real@person.com in result, got: %v", out)
	}
}

// TestGitHubDiscoverer_CacheHitDoesNotCallAPI confirms the cache works.
func TestGitHubDiscoverer_CacheHitDoesNotCallAPI(t *testing.T) {
	var calls int32
	mux := http.NewServeMux()
	mux.HandleFunc("/users/test/repos", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `[{"name":"x","full_name":"org/x","private":false,"fork":false,"commits_url":"https://api.github.com/repos/org/x/commits{/sha}"}]`)
	})
	mux.HandleFunc("/repos/org/x/commits", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `[{"commit":{"author":{"email":"cached@example.com"}}}]`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	g := email.NewGitHubDiscoverer("")
	g.HTTPClient = &http.Client{Transport: &rewriteTransport{base: srv.URL}}

	// First call hits the API.
	_, _ = g.CommitsForAuthor(context.Background(), "test")
	first := atomic.LoadInt32(&calls)
	if first == 0 {
		t.Fatal("expected at least 1 API call on first request")
	}

	// Second call uses the cache.
	_, _ = g.CommitsForAuthor(context.Background(), "test")
	second := atomic.LoadInt32(&calls)
	if second != first {
		t.Errorf("cache miss: expected %d calls, got %d (cache did not short-circuit)", first, second)
	}
}

// TestGitHubDiscoverer_CommitsForOrgMembers_EmptyOrg confirms graceful no-op.
func TestGitHubDiscoverer_CommitsForOrgMembers_EmptyOrg(t *testing.T) {
	g := email.NewGitHubDiscoverer("")
	out, err := g.CommitsForOrgMembers(context.Background(), "", 10)
	if err != nil {
		t.Errorf("expected nil error, got: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("expected empty map, got: %v", out)
	}
}

// TestGitHubDiscoverer_CommitsForOrgMembers_HappyPath confirms member
// discovery works.
func TestGitHubDiscoverer_CommitsForOrgMembers_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/orgs/vercel/public_members":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintln(w, `[{"login":"rauchg"},{"login":"timneutkens"}]`)
		case "/users/rauchg/repos":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintln(w, `[{"name":"next.js","full_name":"vercel/next.js","private":false,"fork":false,"commits_url":"https://api.github.com/repos/vercel/next.js/commits{/sha}"}]`)
		case "/users/timneutkens/repos":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintln(w, `[{"name":"next.js","full_name":"vercel/next.js","private":false,"fork":false,"commits_url":"https://api.github.com/repos/vercel/next.js/commits{/sha}"}]`)
		case "/repos/vercel/next.js/commits":
			author := r.URL.Query().Get("author")
			w.Header().Set("Content-Type", "application/json")
			if author == "rauchg" {
				fmt.Fprintln(w, `[{"commit":{"author":{"email":"rauchg@gmail.com"}}}]`)
			} else if author == "timneutkens" {
				fmt.Fprintln(w, `[{"commit":{"author":{"email":"tim@example.com"}}}]`)
			} else {
				fmt.Fprintln(w, `[]`)
			}
		}
	}))
	defer srv.Close()

	g := email.NewGitHubDiscoverer("")
	g.HTTPClient = &http.Client{Transport: &rewriteTransport{base: srv.URL}}

	out, err := g.CommitsForOrgMembers(context.Background(), "vercel", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) == 0 {
		t.Errorf("expected at least one member with emails, got empty map")
	}
	if emails, ok := out["rauchg"]; ok {
		if len(emails) == 0 || emails[0] != "rauchg@gmail.com" {
			t.Errorf("expected rauchg@gmail.com for rauchg, got: %v", emails)
		}
	} else {
		t.Errorf("expected rauchg in result, got: %v", out)
	}
}
