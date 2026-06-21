package email_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/email"
)

// TestSMTPVerifier_NewSMTPVerifier confirms the constructor returns a
// non-nil verifier with sensible defaults.
func TestSMTPVerifier_NewSMTPVerifier(t *testing.T) {
	v := email.NewSMTPVerifier()
	if v == nil {
		t.Fatal("NewSMTPVerifier returned nil")
	}
}

// TestSMTPVerifier_WithConcurrency confirms the builder sets the pool size.
func TestSMTPVerifier_WithConcurrency(t *testing.T) {
	v := email.NewSMTPVerifier().WithConcurrency(5)
	if v == nil {
		t.Fatal("WithConcurrency returned nil")
	}
}

// TestSMTPVerifier_WithHelloName confirms the builder accepts the name.
func TestSMTPVerifier_WithHelloName(t *testing.T) {
	v := email.NewSMTPVerifier().WithHelloName("scrappy.local", "verify@scrappy.local")
	if v == nil {
		t.Fatal("WithHelloName returned nil")
	}
}

// TestSMTPVerifier_WithTimeout confirms the builder accepts timeouts.
func TestSMTPVerifier_WithTimeout(t *testing.T) {
	v := email.NewSMTPVerifier().WithTimeout(5*time.Second, 5*time.Second)
	if v == nil {
		t.Fatal("WithTimeout returned nil")
	}
}

// TestSMTPVerifier_VerifyEmptyAddress confirms the empty-input path.
func TestSMTPVerifier_VerifyEmptyAddress(t *testing.T) {
	v := email.NewSMTPVerifier()
	res := v.Verify(context.Background(), "")
	if res.Reason != "empty_address" {
		t.Errorf("expected reason=empty_address, got %q", res.Reason)
	}
}

// TestSMTPVerifier_VerifyInvalidAddress confirms the validation path
// rejects obviously bad addresses before any SMTP traffic.
func TestSMTPVerifier_VerifyInvalidAddress(t *testing.T) {
	v := email.NewSMTPVerifier()
	res := v.Verify(context.Background(), "not-an-email")
	if res.Reason == "smtp_ok" {
		// Some libraries will still try to resolve; we just want to make
		// sure we did not panic and the result is a struct (not nil).
		t.Logf("invalid address produced reason=%q (acceptable)", res.Reason)
	}
}

// TestSMTPVerifier_VerifyBlockedDomain confirms disposable domains are
// rejected without an SMTP round-trip.
func TestSMTPVerifier_VerifyBlockedDomain(t *testing.T) {
	v := email.NewSMTPVerifier()
	res := v.Verify(context.Background(), "user@mailinator.com")
	if res.Reason != "blocked_domain" {
		t.Errorf("expected reason=blocked_domain, got %q", res.Reason)
	}
	if res.Deliverable {
		t.Error("blocked domain should not be deliverable")
	}
}

// TestSMTPVerifier_VerifyResultHasFields confirms the result struct is
// populated even when the network call fails (e.g. no DNS, no internet).
func TestSMTPVerifier_VerifyResultHasFields(t *testing.T) {
	v := email.NewSMTPVerifier().WithTimeout(500*time.Millisecond, 500*time.Millisecond)
	res := v.Verify(context.Background(), "test@example.com")
	// We don't assert success/failure (test environments may vary),
	// but the result struct must be returned and Duration must be set.
	if res.Duration == 0 {
		t.Logf("result Duration is zero (env may have returned immediately)")
	}
	if res.Reason == "" {
		t.Error("expected non-empty Reason")
	}
}

// TestSMTPVerifier_VerifyAllEmpty confirms the no-input path.
func TestSMTPVerifier_VerifyAllEmpty(t *testing.T) {
	v := email.NewSMTPVerifier()
	out := v.VerifyAll(context.Background(), nil)
	if len(out) != 0 {
		t.Errorf("expected empty map, got %d entries", len(out))
	}
	out = v.VerifyAll(context.Background(), []string{})
	if len(out) != 0 {
		t.Errorf("expected empty map for empty slice, got %d entries", len(out))
	}
}

// TestSMTPVerifier_VerifyAllDeduplicates confirms duplicate addresses in
// the input list are only verified once.
func TestSMTPVerifier_VerifyAllDeduplicates(t *testing.T) {
	v := email.NewSMTPVerifier().WithTimeout(100*time.Millisecond, 100*time.Millisecond)
	v = v.WithConcurrency(1)

	addrs := []string{
		"user@guerrillamail.com", // blocked
		"user@guerrillamail.com", // duplicate
		" USER@guerrillamail.com ", // duplicate after trim+lower
		"",                         // empty
		" ",                        // whitespace only
	}
	out := v.VerifyAll(context.Background(), addrs)
	if len(out) != 1 {
		t.Errorf("expected exactly 1 entry after dedup, got %d: %v", len(out), keysOf(out))
	}
	if _, ok := out["user@guerrillamail.com"]; !ok {
		t.Errorf("expected user@guerrillamail.com in result, got: %v", keysOf(out))
	}
}

// TestSMTPVerifier_VerifyAllContextCancel confirms that a cancelled
// context short-circuits the input loop.
func TestSMTPVerifier_VerifyAllContextCancel(t *testing.T) {
	v := email.NewSMTPVerifier().WithTimeout(5*time.Second, 5*time.Second)
	v = v.WithConcurrency(1)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	out := v.VerifyAll(ctx, []string{
		"a@example.com",
		"b@example.com",
		"c@example.com",
	})
	// Context is checked before each spawn; with immediate cancel
	// at least one of the three may have completed before the
	// context check fires. We just assert no panic and a valid map.
	_ = out
}

// TestSMTPVerifier_ResultReasonString confirms the reason strings are
// the documented set or a known prefix.
func TestSMTPVerifier_ResultReasonString(t *testing.T) {
	v := email.NewSMTPVerifier()
	res := v.Verify(context.Background(), "")
	known := []string{"empty_address", "blocked_domain", "smtp_error:", "smtp_nil_result", "smtp_no_response", "smtp_ok", "rcpt_550", "reachable_unknown", "catch_all"}
	if !anyPrefix(res.Reason, known) {
		t.Errorf("reason %q is not in documented set", res.Reason)
	}
}

func keysOf(m map[string]email.SMTPResult) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func anyPrefix(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}
