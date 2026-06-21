package email_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/arinbalyan/scrappy/internal/email"
)

// TestMultiPageCompanyEnricher_ExtractsEmailsFromAboutPage confirms emails
// on /about are picked up alongside the root page.
func TestMultiPageCompanyEnricher_ExtractsEmailsFromAboutPage(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body>Homepage with no emails.</body></html>`)
	})
	mux.HandleFunc("/about", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body>Contact us at <a href="mailto:hello@acme.com">hello@acme.com</a></body></html>`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	e := email.NewMultiPageCompanyEnricher(srv.Client(), 3, 0)
	// Disable MX verification for the test by using a nil verifier.
	e.Verifier = nil
	out, err := e.Enrich(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Enrich: %v", err)
	}
	hasHello := false
	for _, x := range out {
		if x.Addr == "hello@acme.com" {
			hasHello = true
		}
	}
	if !hasHello {
		t.Errorf("expected hello@acme.com from /about page, got: %+v", out)
	}
}

// TestMultiPageCompanyEnricher_ExtractsFromMultiplePages confirms that
// emails across /about, /team, /contact are all returned.
func TestMultiPageCompanyEnricher_ExtractsFromMultiplePages(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body>root: info@acme.com</body></html>`)
	})
	mux.HandleFunc("/about", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body>about: hello@acme.com</body></html>`)
	})
	mux.HandleFunc("/team", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body>team: founders@acme.com</body></html>`)
	})
	mux.HandleFunc("/contact", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body>contact: press@acme.com</body></html>`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	e := email.NewMultiPageCompanyEnricher(srv.Client(), 3, 0)
	e.Verifier = nil
	out, err := e.Enrich(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Enrich: %v", err)
	}
	want := map[string]bool{
		"info@acme.com":    false,
		"hello@acme.com":   false,
		"founders@acme.com": false,
		"press@acme.com":   false,
	}
	for _, x := range out {
		if _, ok := want[x.Addr]; ok {
			want[x.Addr] = true
		}
	}
	for addr, found := range want {
		if !found {
			t.Errorf("expected %s in results, missing. got: %+v", addr, out)
		}
	}
}

// TestMultiPageCompanyEnricher_Dedup ensures duplicates across pages are collapsed.
func TestMultiPageCompanyEnricher_Dedup(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body>Contact: hello@acme.com</body></html>`)
	})
	mux.HandleFunc("/about", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body>Contact: hello@acme.com</body></html>`)
	})
	mux.HandleFunc("/team", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body>Contact: hello@acme.com</body></html>`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	e := email.NewMultiPageCompanyEnricher(srv.Client(), 3, 0)
	e.Verifier = nil
	out, err := e.Enrich(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Enrich: %v", err)
	}
	count := 0
	for _, x := range out {
		if x.Addr == "hello@acme.com" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 hello@acme.com after dedup, got %d", count)
	}
}

// TestMultiPageCompanyEnricher_EmptyURL confirms graceful no-op on empty input.
func TestMultiPageCompanyEnricher_EmptyURL(t *testing.T) {
	e := email.NewMultiPageCompanyEnricher(&http.Client{}, 3, 0)
	out, err := e.Enrich(context.Background(), "")
	if err != nil {
		t.Errorf("expected nil error, got: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("expected no emails, got: %+v", out)
	}
}

// TestMultiPageCompanyEnricher_ContextCancel confirms cancellation stops
// the enrichment loop without panicking.
func TestMultiPageCompanyEnricher_ContextCancel(t *testing.T) {
	mux := http.NewServeMux()
	for _, p := range []string{"/", "/about", "/team", "/contact", "/careers"} {
		mux.HandleFunc(p, func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `<html><body>some@acme.com</body></html>`)
		})
	}
	srv := httptest.NewServer(mux)
	defer srv.Close()

	e := email.NewMultiPageCompanyEnricher(srv.Client(), 1, 100)
	e.Verifier = nil

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before call
	_, _ = e.Enrich(ctx, srv.URL) // should not panic
}

// TestMultiPageCompanyEnricher_SourceIsCompanyPage confirms the Source
// field is set to "company_page" for every returned email.
func TestMultiPageCompanyEnricher_SourceIsCompanyPage(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body>info@acme.com</body></html>`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	e := email.NewMultiPageCompanyEnricher(srv.Client(), 3, 0)
	e.Verifier = nil
	out, err := e.Enrich(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Enrich: %v", err)
	}
	if len(out) == 0 {
		t.Fatalf("expected at least one email")
	}
	for _, x := range out {
		if x.Source != "company_page" {
			t.Errorf("expected source=company_page, got %q", x.Source)
		}
	}
}

// TestMultiPageCompanyEnricher_FetchFailureIsNonFatal confirms that a
// 5xx response from one page does not stop the loop.
func TestMultiPageCompanyEnricher_FetchFailureIsNonFatal(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body>info@acme.com</body></html>`)
	})
	mux.HandleFunc("/about", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	})
	mux.HandleFunc("/team", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body>team@acme.com</body></html>`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	e := email.NewMultiPageCompanyEnricher(srv.Client(), 3, 0)
	e.Verifier = nil
	out, err := e.Enrich(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Enrich: %v", err)
	}
	hasInfo := false
	hasTeam := false
	for _, x := range out {
		if x.Addr == "info@acme.com" {
			hasInfo = true
		}
		if x.Addr == "team@acme.com" {
			hasTeam = true
		}
	}
	if !hasInfo {
		t.Errorf("expected info@acme.com (root page), got: %+v", out)
	}
	if !hasTeam {
		t.Errorf("expected team@acme.com (after /about failed), got: %+v", out)
	}
}

// TestMultiPageCompanyEnricher_CustomPaths confirms that callers can
// override the default path list via PagePaths.
func TestMultiPageCompanyEnricher_CustomPaths(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/custom-page") {
			fmt.Fprint(w, `<html><body>custom@acme.com</body></html>`)
			return
		}
		fmt.Fprint(w, `<html><body>root@acme.com</body></html>`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	e := email.NewMultiPageCompanyEnricher(srv.Client(), 3, 0)
	e.PagePaths = []string{"/custom-page"}
	e.Verifier = nil
	out, err := e.Enrich(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Enrich: %v", err)
	}
	hasCustom := false
	for _, x := range out {
		if x.Addr == "custom@acme.com" {
			hasCustom = true
		}
	}
	if !hasCustom {
		t.Errorf("expected custom@acme.com from /custom-page, got: %+v", out)
	}
}
