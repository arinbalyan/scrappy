package export

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/arinbalyan/scrappy/internal/model"
)

func WriteCSV(path string, jobs []model.JobPost) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create csv file: %w", err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	headers := []string{
		"site", "title", "company_name", "location", "is_remote", "job_type", "date_posted",
		"description", "job_url", "emails", "emails_verified", "email_source", "apply_method",
		"seniority", "department", "company_url", "job_url_direct", "company_industry",
		"company_logo", "company_revenue", "company_num_employees", "company_addresses",
		"company_description", "skills", "experience_range", "company_rating",
		"company_reviews_count", "vacancy_count", "work_from_home_type", "quality_score",
		"salary_interval", "salary_min", "salary_max", "salary_currency",
	}
	if err := w.Write(headers); err != nil {
		return fmt.Errorf("write csv headers: %w", err)
	}

	for _, job := range jobs {
		row := []string{
			string(job.Site),
			job.Title,
			job.CompanyName,
			job.Location.Display(),
			strconv.FormatBool(job.IsRemote),
			job.JobType,
			formatDate(job.DatePosted),
			job.Description,
			job.JobURL,
			joinEmails(job.Emails),
			joinEmailVerified(job.Emails),
			joinEmailSources(job.Emails),
			job.ApplyMethod,
			job.Seniority,
			job.Department,
			job.CompanyURL,
			job.JobURLDirect,
			job.CompanyIndustry,
			job.CompanyLogo,
			job.CompanyRevenue,
			job.CompanyNumEmployees,
			job.CompanyAddresses,
			job.CompanyDescription,
			strings.Join(job.Skills, ";"),
			job.ExperienceRange,
			formatRating(job.CompanyRating),
			strconv.Itoa(job.CompanyReviews),
			strconv.Itoa(job.VacancyCount),
			job.WorkFromHome,
			strconv.Itoa(job.QualityScore),
			formatCompInterval(job.Compensation),
			formatCompMin(job.Compensation),
			formatCompMax(job.Compensation),
			formatCompCurrency(job.Compensation),
		}
		if err := w.Write(row); err != nil {
			return fmt.Errorf("write csv row: %w", err)
		}
	}

	if err := w.Error(); err != nil {
		return fmt.Errorf("flush csv writer: %w", err)
	}

	return nil
}


