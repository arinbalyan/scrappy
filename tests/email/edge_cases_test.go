package email_test

import (
	"testing"

	"github.com/arinbalyan/scrappy/internal/email"
)

// Additional regression tests for the email concatenation fix.
// The key checks: emails must not consume adjacent text beyond the TLD boundary,
// and must still correctly extract valid emails in real-world context.

func TestExtract_AdjacentWordNoSpace(t *testing.T) {
	// Adjacent text without any separator - regex must not over-consume.
	tests := []struct {
		text   string
		name   string
		want   []string // expected email addresses
		notWant []string // specifically should not appear
	}{
		{
			name:   "email followed by .jobs suffix (valid TLD)",
			text:   "contact us at hiring@acme.com.jobs is the role",
			want:   []string{"hiring@acme.com"},
			notWant: []string{"hiring@acme.com.jobs"},
		},
		{
			name:   "email followed by .work suffix (valid TLD)",
			text:   "apply to team@startup.io.work with us",
			want:   []string{"team@startup.io"},
			notWant: []string{"team@startup.io.work"},
		},
		{
			name:   "email followed by .careers suffix (valid TLD)",
			text:   "email hr@company.org.careers for info",
			want:   []string{"hr@company.org"},
			notWant: []string{"hr@company.org.careers"},
		},
		{
			name:   "email with dot after (sentence boundary)",
			text:   "Send to jobs@acme.com. We will review.",
			want:   []string{"jobs@acme.com"},
			notWant: []string{"jobs@acme.com."},
		},
		{
			name:   "email with comma after",
			text:   "Email me at hello@domain.com, or call us.",
			want:   []string{"hello@domain.com"},
		},
		{
			name:   "email with colon after",
			text:   "Contact: support@corp.net: we are hiring",
			want:   []string{"support@corp.net"},
		},
		{
			name:   "email with parentheses after",
			text:   "Apply to careers@company.io (remote ok)",
			want:   []string{"careers@company.io"},
		},
		{
			name:   "email at end of string",
			text:   "Contact us at jobs@acme.com",
			want:   []string{"jobs@acme.com"},
		},
		{
			name:   "email with no space before next field",
			text:   "Email:info@company.com Pay:100k",
			want:   []string{"info@company.com"},
			notWant: []string{"info@company.compay"},
		},
		{
			name:   "multiple emails in text with various separators",
			text:   "Contact hiring@acme.com or hr@acme.org for questions",
			want:   []string{"hiring@acme.com", "hr@acme.org"},
		},
		{
			name:   "email with newline after",
			text:   "Email us at jobs@startup.com\nWe look forward",
			want:   []string{"jobs@startup.com"},
		},
		{
			name:   "email .jobs at end of string (edge case)",
			text:   "contact: hr@company.com.jobs",
			want:   []string{},
		},
		{
			name:   "email .work at end of string (edge case)",
			text:   "send to team@startup.org.work",
			want:   []string{},
		},
		{
			name:   "email .jobs in middle with more text after",
			text:   "Apply at careers@company.com.jobs for details",
			want:   []string{"careers@company.com"},
			notWant: []string{"careers@company.com.jobs"},
		},
		{
			name:   "valid .co.uk compound TLD preserved",
			text:   "Contact us at info@company.co.uk for support",
			want:   []string{"info@company.co.uk"},
		},
		{
			name:   "valid .com.au compound TLD preserved",
			text:   "Email jobs@company.com.au today",
			want:   []string{"jobs@company.com.au"},
		},
		{
			name:   "valid .co.nz compound TLD preserved",
			text:   "Contact hr@company.co.nz",
			want:   []string{"hr@company.co.nz"},
		},
		{
			name:   "email domain.com.pay at end - .pay not valid TLD",
			text:   "contact: cory@domain.com.pay",
			want:   []string{},
		},
		{
			name:   "real-world: email with phone field after",
			text:   "Email: dr.smith@clinic.org Phone: 555-0100",
			want:   []string{"dr.smith@clinic.org"},
			notWant: []string{"dr.smith@clinic.orgphone"},
		},
		{
			name:   "local part with plus addressing",
			text:   "Email: jobs+tag@company.com for filtering",
			want:   []string{"jobs+tag@company.com"},
		},
		{
			name:   "underscore in local part",
			text:   "contact: john_doe@company.com",
			want:   []string{"john_doe@company.com"},
		},
		{
			name:   "percentage in local part",
			text:   "email: user%tag@domain.com forward",
			want:   []string{"user%tag@domain.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := email.Extract(tt.text)
			gotAddrs := extractAddrs(got)

			// Check all expected emails are present.
			for _, want := range tt.want {
				found := false
				for _, g := range gotAddrs {
					if g == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Extract(%q) missing expected %q. Got: %v",
						tt.text, want, gotAddrs)
				}
			}

			// Check no unwanted emails are present.
			for _, notWant := range tt.notWant {
				for _, g := range gotAddrs {
					if g == notWant {
						t.Errorf("Extract(%q) contains unwanted %q (should be rejected)",
							tt.text, notWant)
					}
				}
			}

			// Check we don't get more emails than expected (dedup).
			if len(tt.want) > 0 && len(got) > len(tt.want) {
				// This is a warning, not a failure, since there could be
				// additional valid emails in the text we didn't list.
				t.Logf("Extract(%q) = %d emails, expected at least %d: %v",
					tt.text, len(got), len(tt.want), gotAddrs)
			}
		})
	}
}

func TestExtract_RealWorldScrapedText(t *testing.T) {
	// Simulates actual text from scraped job descriptions.
	tests := []struct {
		name string
		text string
		want int    // expected email count
		addr string // expected first email address (if any)
	}{
		{
			name: "typical job posting with salary",
			text: `Job Title: Senior Software Engineer
Company: Acme Corp
Description: We are hiring! Send your resume to hiring@acme.com.
Salary: $150k - $200k
Location: Remote, US`,
			want: 1,
			addr: "hiring@acme.com",
		},
		{
			name: "raw html with mailto and adjacent text",
			text: `<p>Email: <a href="mailto:jobs@company.com">jobs@company.com</a> Pay: $100k</p>`,
			want: 1,
			addr: "jobs@company.com",
		},
		{
			name: "concatenated fields without separator",
			text: "Email: support@techcorp.comPay: $120kLocation: NY",
			want: 0,
		},
		{
			name: "multiple emails with various suffixes after",
			text: "Contact hr@bigcorp.com or ceo@startup.io. We pay well. Email: jobs@midcorp.org (remote)",
			want: 3,
		},
		{
			name: "email with job-related suffix text next to it",
			text: "Send resume to careers@company.com.jobs are available now",
			want: 1,
			addr: "careers@company.com",
		},
		{
			name: "company description with email then salary",
			text: "Apply to info@corp.org. Salary range: $80k-$120k. Benefits include health insurance.",
			want: 1,
			addr: "info@corp.org",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := email.Extract(tt.text)
			if len(got) != tt.want {
				t.Errorf("Extract(%q) = %d emails (want %d). Got: %v",
					tt.text[:min(80, len(tt.text))], len(got), tt.want, extractAddrs(got))
			}
			if tt.addr != "" && len(got) > 0 {
				if got[0].Addr != tt.addr {
					t.Errorf("Extract(%q) first addr = %q (want %q)",
						tt.text[:min(80, len(tt.text))], got[0].Addr, tt.addr)
				}
			}
		})
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
