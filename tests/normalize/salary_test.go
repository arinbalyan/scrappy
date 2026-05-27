package normalize_test

import (
	"testing"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/normalize"
)

func fptr(v float64) *float64 { return &v }

func TestAnnualizeCompensationHourly(t *testing.T) {
	in := &model.Compensation{Interval: model.IntervalHourly, MinAmount: fptr(50), MaxAmount: fptr(80), Currency: "USD"}
	out := normalize.AnnualizeCompensation(in)
	if out.Interval != model.IntervalYearly {
		t.Fatalf("expected yearly interval, got %s", out.Interval)
	}
	if out.MinAmount == nil || *out.MinAmount != 104000 {
		t.Fatalf("expected min 104000, got %v", out.MinAmount)
	}
	if out.MaxAmount == nil || *out.MaxAmount != 166400 {
		t.Fatalf("expected max 166400, got %v", out.MaxAmount)
	}
}

func TestAnnualizeCompensationMonthly(t *testing.T) {
	in := &model.Compensation{Interval: model.IntervalMonthly, MinAmount: fptr(8000), Currency: "EUR"}
	out := normalize.AnnualizeCompensation(in)
	if out.MinAmount == nil || *out.MinAmount != 96000 {
		t.Fatalf("expected min 96000, got %v", out.MinAmount)
	}
	if out.Currency != "EUR" {
		t.Fatalf("expected currency EUR, got %s", out.Currency)
	}
}

func TestAnnualizeCompensationNil(t *testing.T) {
	if out := normalize.AnnualizeCompensation(nil); out != nil {
		t.Fatal("expected nil output for nil input")
	}
}
