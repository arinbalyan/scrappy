package quality_test

import (
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	qualitypkg "github.com/arinbalyan/scrappy/internal/quality"
)

func TestScore_HasSalary(t *testing.T) {
	min := 100000.0
	job := &model.JobPost{Compensation: &model.Compensation{MinAmount: &min}}
	if got := qualitypkg.Score(job); got < 30 {
		t.Fatalf("expected score >= 30, got %d", got)
	}
}

func TestScore_DirectApply(t *testing.T) {
	job := &model.JobPost{ApplyMethod: "easy_apply"}
	if got := qualitypkg.Score(job); got < 20 {
		t.Fatalf("expected score >= 20, got %d", got)
	}
}

func TestScore_Fresh(t *testing.T) {
	now := time.Now()
	job := &model.JobPost{DatePosted: &now}
	if got := qualitypkg.Score(job); got < 15 {
		t.Fatalf("expected score >= 15, got %d", got)
	}
}

func TestScore_EmailMatchesDomain(t *testing.T) {
	job := &model.JobPost{Emails: []model.Email{{Addr: "contact@realcompany.com"}}, Domain: "realcompany.com"}
	if got := qualitypkg.Score(job); got < 15 {
		t.Fatalf("expected score >= 15, got %d", got)
	}
}
