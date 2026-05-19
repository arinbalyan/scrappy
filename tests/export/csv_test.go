package export_test

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	exportpkg "github.com/arinbalyan/scrappy/internal/export"
	"github.com/arinbalyan/scrappy/internal/model"
)

func TestWriteCSV(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "jobs.csv")
	now := time.Now()
	rating := 4.2

	jobs := []model.JobPost{{
		Title:         "Software Engineer",
		CompanyName:   "Acme",
		Location:      model.Location{City: "SF", State: "CA", Country: "USA"},
		IsRemote:      true,
		JobType:       "fulltime",
		DatePosted:    &now,
		Description:   "great role",
		JobURL:        "https://example.com/job/1",
		Emails:        []model.Email{{Addr: "hr@acme.com", Verified: true, Source: "description"}},
		ApplyMethod:   "easy_apply",
		Seniority:     "senior",
		Department:    "engineering",
		CompanyURL:    "https://acme.com",
		CompanyRating: &rating,
		QualityScore:  75,
	}}

	if err := exportpkg.WriteCSV(out, jobs); err != nil {
		t.Fatalf("write csv: %v", err)
	}

	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}
	content := string(b)
	if !strings.Contains(content, "Software Engineer") || !strings.Contains(content, "hr@acme.com") {
		t.Fatalf("unexpected csv content: %s", content)
	}

	f, err := os.Open(out)
	if err != nil {
		t.Fatalf("open csv: %v", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("parse csv: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 csv rows (header + one data row), got %d", len(records))
	}
	if len(records[0]) != len(records[1]) {
		t.Fatalf("header/data column mismatch: %d vs %d", len(records[0]), len(records[1]))
	}
}
