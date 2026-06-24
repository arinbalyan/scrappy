# Deduplication

scrappy runs deduplication at two levels: within the engine's scrape loop (global dedup) and via the `DedupFilters` function for post-processing.

---

## Global dedup (engine-level)

In the engine's post-processing pipeline (`pkg/scrappy/engine.go`), every job is deduplicated by URL as it streams back from scraper goroutines:

```go
seenGlobal := make(map[string]struct{})
for res := range resultsCh {
    for i := range res.jobs {
        // ... processing ...
        key := strings.TrimSpace(jobs[i].JobURL)
        if key == "" {
            key = strings.TrimSpace(jobs[i].ID)
        }
        if key == "" {
            continue
        }
        if _, ok := seenGlobal[key]; ok {
            continue  // duplicate — skip
        }
        seenGlobal[key] = struct{}{}
        all = append(all, jobs[i])
    }
}
```

**Key points:**
- Dedup key is `JobURL` first, falling back to `ID`.
- Empty key → job is dropped.
- Runs **during collection**, not after — reduces memory pressure.
- The map is a plain `map[string]struct{}`, no external dependencies.

This is controlled by the `--dedup` flag (default: `true`).

---

## `dedup.Set` — `internal/dedup/dedup.go`

A thread-safe deduplication set keyed by job URL:

```go
type Set struct {
    mu   sync.Mutex
    seen map[string]bool
}
```

- **`Add(url string) bool`**: returns `true` if the URL was newly added (not seen before), `false` if it's a duplicate.
- Thread-safe via `sync.Mutex` — safe for concurrent goroutine access.

---

## `DedupFilters` — `internal/dedup/dedup.go`

Applies URL dedup and, optionally, company dedup to a slice of jobs:

```go
func DedupFilters(jobs []model.JobPost, skipURLDedup bool, companyDedup, dedupNullCompany bool) []model.JobPost
```

**Parameters:**

| Param | Effect |
|-------|--------|
| `skipURLDedup` | When `true`, skip URL dedup. Controlled by `--dedup` flag. |
| `companyDedup` | When `true`, keep only one posting per company. Controlled by `--dedup-by-company` flag. |
| `dedupNullCompany` | When `true`, companies with empty names get a `"null:"` prefix so they are deduplicated *among themselves* (all null-company jobs collapse to one). |

**Logic:**

1. Create a `urlSet` and `companySet`.
2. For each job, check if `JobURL` is already in `urlSet` (unless `skipURLDedup`).
3. If `companyDedup`, check if `CompanyName` is already in `companySet`.
4. Jobs failing either check are dropped from the output slice.

---

## Dedup within a site (engine internals)

Individual scraper results are also deduplicated by URL within the same site before aggregation:

```go
func dedupWithinSite(in []model.JobPost) []model.JobPost {
    seen := map[string]struct{}{}
    out := make([]model.JobPost, 0, len(in))
    for _, j := range in {
        key := strings.TrimSpace(j.JobURL)
        if key == "" {
            key = strings.TrimSpace(j.ID)
        }
        if key == "" {
            out = append(out, j)
            continue
        }
        if _, ok := seen[key]; ok {
            continue
        }
        seen[key] = struct{}{}
        out = append(out, j)
    }
    return out
}
```

This is applied to each scraper's output. Combined with global dedup, the same job posted on multiple sites (e.g., reposted on both LinkedIn and Indeed) is only exported once.

---

## Email dedup

Email addresses within a job are also deduplicated during `enrichJobEmails`:

```go
func dedupEmails(in []model.Email) []model.Email {
    seen := make(map[string]struct{}, len(in))
    out := make([]model.Email, 0, len(in))
    for _, e := range in {
        addr := strings.TrimSpace(strings.ToLower(e.Addr))
        if addr == "" {
            continue
        }
        if _, ok := seen[addr]; ok {
            continue
        }
        seen[addr] = struct{}{}
        e.Addr = addr
        out = append(out, e)
    }
    return out
}
```

Case-insensitive dedup. Runs at every stage: after scraper-set emails, after text extraction, and after company page enrichment.

---

## Usage

```bash
# Default: URL dedup enabled, company dedup disabled
scrappy --sites linkedin,indeed --search "golang"

# Disable URL dedup
scrappy --sites linkedin --search "golang" --dedup=false

# Keep only one posting per company
scrappy --sites linkedin --search "golang" --dedup-by-company
```

When `--dedup-by-company` is used, the first job encountered for each company is kept; subsequent jobs from the same company are dropped regardless of quality score. Use with `--min-score` to ensure the surviving job is the highest-quality one.
