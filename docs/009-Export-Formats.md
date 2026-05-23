# Export Formats

`internal/export/` -- four output writers: JSONL, CSV, XLSX, Parquet. All write to a local file path specified by `--out`.

## Format overview

| Format | CLI value | Default | Best for |
|--------|-----------|---------|----------|
| JSONL | `jsonl` | **Yes** | Pipelines, streaming, programmatic consumption |
| CSV | `csv` | No | Spreadsheets, quick review, email marketing |
| XLSX | `xlsx` | No | Excel users, formatted multi-sheet output |
| Parquet | `parquet` | No | Big-data analysis, columnar compression |

## JSONL

Line-delimited JSON -- one `JobPost` object per line. Default format when `--format` is not set.

```
{"site":"indeed","title":"Software Engineer","company_name":"Acme",...}
{"site":"remoteok","title":"DevOps Engineer","company_name":"Beta",...}
```

```bash
scrappy --sites indeed,remoteok --search "golang" --out jobs.jsonl
```

- Streaming writer -- O(n) memory, one row at a time
- Preserves all nested structs (Compensation, Location, Emails)
- Easy to pipe into `jq` or ingest into data pipelines

## CSV

Flat table, one row per job. Nested fields (Emails, skills) are semicolon-joined.

```bash
scrappy --sites indeed --search "rust" --format csv --out jobs.csv
```

### Column schema (30 columns)

```
site, title, company_name, location, is_remote, job_type, date_posted,
description, job_url, emails, emails_verified, email_source, apply_method,
seniority, department, company_url, job_url_direct, company_industry,
company_logo, company_revenue, company_num_employees, company_addresses,
company_description, skills, experience_range, company_rating,
company_reviews_count, vacancy_count, work_from_home_type, quality_score
```

- Array fields (emails, skills) are semicolon-delimited
- `emails_verified` and `email_source` parallel the `emails` column
- `quality_score` appended at the end

### Emails-only CSV

When you only need contact data, use `--email` to filter jobs with no email addresses:

```bash
scrappy --sites linkedin --search "engineer" --format csv \
  --out contacts.csv --email
```

## XLSX

Excel Open XML format. Uses the same 30-column schema as CSV, one sheet named `jobs`.

```bash
scrappy --sites glassdoor --search "developer" --format xlsx --out jobs.xlsx
```

- ~1M row limit per sheet (Excel constraint)
- Supports formatted columns when opened in Excel/Google Sheets
- Uses `github.com/xuri/excelize/v2`

## Parquet

Columnar storage with Snappy compression. Ideal for analytical workloads.

```bash
scrappy --sites linkedin,indeed --search "data engineer" \
  --format parquet --out jobs.parquet
```

- Snappy compression -- 5-10x smaller than JSONL on text-heavy data
- Dictionary encoding on string columns (site, title, company_name)
- Row group size: 128 MB
- 4 concurrent row groups for balanced read/write

### Parquet schema

```go
type parquetJobRow struct {
    Site                string  // PLAIN_DICTIONARY encoding
    Title               string
    CompanyName         string
    Location            string
    IsRemote            bool
    JobType             string
    DatePosted          string  // RFC3339
    Description         string
    JobURL              string
    Emails              string  // semicolon-joined
    EmailsVerified      string
    EmailSource         string
    ApplyMethod         string
    Seniority           string
    Department          string
    CompanyURL          string
    JobURLDirect        string
    CompanyIndustry     string
    CompanyLogo         string
    CompanyRevenue      string
    CompanyNumEmployees string
    CompanyAddresses    string
    CompanyDescription  string
    Skills              string
    ExperienceRange     string
    CompanyRating       string
    CompanyReviewsCount int64
    VacancyCount        int64
    WorkFromHomeType    string
    QualityScore        int64
}
```

## Description format

Control how descriptions are rendered with `--description-format`:

| Value | Behavior |
|-------|----------|
| `plain` | HTML stripped, plain text (default) |
| `markdown` | HTML stripped, minimal markdown preserved |
| `html` | Raw HTML kept as-is |

The engine always strips HTML tags before output (via `golang.org/x/net/html` tokenizer). The format flag controls additional whitespace and structure normalization.

## Performance

| Format | Write speed (1k jobs) | File size (1k jobs) | Memory |
|--------|----------------------|---------------------|--------|
| JSONL | ~5 ms | ~800 KB | O(n) rows |
| CSV | ~4 ms | ~750 KB | O(columns) |
| XLSX | ~50 ms | ~1.2 MB | O(rows) in memory |
| Parquet | ~60 ms | ~120 KB | O(row group) |

Parquet is the most space-efficient but slower to write due to columnar encoding. JSONL is the fastest and most pipeline-friendly.
