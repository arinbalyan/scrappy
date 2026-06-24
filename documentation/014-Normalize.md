# Data Normalization

scrappy normalizes job data before export to ensure consistent, comparable output across 141 sites.

---

## Salary normalization — `internal/normalize/salary.go`

### `AnnualizeCompensation(c *model.Compensation) *model.Compensation`

Converts all compensation intervals to yearly equivalents. Preserves currency and scales amounts:

| Input interval | Multiplier | Example |
|----------------|-----------|---------|
| `yearly` (or empty) | 1× | $100,000 → $100,000 |
| `monthly` | 12× | $8,000 → $96,000 |
| `weekly` | 52× | $2,000 → $104,000 |
| `daily` | 260× | $400 → $104,000 |
| `hourly` | 2,080× | $50 → $104,000 |

**Assumptions:**
- Daily: 260 working days/year (5 days × 52 weeks).
- Hourly: 2,080 hours/year (40 hours × 52 weeks).
- Weekly: 52 weeks/year.
- Monthly: 12 months/year.

```go
func AnnualizeCompensation(c *model.Compensation) *model.Compensation {
    if c == nil {
        return nil
    }
    multiplier := 1.0
    switch c.Interval {
    case model.IntervalYearly, "":
        multiplier = 1
    case model.IntervalMonthly:
        multiplier = 12
    case model.IntervalWeekly:
        multiplier = 52
    case model.IntervalDaily:
        multiplier = 260
    case model.IntervalHourly:
        multiplier = 2080
    }
    // ... multiply MinAmount and MaxAmount by multiplier
}
```

The normalization is **only applied when `--enforce-annual-salary`** is set. Without the flag, compensation is stored in its original interval.

When applied, the interval is set to `yearly` and default currency is `USD` if not specified.

---

## Date formatting — `internal/export/helpers.go`

Dates are formatted to RFC 3339 in all export formats:

```go
func formatDate(t *time.Time) string {
    if t == nil || t.IsZero() {
        return ""
    }
    return t.UTC().Format(time.RFC3339)
}
```

Example: `2026-06-24T15:04:05Z`

Nil or zero-value dates produce an empty string.

---

## Location formatting — `internal/model/types.go`

Locations are assembled from three components:

```go
type Location struct {
    City    string
    State   string
    Country string
}

func (l Location) Display() string {
    parts := make([]string, 0, 3)
    if l.City != "" {
        parts = append(parts, l.City)
    }
    if l.State != "" {
        parts = append(parts, l.State)
    }
    if l.Country != "" {
        parts = append(parts, l.Country)
    }
    return strings.Join(parts, ", ")
}
```

Examples:
- `{City: "San Francisco", State: "CA"}` → `"San Francisco, CA"`
- `{City: "London", Country: "UK"}` → `"London, UK"`
- `{}` → `""`

---

## HTML stripping

Job descriptions are stripped of HTML before export using `util.StripHTML`:

```go
jobs[i].Description = util.StripHTML(jobs[i].Description)
jobs[i].CompanyDescription = util.StripHTML(jobs[i].CompanyDescription)
```

The raw HTML is saved to variables before stripping so `ExtractFromHTML` can parse `mailto:` links and text-based email patterns from the original markup.

---

## Text cleanup

1. **Description trimming**: HTML stripped, entities decoded.
2. **Skills normalization**: `nil` skills slices are set to `[]string{}` to avoid JSON serialization issues:

    ```go
    func normalizeJobPost(j *model.JobPost) {
        if j.Skills == nil {
            j.Skills = []string{}
        }
    }
    ```

3. **Email normalization** (handled in `internal/email/extract.go`):

    - Lowercased: `John.Doe@Acme.Com` → `john.doe@acme.com`.
    - Whitespace trimmed.
    - HTML entities decoded: `&#64;` → `@`, `&#46;` → `.`.
    - Obfuscation reversed: `john [at] acme [dot] com` → `john@acme.com`.

---

## Email address normalization — `internal/email/extract.go`

```go
func normalizeAddr(raw string) string {
    return strings.ToLower(strings.TrimSpace(raw))
}
```

Used as the canonical form for dedup keys and storage.

Role address detection:

```go
var rolePrefixes = map[string]bool{
    "info": true, "admin": true, "support": true, "contact": true,
    "sales": true, "hello": true, "careers": true, "hr": true,
    "recruiting": true, "noreply": true, "help": true, ...
}
```

Role-based addresses (`info@`, `admin@`, etc.) are marked with `Role: true` but not filtered out — the caller decides how to weight them.

---

## Summary of normalization pipeline order (in engine)

1. `normalizeJobPost()` — ensure nil slices
2. `util.StripHTML()` — strip HTML from description
3. `internalemail.ExtractFromHTML()` — run on raw HTML before strip
4. `internalemail.Extract()` — run on stripped text
5. `AnnualizeCompensation()` — if `--enforce-annual-salary`
6. All stored at model layer; formatting applied at export time
