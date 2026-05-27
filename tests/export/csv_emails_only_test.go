package export_test

import (
	"os"
	"strings"
	"testing"

	"github.com/arinbalyan/scrappy/internal/export"
	"github.com/arinbalyan/scrappy/internal/model"
)

func TestWriteCSVEmailsOnly(t *testing.T) {
	tmp := t.TempDir() + "/emails.csv"
	jobs := []model.JobPost{
		{
			ID: "j1", Title: "Engineer", CompanyName: "Acme", Site: "indeed", JobURL: "https://x/1",
			Emails: []model.Email{{Addr: "a@acme.com", Verified: true, Source: "description"}},
		},
		{
			ID: "j2", Title: "Developer", CompanyName: "Beta", Site: "linkedin", JobURL: "https://x/2",
			Emails: []model.Email{{Addr: "b@beta.com", Verified: false, Source: "company_page"}},
		},
	}
	if err := export.WriteCSVEmailsOnly(tmp, jobs); err != nil {
		t.Fatalf("WriteCSVEmailsOnly failed: %v", err)
	}
	b, err := os.ReadFile(tmp)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	s := string(b)
	if !strings.Contains(s, "email,verified,source,role,site,job_id,title,company_name,job_url") {
		t.Fatalf("missing headers: %s", s)
	}
	if !strings.Contains(s, "a@acme.com,true,description,false,indeed,j1") {
		t.Fatalf("missing row for a@acme.com: %s", s)
	}
	if !strings.Contains(s, "b@beta.com,false,company_page,false,linkedin,j2") {
		t.Fatalf("missing row for b@beta.com: %s", s)
	}
}
