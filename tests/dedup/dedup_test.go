package dedup_test

import (
	"testing"
	"github.com/arinbalyan/scrappy/internal/dedup"
	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestSet_AddAndSeen(t *testing.T) {
	s := dedup.NewSet()
	assert.True(t, s.Add("http://example.com"))
	assert.False(t, s.Add("http://example.com"))
}

func TestSet_EmptyURL(t *testing.T) {
	s := dedup.NewSet()
	assert.True(t, s.Add(""), "empty URL should still be added (caller's responsibility to skip)")
}

func TestDedupFilters_URLDedup(t *testing.T) {
	jobs := []model.JobPost{
		{JobURL: "http://example.com/1", Title: "A"},
		{JobURL: "http://example.com/2", Title: "B"},
		{JobURL: "http://example.com/1", Title: "A-dup"},
	}
	out := dedup.DedupFilters(jobs, false, false, false)
	assert.Len(t, out, 2)
}

func TestDedupFilters_CompanyDedup(t *testing.T) {
	jobs := []model.JobPost{
		{JobURL: "http://a.com/1", CompanyName: "Acme"},
		{JobURL: "http://b.com/2", CompanyName: "Acme"},
		{JobURL: "http://c.com/3", CompanyName: "Beta"},
	}
	out := dedup.DedupFilters(jobs, false, true, false)
	assert.Len(t, out, 2)
}

func TestDedupFilters_NullCompany(t *testing.T) {
	jobs := []model.JobPost{
		{JobURL: "http://a.com/1", CompanyName: ""},
		{JobURL: "http://b.com/2", CompanyName: ""},
	}
	out := dedup.DedupFilters(jobs, false, true, true)
	assert.Len(t, out, 1)
}

func TestDedupFilters_SkipURLDedup(t *testing.T) {
	jobs := []model.JobPost{
		{JobURL: "http://same.com/1", Title: "A"},
		{JobURL: "http://same.com/1", Title: "B"},
	}
	out := dedup.DedupFilters(jobs, true, false, false)
	assert.Len(t, out, 2)
}

func TestDedupFilters_NilInput(t *testing.T) {
	out := dedup.DedupFilters(nil, false, false, false)
	assert.Empty(t, out)
}

func TestDedupFilters_EmptyInput(t *testing.T) {
	out := dedup.DedupFilters([]model.JobPost{}, false, false, false)
	assert.Empty(t, out)
}
