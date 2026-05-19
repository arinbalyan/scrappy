package quality

import (
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
)

func TestScore_HasSalary(t *testing.T) {
	min := 100000.0
	job := &model.JobPost{
		Compensation: &model.Compensation{MinAmount: &min},
	}
	if got := Score(job); got < 30 {
		t.Errorf("expected score >= 30 (has salary), got %d", got)
	}
}

func TestScore_DirectApply(t *testing.T) {
	job := &model.JobPost{ApplyMethod: "easy_apply"}
	if got := Score(job); got < 20 {
		t.Errorf("expected score >= 20 (direct apply), got %d", got)
	}
}

func TestScore_NoDirectApplyForGenericURL(t *testing.T) {
	job := &model.JobPost{JobURL: "https://example.com/jobs/123"}
	if got := Score(job); got >= 20 {
		t.Errorf("expected score < 20 for generic URL without apply method, got %d", got)
	}
}

func TestScore_Fresh(t *testing.T) {
	now := time.Now()
	job := &model.JobPost{
		DatePosted: &now,
	}
	if got := Score(job); got < 15 {
		t.Errorf("expected score >= 15 (fresh), got %d", got)
	}
}

func TestScore_LongDescription(t *testing.T) {
	longDesc := make([]byte, 250)
	for i := range longDesc {
		longDesc[i] = 'a'
	}
	agency := &model.JobPost{CompanyName: "Randstad Staffing"}
	if got := Score(agency); got != 0 {
		t.Errorf("expected score 0 without long description and agency penalty, got %d", got)
	}
	agency.Description = string(longDesc)
	if got := Score(agency); got != 10 {
		t.Errorf("expected score 10 with long description, got %d", got)
	}
}

func TestScore_NotAgency(t *testing.T) {
	job := &model.JobPost{
		CompanyName: "Real Company",
	}
	if got := Score(job); got < 10 {
		t.Errorf("expected score >= 10 (not agency), got %d", got)
	}
}

func TestScore_Agency(t *testing.T) {
	job := &model.JobPost{
		CompanyName: "Randstad Staffing",
	}
	score := Score(job)
	if score >= 10 {
		t.Errorf("expected score < 10 (is agency), got %d", score)
	}
}

func TestScore_AgencyByDomain(t *testing.T) {
	job := &model.JobPost{Domain: "jobs.randstad.com"}
	if got := Score(job); got >= 10 {
		t.Errorf("expected score < 10 for agency domain, got %d", got)
	}
}

func TestScore_NotAgencyBySubstring(t *testing.T) {
	job := &model.JobPost{CompanyName: "Stafford Logistics"}
	if got := Score(job); got < 10 {
		t.Errorf("expected score >= 10 for non-agency substring case, got %d", got)
	}
}

func TestScore_EmailMatchesDomain(t *testing.T) {
	job := &model.JobPost{
		Emails: []model.Email{{Addr: "contact@realcompany.com"}},
		Domain: "realcompany.com",
	}
	if got := Score(job); got < 15 {
		t.Errorf("expected score >= 15 (email matches domain), got %d", got)
	}
}
