package export

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"

	"github.com/arinbalyan/scrappy/internal/model"
)

// WriteCSVEmailsOnly writes one row per email address across all jobs.
// Useful for outreach workflows where each email needs independent handling.
func WriteCSVEmailsOnly(path string, jobs []model.JobPost) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create csv file: %w", err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	headers := []string{"email", "verified", "source", "role", "site", "job_id", "title", "company_name", "job_url"}
	if err := w.Write(headers); err != nil {
		return fmt.Errorf("write csv headers: %w", err)
	}

	seen := map[string]struct{}{}
	for _, job := range jobs {
		for _, e := range job.Emails {
			if e.Addr == "" {
				continue
			}
			key := e.Addr + "|" + job.ID
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}

			row := []string{
				e.Addr,
				strconv.FormatBool(e.Verified),
				e.Source,
				strconv.FormatBool(e.Role),
				job.Site,
				job.ID,
				job.Title,
				job.CompanyName,
				job.JobURL,
			}
			if err := w.Write(row); err != nil {
				return fmt.Errorf("write csv row: %w", err)
			}
		}
	}
	if err := w.Error(); err != nil {
		return fmt.Errorf("flush csv writer: %w", err)
	}
	return nil
}
