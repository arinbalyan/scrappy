package export

import (
	"fmt"
	"strings"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/xitongsys/parquet-go-source/local"
	"github.com/xitongsys/parquet-go/parquet"
	"github.com/xitongsys/parquet-go/writer"
)

type parquetJobRow struct {
	Site               string `parquet:"name=site, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	Title              string `parquet:"name=title, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	CompanyName        string `parquet:"name=company_name, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	Location           string `parquet:"name=location, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	IsRemote           bool   `parquet:"name=is_remote, type=BOOLEAN"`
	JobType            string `parquet:"name=job_type, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	DatePosted         string `parquet:"name=date_posted, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	Description        string `parquet:"name=description, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	JobURL             string `parquet:"name=job_url, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	Emails             string `parquet:"name=emails, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	EmailsVerified     string `parquet:"name=emails_verified, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	EmailSource        string `parquet:"name=email_source, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	ApplyMethod        string `parquet:"name=apply_method, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	Seniority          string `parquet:"name=seniority, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	Department         string `parquet:"name=department, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	CompanyURL         string `parquet:"name=company_url, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	JobURLDirect       string `parquet:"name=job_url_direct, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	CompanyIndustry    string `parquet:"name=company_industry, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	CompanyLogo        string `parquet:"name=company_logo, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	CompanyRevenue     string `parquet:"name=company_revenue, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	CompanyNumEmployees string `parquet:"name=company_num_employees, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	CompanyAddresses   string `parquet:"name=company_addresses, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	CompanyDescription string `parquet:"name=company_description, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	Skills             string `parquet:"name=skills, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	ExperienceRange    string `parquet:"name=experience_range, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	CompanyRating      string `parquet:"name=company_rating, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	CompanyReviewsCount int64 `parquet:"name=company_reviews_count, type=INT64"`
	VacancyCount       int64  `parquet:"name=vacancy_count, type=INT64"`
	WorkFromHomeType   string `parquet:"name=work_from_home_type, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	QualityScore       int64  `parquet:"name=quality_score, type=INT64"`
}

func WriteParquet(path string, jobs []model.JobPost) error {
	fw, err := local.NewLocalFileWriter(path)
	if err != nil {
		return fmt.Errorf("create parquet file: %w", err)
	}
	defer fw.Close()

	pw, err := writer.NewParquetWriter(fw, new(parquetJobRow), 4)
	if err != nil {
		return fmt.Errorf("create parquet writer: %w", err)
	}
	defer pw.WriteStop()

	pw.RowGroupSize = 128 * 1024 * 1024
	pw.CompressionType = parquet.CompressionCodec_SNAPPY

	for _, job := range jobs {
		row := parquetJobRow{
			Site:               "",
			Title:              job.Title,
			CompanyName:        job.CompanyName,
			Location:           job.Location.Display(),
			IsRemote:           job.IsRemote,
			JobType:            job.JobType,
			DatePosted:         formatDate(job.DatePosted),
			Description:        job.Description,
			JobURL:             job.JobURL,
			Emails:             joinEmails(job.Emails),
			EmailsVerified:     joinEmailVerified(job.Emails),
			EmailSource:        joinEmailSources(job.Emails),
			ApplyMethod:        job.ApplyMethod,
			Seniority:          job.Seniority,
			Department:         job.Department,
			CompanyURL:         job.CompanyURL,
			JobURLDirect:       job.JobURLDirect,
			CompanyIndustry:    job.CompanyIndustry,
			CompanyLogo:        job.CompanyLogo,
			CompanyRevenue:     job.CompanyRevenue,
			CompanyNumEmployees: job.CompanyNumEmployees,
			CompanyAddresses:   job.CompanyAddresses,
			CompanyDescription: job.CompanyDescription,
			Skills:             strings.Join(job.Skills, ";"),
			ExperienceRange:    job.ExperienceRange,
			CompanyRating:      formatRating(job.CompanyRating),
			CompanyReviewsCount: int64(job.CompanyReviews),
			VacancyCount:       int64(job.VacancyCount),
			WorkFromHomeType:   job.WorkFromHome,
			QualityScore:       int64(job.QualityScore),
		}

		if err := pw.Write(row); err != nil {
			return fmt.Errorf("write parquet row: %w", err)
		}
	}

	if err := pw.WriteStop(); err != nil {
		return fmt.Errorf("finalize parquet file: %w", err)
	}
	return nil
}
