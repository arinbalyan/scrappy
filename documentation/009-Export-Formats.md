# Export Formats

scrappy supports four export formats. The format is selected via `--format` and output goes to `--out` (or stdout for JSONL).

---

## 1. JSONL — `internal/export/jsonl.go`

**Default format.** One JSON object per line, each representing a complete `JobPost`.

```bash
scrappy --sites remoteok --search "golang" --format jsonl --out jobs.jsonl
```

```json
{
  "id": "remoteok-123",
  "title": "Go Developer",
  "company_name": "Acme Corp",
  "job_url": "https://remoteok.com/...",
  "site": "remoteok",
  "emails": [{"addr": "jobs@acme.com", "verified": true, "source": "company_page", "role": false}],
  "quality_score": 72,
  "compensation": {"interval": "yearly", "min_amount": 120000, "max_amount": 180000, "currency": "USD"}
}
```

**Schema:** Every `JobPost` field serialized as-is via `encoding/json`. Suitable for streaming ingestion into data pipelines, BigQuery, or Elasticsearch.

---

## 2. CSV — `internal/export/csv.go`

Standard flat table with 34 columns. One row per job.

```bash
scrappy --sites linkedin --search "engineer" --format csv --out jobs.csv
```

**Columns:**

| Column | Description |
|--------|-------------|
| `site` | Source site name |
| `title` | Job title |
| `company_name` | Employer name |
| `location` | City, State, Country (formatted) |
| `is_remote` | true/false |
| `job_type` | fulltime/parttime/contract/internship |
| `date_posted` | RFC 3339 timestamp |
| `description` | Plain text (HTML stripped) |
| `job_url` | URL on the job board |
| `emails` | Semicolon-separated addresses |
| `emails_verified` | Semicolon-separated boolean strings |
| `email_source` | Semicolon-separated source labels |
| `apply_method` | easy_apply/email/direct_url/external_url |
| `seniority` | entry/mid/senior/lead |
| `department` | eng/data/product/... |
| `company_url` | Company website |
| `job_url_direct` | Direct apply URL |
| `company_industry` | Industry classification |
| `company_logo` | Logo URL |
| `company_revenue` | Revenue string |
| `company_num_employees` | Employee count |
| `company_addresses` | Office locations |
| `company_description` | Company blurb |
| `skills` | Semicolon-separated list |
| `experience_range` | e.g. "3-5 years" |
| `company_rating` | Float rating |
| `company_reviews_count` | Number of reviews |
| `vacancy_count` | Open positions |
| `work_from_home_type` | Remote type label |
| `quality_score` | 0-100 score |
| `salary_interval` | yearly/monthly/weekly/daily/hourly |
| `salary_min` | Minimum salary amount |
| `salary_max` | Maximum salary amount |
| `salary_currency` | ISO currency code |

### CSV Emails-Only — `internal/export/csv_emails_only.go`

A separate variant, activated with `--csv-emails-only`, writes **one row per email address** instead of one row per job:

```bash
scrappy --sites remoteok --search "golang" --format csv --csv-emails-only --out emails.csv
```

**Columns:** `email`, `verified`, `source`, `role`, `site`, `job_id`, `title`, `company_name`, `job_url`

Useful for outreach workflows where each email needs independent handling (CRM imports, mail merge, recruiter contact lists).

Each row is deduplicated by (email, job_id) to avoid repeats from multi-email jobs.

---

## 3. XLSX — `internal/export/xlsx.go`

Excel spreadsheet with a single sheet named `jobs`. Same 34 columns as CSV, written via the [excelize](https://github.com/xuri/excelize) library.

```bash
scrappy --sites indeed --search "developer" --format xlsx --out jobs.xlsx
```

Multi-value fields (emails, skills) are semicolon-delimited strings within single cells. No formulas or formatting — clean tabular data for spreadsheet consumption.

---

## 4. Parquet — `internal/export/parquet.go`

Columnar storage format optimized for analytical workloads. Uses the [parquet-go](https://github.com/xitongsys/parquet-go) library with Snappy compression.

```bash
scrappy --sites linkedin --search "data engineer" --format parquet --out jobs.parquet
```

**Schema:** 34 typed columns (string, boolean, int64). String columns use `PLAIN_DICTIONARY` encoding for efficient storage of repeated values. Row group size is 128 MiB.

| Parquet column | Type |
|----------------|------|
| `site` | UTF8 |
| `title` | UTF8 |
| `company_name` | UTF8 |
| `is_remote` | BOOLEAN |
| `quality_score` | INT64 |
| `company_reviews_count` | INT64 |
| `vacancy_count` | INT64 |
| *(all others)* | UTF8 |

Ideal for loading into Pandas, Spark, or DuckDB for analysis:

```python
import pandas as pd
df = pd.read_parquet("jobs.parquet")
df[df.quality_score > 50].groupby("site").size()
```

---

## Format selection guideline

| Format | Best for |
|--------|----------|
| JSONL | Streaming, pipelines, BigQuery |
| CSV | Spreadsheets, email outreach (with `--csv-emails-only`) |
| XLSX | Excel users, quick visual scan |
| Parquet | Analytical queries, large datasets, Pandas/Spark |
