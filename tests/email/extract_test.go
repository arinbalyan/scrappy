package email_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/arinbalyan/scrappy/internal/email"
)

var bg = context.Background()

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

func TestDeduplicate(t *testing.T) {
	emails := []email.Email{{Addr: "a@x.com"}, {Addr: "b@x.com"}, {Addr: "a@x.com"}}
	result := email.Deduplicate(emails)
	assert.Len(t, result, 2)
}

func TestFilterRole(t *testing.T) {
	emails := []email.Email{{Addr: "info@x.com"}, {Addr: "eng@x.com"}}
	result := email.FilterRole(emails)
	assert.Len(t, result, 1)
	assert.Equal(t, "eng@x.com", result[0].Addr)
}

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

func TestDeduplicate_Empty(t *testing.T) {
	assert.Empty(t, email.Deduplicate(nil))
}

func TestDeduplicate_AllSame(t *testing.T) {
	emails := []email.Email{{Addr: "a@x.com"}, {Addr: "a@x.com"}, {Addr: "a@x.com"}}
	result := email.Deduplicate(emails)
	assert.Len(t, result, 1)
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
	assert.True(t, v.Verify(bg, "anything@example.com"))
}

func TestMXVerifier_InvalidAddr(t *testing.T) {
	v := email.NewMXVerifier()
	assert.False(t, v.Verify(bg, "notanemail"))
}

func TestDomainFrom(t *testing.T) {
	assert.Equal(t, "example.com", email.DomainFrom("user@example.com"))
	assert.Equal(t, "", email.DomainFrom("bogus"))
}

func TestIsRole(t *testing.T) {
	assert.True(t, email.IsRole("info@x.com"))
	assert.True(t, email.IsRole("noreply@x.com"))
	assert.False(t, email.IsRole("eng@x.com"))
}
