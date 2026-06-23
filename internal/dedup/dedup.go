package dedup

import (
	"sync"
	"github.com/arinbalyan/scrappy/internal/model"
)

// Set is a thread-safe deduplication set keyed by job URL.
type Set struct {
	mu   sync.Mutex
	seen map[string]bool
}

// NewSet returns a ready-to-use Set.
func NewSet() *Set {
	return &Set{seen: make(map[string]bool)}
}

// Add records url and returns true if it was newly added (not seen before).
func (s *Set) Add(url string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.seen[url] {
		return false
	}
	s.seen[url] = true
	return true
}

// DedupFilters runs URL dedup and, when provided, company dedup on a slice of jobs.
// Returns the filtered slice, discarding any job whose URL or company name has already been seen.
func DedupFilters(jobs []model.JobPost, skipURLDedup bool, companyDedup, dedupNullCompany bool) []model.JobPost {
	if len(jobs) == 0 {
		return jobs
	}

	urlSet := NewSet()
	companySet := NewSet()

	var out []model.JobPost
	for _, j := range jobs {
		if !skipURLDedup && j.JobURL != "" {
			if !urlSet.Add(j.JobURL) {
				continue // duplicate URL
			}
		}
		if companyDedup {
			key := j.CompanyName
			if dedupNullCompany {
				key = "null:" + key
			}
			if !companySet.Add(key) {
				continue // duplicate company
			}
		}
		out = append(out, j)
	}
	return out
}
