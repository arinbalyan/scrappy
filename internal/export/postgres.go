package export

import (
	"database/sql"

	"github.com/arinbalyan/scrappy/internal/types"
)

type Postgres struct {
	db   *sql.DB
	stmt *sql.Stmt
}

func NewPostgres() *Postgres { return &Postgres{} }

func (p *Postgres) Name() string { return "postgres" }

func (p *Postgres) Open(url string) error {
	db, err := sql.Open("pgx", url)
	if err != nil {
		return err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return err
	}
	p.db = db

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS jobs (
		id TEXT PRIMARY KEY,
		title TEXT, company TEXT, location TEXT, description TEXT,
		url TEXT UNIQUE, posted_at TIMESTAMPTZ, source TEXT,
		job_type TEXT, salary TEXT, salary_min DOUBLE PRECISION,
		salary_max DOUBLE PRECISION, salary_period TEXT, currency TEXT,
		easy_apply BOOLEAN, remote BOOLEAN,
		company_url TEXT, industry TEXT
	)`)
	if err != nil {
		_ = db.Close()
		return err
	}

	p.stmt, err = db.Prepare(`INSERT INTO jobs (id,title,company,location,description,url,posted_at,source,job_type,salary,salary_min,salary_max,salary_period,currency,easy_apply,remote,company_url,industry)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)
		ON CONFLICT (url) DO NOTHING`)
	return err
}

func (p *Postgres) WriteJob(j types.JobPosting) error {
	var ea, rm interface{}
	if j.EasyApply != nil {
		ea = *j.EasyApply
	}
	if j.Remote != nil {
		rm = *j.Remote
	}
	_, err := p.stmt.Exec(
		j.ID, j.Title, j.Company, j.Location, j.Description, j.URL,
		j.PostedAt, j.Source, j.JobType, j.Salary,
		j.SalaryMin, j.SalaryMax, j.SalaryPeriod, j.Currency,
		ea, rm, j.CompanyURL, j.Industry,
	)
	return err
}

func (p *Postgres) Close() error {
	var err error
	if p.stmt != nil {
		err = p.stmt.Close()
	}
	if p.db != nil {
		if e := p.db.Close(); e != nil {
			err = e
		}
	}
	return err
}
