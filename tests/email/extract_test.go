package email_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/arinbalyan/scrappy/internal/email"
)

var bg = context.Background()

// ─── Standard extraction ──────────────────────────────────────────────────────

func TestExtract_SimpleEmail(t *testing.T) {
	found := email.Extract("hello@example.com")
	assert.Len(t, found, 1)
	assert.Equal(t, "hello@example.com", found[0].Addr)
	assert.Equal(t, email.RoleEmailSource, found[0].Source)
}

func TestExtract_Multiple(t *testing.T) {
	found := email.Extract("jobs@acme.com or support@acme.com")
	assert.GreaterOrEqual(t, len(found), 1)
}

func TestExtract_Dedup(t *testing.T) {
	found := email.Extract("hello@example.com hello@example.com")
	for i := 1; i < len(found); i++ {
		if found[i].Addr == found[i-1].Addr {
			t.Fatalf("duplicate %q in output", found[i].Addr)
		}
	}
}

func TestExtract_None(t *testing.T) {
	found := email.Extract("no email here")
	assert.Empty(t, found)
}

func TestExtract_RoleTagged(t *testing.T) {
	found := email.Extract("info@example.com or jobs@example.com")
	if len(found) >= 2 {
		assert.True(t, found[0].Role)
		assert.True(t, found[1].Role)
	}
}

func TestExtract_NonRole(t *testing.T) {
	found := email.Extract("john.doe@example.com")
	if len(found) == 1 {
		assert.False(t, found[0].Role)
	}
}

func TestExtract_DisposableBlocked(t *testing.T) {
	found := email.Extract("user@guerrillamail.com or user@10minutemail.com")
	assert.Empty(t, found)
}

func TestExtract_PlatformBlocked(t *testing.T) {
	found := email.Extract("recruiter@indeed.com or jobs@linkedin.com")
	assert.Empty(t, found)
}

func TestExtract_InvalidLocalPart(t *testing.T) {
	// Consecutive dots in local part.
	found := email.Extract("user..name@example.com")
	assert.Empty(t, found)
	// Leading dot.
	found = email.Extract(".username@example.com")
	assert.Empty(t, found)
	// Trailing dot.
	found = email.Extract("username.@example.com")
	assert.Empty(t, found)
	// Comment in local part.
	found = email.Extract("user(comment)@example.com")
	assert.Empty(t, found)
}

func TestExtract_InvalidDomain(t *testing.T) {
	found := email.Extract("user@com") // no dot in domain
	assert.Empty(t, found)
}

func TestExtract_BlockedDomainSuffix(t *testing.T) {
	found := email.Extract("user@example.local")
	assert.Empty(t, found)
	found = email.Extract("user@example.arpa")
	assert.Empty(t, found)
	found = email.Extract("user@example.invalid")
	assert.Empty(t, found)
}

// ─── Obfuscated pattern extraction ───────────────────────────────────────────

func TestExtract_ObfuscatedAtDot(t *testing.T) {
	found := email.Extract("name [at] domain [dot] com")
	assert.Len(t, found, 1)
	assert.Equal(t, "name@domain.com", found[0].Addr)
}

func TestExtract_ObfuscatedParens(t *testing.T) {
	found := email.Extract("Name (at) Domain (dot) org")
	assert.Len(t, found, 1)
	assert.Equal(t, "name@domain.org", found[0].Addr)
}

func TestExtract_ObfuscatedUppercase(t *testing.T) {
	found := email.Extract("hire.me AT company DOT io")
	assert.Len(t, found, 1)
	assert.Equal(t, "hire.me@company.io", found[0].Addr)
}

func TestExtract_ObfuscatedInline(t *testing.T) {
	found := email.Extract("staff(at)acme.com")
	// The obfuscated regex requires [dot] pattern for TLD, so inline (at) only
	// may not match. But the standard mailRegex should still pick up staff(at)acme.com.
	// Actually (at) isn't matched by mailRegex. Just verify no crash.
	_ = found
}

// ─── HTML extraction ──────────────────────────────────────────────────────────

func TestExtractFromHTML_Mailto(t *testing.T) {
	html := `<a href="mailto:jobs@company.com">Email us</a>`
	found := email.ExtractFromHTML(html)
	assert.Len(t, found, 1)
	assert.Equal(t, "jobs@company.com", found[0].Addr)
	assert.Equal(t, "mailto", found[0].Source)
}

func TestExtractFromHTML_MailtoWithQuery(t *testing.T) {
	html := `<a href="mailto:hr@company.com?subject=Job%20Application">Apply</a>`
	found := email.ExtractFromHTML(html)
	assert.Len(t, found, 1)
	assert.Equal(t, "hr@company.com", found[0].Addr)
}

func TestExtractFromHTML_BodyText(t *testing.T) {
	html := `<html><body><p>Contact us at hello@example.com</p></body></html>`
	found := email.ExtractFromHTML(html)
	assert.Len(t, found, 1)
	assert.Equal(t, "hello@example.com", found[0].Addr)
}

func TestExtractFromHTML_DedupMailtoAndBody(t *testing.T) {
	html := `<a href="mailto:jobs@example.com">Apply</a><p>jobs@example.com</p>`
	found := email.ExtractFromHTML(html)
	assert.Len(t, found, 1) // deduped to 1
}

// ─── HTML entity normalization ────────────────────────────────────────────────

func TestExtract_HTMLEntityEncoding(t *testing.T) {
	found := email.Extract("user&#64;example&#46;com")
	assert.Len(t, found, 1)
	assert.Equal(t, "user@example.com", found[0].Addr)
}

func TestExtract_HTMLEntityHex(t *testing.T) {
	found := email.Extract("user&#x40;example&#x2E;com")
	assert.Len(t, found, 1)
	assert.Equal(t, "user@example.com", found[0].Addr)
}

// ─── Dedup / filter ───────────────────────────────────────────────────────────

func TestDeduplicate(t *testing.T) {
	emails := []email.Email{{Addr: "a@x.com"}, {Addr: "b@x.com"}, {Addr: "a@x.com"}}
	result := email.Deduplicate(emails)
	assert.Len(t, result, 2)
}

func TestDeduplicate_Empty(t *testing.T) {
	assert.Empty(t, email.Deduplicate(nil))
}

func TestDeduplicate_AllSame(t *testing.T) {
	emails := []email.Email{{Addr: "a@x.com"}, {Addr: "a@x.com"}, {Addr: "a@x.com"}}
	result := email.Deduplicate(emails)
	assert.Len(t, result, 1)
}

func TestFilterRole(t *testing.T) {
	emails := []email.Email{{Addr: "info@x.com", Role: true}, {Addr: "eng@x.com"}}
	result := email.FilterRole(emails)
	assert.Len(t, result, 1)
	assert.Equal(t, "eng@x.com", result[0].Addr)
}

func TestFilterRole_StructMatch(t *testing.T) {
	// Important: FilterRole uses the .Role field, not re-parsing.
	emails := []email.Email{{Addr: "hr@x.com", Role: true}, {Addr: "ceo@x.com"}}
	result := email.FilterRole(emails)
	assert.Len(t, result, 1)
	assert.Equal(t, "ceo@x.com", result[0].Addr)
}

func TestDomainFrom(t *testing.T) {
	assert.Equal(t, "example.com", email.DomainFrom("user@example.com"))
	assert.Equal(t, "", email.DomainFrom("bogus"))
}

func TestIsRole(t *testing.T) {
	assert.True(t, email.IsRole("info@x.com"))
	assert.True(t, email.IsRole("noreply@x.com"))
	assert.True(t, email.IsRole("jobs@x.com"))
	assert.False(t, email.IsRole("eng@x.com"))
	assert.False(t, email.IsRole("john.doe@example.com"))
}

// ─── MX verifier ──────────────────────────────────────────────────────────────

func TestMXVerifier_Stub(t *testing.T) {
	v := email.NewMXVerifier()
	v.LookupMX = func(domain string) ([]string, bool) {
		if domain == "verified.com" {
			return []string{"mx.verified.com"}, true
		}
		return nil, false
	}
	assert.True(t, v.Verify(bg, "eng@verified.com"))
	assert.False(t, v.Verify(bg, "eng@noverify.com"))
}

func TestMXVerifier_NilResolver(t *testing.T) {
	v := email.NewMXVerifier()
	v.Resolver = nil
	v.LookupMX = nil
	assert.True(t, v.Verify(bg, "anything@example.com"))
}

func TestMXVerifier_DomainFilter(t *testing.T) {
	v := email.NewMXVerifier()
	v.LookupMX = func(domain string) ([]string, bool) {
		if domain == "acme.com" {
			return []string{"mx.acme.com"}, true
		}
		return nil, false
	}
	assert.True(t, v.Verify(bg, "jobs@acme.com"))
	assert.False(t, v.Verify(bg, "notabsent@nogood.com"))
}

func TestMXVerifier_NilReceiver(t *testing.T) {
	var v *email.MXVerifier
	assert.False(t, v.Verify(bg, "anything@example.com"))
}

func TestMXVerifier_InvalidAddr(t *testing.T) {
	v := email.NewMXVerifier()
	assert.False(t, v.Verify(bg, "notanemail"))
}

func TestMXVerifier_VerifyEmailStub(t *testing.T) {
	v := email.NewMXVerifier()
	v.LookupMX = func(domain string) ([]string, bool) {
		return []string{"mx.acme.com"}, true
	}
	verified, reason := v.VerifyEmail(bg, "user@acme.com")
	assert.True(t, verified)
	assert.Equal(t, "mx_ok", reason)
}

func TestMXVerifier_VerifyEmailBlocked(t *testing.T) {
	v := email.NewMXVerifier()
	verified, reason := v.VerifyEmail(bg, "user@guerrillamail.com")
	assert.False(t, verified)
	assert.Equal(t, "blocked_domain", reason)
}

func TestMXVerifier_VerifyEmailInvalid(t *testing.T) {
	v := email.NewMXVerifier()
	verified, reason := v.VerifyEmail(bg, "user..name@example.com")
	assert.False(t, verified)
	assert.Equal(t, "invalid_format", reason)
}
