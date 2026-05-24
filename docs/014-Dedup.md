# Deduplication

`internal/dedup/` -- removes duplicate postings before output.

## CLI flags

| Flag | Default | Description |
|------|---------|-------------|
| `--dedup` | `true` | Drop duplicate job URLs across sites. |
| `--dedup-by-company` | `false` | Keep only one posting per company across the entire result set. |

## Strategies

### URL dedup (cross-site)

Thread-safe `Set` keyed by job URL (uses `sync.Mutex` + `map[string]bool`). If Indeed and LinkedIn both return the same `indeed.com/viewjob?jk=...` URL, the second is dropped. O(1) lookup per posting.

```go
type Set struct {
    mu   sync.Mutex
    seen map[string]bool
}

func (s *Set) Add(url string) bool {
    if s.seen[url] {
        return false // already seen
    }
    s.seen[url] = true
    return true
}
```

**Edge case**: Same URL from different sites -- first seen wins (order is undefined, depends on goroutine scheduling). Different URLs pointing to the same job are not deduplicated.

### Company dedup

`--dedup-by-company` keeps exactly one posting per company across the entire result set. When two postings have the same `CompanyName`, whichever appears first in the filtered output wins.

```go
if companyDedup {
    key := j.CompanyName
    if !companySet.Add(key) {
        continue // duplicate company
    }
}
```

The `dedupNullCompany` flag controls whether empty company names are deduplicated against each other (when true, all empty-company postings are reduced to one).

### Within-site dedup

Before cross-site dedup, the engine runs a per-site pass that drops duplicate URLs within the same site. This handles sites that occasionally return the same job on different pages.

## DedupFilters function

The main entry point is `DedupFilters`, called by the engine after all sites finish:

```go
func DedupFilters(jobs []JobPost, skipURLDedup bool, companyDedup, dedupNullCompany bool) []JobPost
```

- `skipURLDedup`: when true, URL dedup is disabled (set when `--dedup` is false)
- `companyDedup`: when true, one posting per company is enforced (set by `--dedup-by-company`)
- `dedupNullCompany`: when true, empty `CompanyName` values are treated as a dedup group

## Pipeline order

```
1. Per-site URL dedup (engine, always on)
2. Cross-site URL dedup (dedup.DedupFilters, --dedup)
3. Cross-site company dedup (dedup.DedupFilters, --dedup-by-company)
4. Quality score filtering (--min-score)
5. Emails-only filtering (--email)
6. Results-wanted truncation
```

## Examples

```bash
# URL dedup only (default)
scrappy --sites linkedin,indeed --search "engineer" --dedup

# URL + company dedup
scrappy --sites linkedin,indeed --search "engineer" --dedup --dedup-by-company

# Disable all dedup
scrappy --sites linkedin,indeed --search "engineer" --dedup=false
```

When `--dedup-by-company` is combined with `--min-score`, the company dedup runs before score filtering, which means lower-quality jobs may represent a company in the dedup pass. If you want quality-based company selection, run a higher `--results-wanted` and rely on the downstream `--min-score` filter.
