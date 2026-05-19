package export

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/xuri/excelize/v2"
)

func WriteXLSX(path string, jobs []model.JobPost) error {
	f := excelize.NewFile()
	sheet := "jobs"
	f.SetSheetName("Sheet1", sheet)

	headers := []string{
		"site", "title", "company_name", "location", "is_remote", "job_type", "date_posted",
		"description", "job_url", "emails", "emails_verified", "email_source", "apply_method",
		"seniority", "department", "company_url", "job_url_direct", "company_industry",
		"company_logo", "company_revenue", "company_num_employees", "company_addresses",
		"company_description", "skills", "experience_range", "company_rating",
		"company_reviews_count", "vacancy_count", "work_from_home_type", "quality_score",
	}

	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(sheet, cell, h); err != nil {
			return fmt.Errorf("set xlsx header cell: %w", err)
		}
	}

	for idx, job := range jobs {
		row := []string{
			"",
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
		}

		for col, val := range row {
			cell, _ := excelize.CoordinatesToCellName(col+1, idx+2)
			if err := f.SetCellValue(sheet, cell, val); err != nil {
				return fmt.Errorf("set xlsx row cell: %w", err)
			}
		}
	}

	if err := f.SaveAs(path); err != nil {
		return fmt.Errorf("save xlsx file: %w", err)
	}
	return nil
}
