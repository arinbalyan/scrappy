package export_test

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	exportpkg "github.com/arinbalyan/scrappy/internal/export"
	"github.com/arinbalyan/scrappy/internal/model"
)

func TestWriteJSONL(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "jobs.jsonl")
	jobs := []model.JobPost{
		{Title: "Engineer", JobURL: "https://example.com/job/1", Emails: []model.Email{{Addr: "a@example.com"}, {Addr: "b@example.com"}}},
		{Title: "Analyst", JobURL: "https://example.com/job/2"},
	}

	if err := exportpkg.WriteJSONL(out, jobs); err != nil {
		t.Fatalf("write jsonl: %v", err)
	}

	f, err := os.Open(out)
	if err != nil {
		t.Fatalf("open jsonl: %v", err)
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	count := 0
	for s.Scan() {
		count++
		var jp model.JobPost
		if err := json.Unmarshal(s.Bytes(), &jp); err != nil {
			t.Fatalf("invalid jsonl line: %v", err)
		}
		if count == 1 && len(jp.Emails) != 2 {
			t.Fatalf("expected first row to keep 2 emails, got %d", len(jp.Emails))
		}
	}
	if err := s.Err(); err != nil {
		t.Fatalf("scan jsonl: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 rows, got %d", count)
	}
}
