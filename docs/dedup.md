# Deduplication

`internal/dedup/` — removes duplicate postings before output.

## Strategies

### URL dedup (cross-site)

`sync.Map` keyed by job URL. If Indeed and LinkedIn both return the same `indeed.com/viewjob?jk=...` URL, drop the second. Fast O(1) membership test per posting across all sites.

### Company dedup

`--dedup-by-company` keeps exactly one posting per company for the current search. Preferred representative: highest quality score, then most recent, then longest description.

### Implementation

```go
type DedupeSet struct {
    urls  sync.Map   // url → true
    byCo  sync.Map   // companyName → JobPost
}

func (d *DedupeSet) Add(job JobPost) bool {
    if _, seen := d.urls.Load(job.JobURL); seen {
        return false // duplicate, skip
    }
    d.urls.Store(job.JobURL, true)

    if d.byCompany {
        existing, ok := d.byCo.LoadOrStore(job.CompanyName, job)
        if ok {
            existingJob := existing.(JobPost)
            if quality.Score(job) > quality.Score(existingJob) {
                d.byCo.Store(job.CompanyName, job)
            }
        }
        return true // kept (or replaced a lower-quality same-company entry)
    }
    return true
}
```

## CLI flags

```
--dedup           Drop duplicate job URLs across sites
--dedup-by-company     # Keep 1 posting per company
```
