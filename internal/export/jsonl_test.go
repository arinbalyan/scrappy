package export

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
)

func TestWriteJSONL(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "jobs.jsonl")
	jobs := []model.JobPost{{Title: "Engineer", JobURL: "https://example.com/job/1"}, {Title: "Analyst", JobURL: "https://example.com/job/2"}}

	if err := WriteJSONL(out, jobs); err != nil {
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
	}
	if err := s.Err(); err != nil {
		t.Fatalf("scan jsonl: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 rows, got %d", count)
	}
}
