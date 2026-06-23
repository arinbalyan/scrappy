package export

import (
	"encoding/csv"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/arinbalyan/scrappy/internal/types"
)

type CSV struct {
	mu sync.Mutex
	w  *csv.Writer
	f  *os.File
}

func NewCSV() *CSV { return &CSV{} }

func (c *CSV) Name() string { return "csv" }

func (c *CSV) Open(path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	c.f = f
	c.w = csv.NewWriter(f)
	return c.w.Write([]string{
		"id", "title", "company", "location", "description", "url",
		"posted_at", "source", "job_type", "salary", "salary_min",
		"salary_max", "salary_period", "currency", "easy_apply", "remote",
		"company_url", "industry",
	})
}

func (c *CSV) WriteJob(j types.JobPosting) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var posted string
	if j.PostedAt != nil {
		posted = j.PostedAt.Format(time.RFC3339)
	}

	return c.w.Write([]string{
		j.ID, j.Title, j.Company, j.Location,
		j.Description, j.URL, posted, j.Source,
		j.JobType, j.Salary,
		f64(j.SalaryMin), f64(j.SalaryMax), j.SalaryPeriod, j.Currency,
		b(j.EasyApply), b(j.Remote), j.CompanyURL, j.Industry,
	})
}

func (c *CSV) Close() error {
	c.w.Flush()
	if c.f != nil {
		return c.f.Close()
	}
	return nil
}

func f64(v *float64) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%.2f", *v)
}

func b(v *bool) string {
	if v == nil {
		return ""
	}
	if *v {
		return "true"
	}
	return "false"
}
