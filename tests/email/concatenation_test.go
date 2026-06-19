package email_test

import (
	"testing"

	"github.com/arinbalyan/scrappy/internal/email"
)

// This test reproduces the concatenation bug reported in the email
// field extraction issue. Each case is an email-like string that
// should NOT be extracted because it contains garbage suffixes.
func TestExtract_ConcatenationBug_ReportedExamples(t *testing.T) {
	tests := []struct {
		text   string
		name   string
		want   int    // expected number of extracted emails
		wantOK string // if needAddr>0, the valid address to expect
	}{
		// ── No separator (domain + suffix without dot) ──
		{"contact jimlandry@pratikopratik.comimportant", "no-dot .important suffix", 0, ""},
		{"reach dr.saxena@saxenacarerecovery.comwe", "no-dot .we suffix", 0, ""},
		{"email truth@experience-pt.comlet", "no-dot .let suffix", 0, ""},
		{"apply hr@revolutionfield.comto", "no-dot .to suffix", 0, ""},
		{"careers@streetdreamscny.comcall", "no-dot .call suffix", 0, ""},
		{"support@mercor.comps", "no-dot .comps suffix", 0, ""},
		{"humanresources@vulcanelectric.comjob", "no-dot .job suffix", 0, ""},

		// ── With separator (domain + .suffix) — the fix recovers the real email ──
		{"cory.bickham@thelakesbehavioralhealth.com.pay", "dot .pay suffix, recovered", 1, "cory.bickham@thelakesbehavioralhealth.com"},
		{"operations@merrimack-services.com.equal", "dot .equal suffix, recovered", 1, "operations@merrimack-services.com"},
		{"info@oer.ny.gov.background", "dot .background suffix, recovered", 1, "info@oer.ny.gov"},
		{"employment@townofmccandless.org.review", "dot .review suffix, recovered", 1, "employment@townofmccandless.org"},
		{"office@seattlenac.com.seattle", "dot .seattle suffix, recovered", 1, "office@seattlenac.com"},
		{"jobs@lni.wa.gov.diversity", "dot .diversity suffix, recovered", 1, "jobs@lni.wa.gov"},

		// ── Person names appended ──
		{"denise.tan@celecti.com.sg.denise", "dot .denise (name), ex: denise.tan@celecti.com.sg", 1, "denise.tan@celecti.com.sg"},
		{"kobusoffice@kohlbrat.com.kobus", "dot .kobus suffix, recovered", 1, "kobusoffice@kohlbrat.com"},
		{"melissa@recruitexpress.com.sgmelissa", "no-dot sgmelissa, recovered", 1, "melissa@recruitexpress.com"},

		// ── Partial field truncation ──
		{"nini@beautydrop.us.subject", "dot .subject, recovered", 1, "nini@beautydrop.us"},
		{"info@bridgtonschoolhouse.org.job", "dot .job suffix, recovered", 1, "info@bridgtonschoolhouse.org"},
		{"info@bridgtonschoolhouse.org.job", "dot .job suffix, recovered", 1, "info@bridgtonschoolhouse.org"},
		{"careers@techstartup.io.position", "dot .position, recovered", 1, "careers@techstartup.io"},
		{"jobs@company.com.phone", "dot .phone, recovered", 1, "jobs@company.com"},
		{"hr@acme.org.limited", "dot .limited, recovered", 1, "hr@acme.org"},
		{"contact@startup.com.please", "dot .please, recovered", 1, "contact@startup.com"},
		{"team@corp.net.north", "dot .north, recovered", 1, "team@corp.net"},
		{"hello@company.io.multi", "dot .multi, recovered", 1, "hello@company.io"},

		// ── Dangling parsed text ──
		{"user@domain.comcall", "no-dot call suffix", 0, ""},
		{"someone@somecorp.comother", "no-dot other suffix", 0, ""},
		{"contact@website.comhead", "no-dot head suffix", 0, ""},

		// ── Valid emails that SHOULD be extracted (control cases) ──
		{"Contact us at hiring@acme.com", "valid control", 1, "hiring@acme.com"},
		{"jobs@company.org today", "valid control space", 1, "jobs@company.org"},
		{"hr@domain.co.uk nice", "valid .co.uk", 1, "hr@domain.co.uk"},
		{"info@mycompany.io", "valid .io", 1, "info@mycompany.io"},
		{"team@startup.ai", "valid .ai", 1, "team@startup.ai"},
		{"careers@company.dev", "valid .dev", 1, "careers@company.dev"},
		{"contact@firm.blog", "valid .blog", 1, "contact@firm.blog"},
		// Compound TLDs that are valid
		{"user@example.com.au", "valid .com.au", 1, "user@example.com.au"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := email.Extract(tt.text)
			if len(got) != tt.want {
				t.Errorf("Extract(%q) = %d emails (want %d). Extracted: %v",
					tt.text, len(got), tt.want, extractAddrs(got))
			}
			if tt.want == 1 && tt.wantOK != "" && len(got) == 1 {
				if got[0].Addr != tt.wantOK {
					t.Errorf("Extract(%q) addr = %q (want %q)",
						tt.text, got[0].Addr, tt.wantOK)
				}
			}
		})
	}
}

func extractAddrs(emails []email.Email) []string {
	out := make([]string, len(emails))
	for i, e := range emails {
		out[i] = e.Addr
	}
	return out
}
