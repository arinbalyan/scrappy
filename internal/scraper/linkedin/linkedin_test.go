package linkedin_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/scraper/linkedin"
	"github.com/stretchr/testify/assert"
)

func TestScraper_SiteName(t *testing.T) {
	s := linkedin.New(nil)
	assert.Equal(t, model.SiteLinkedIn, s.SiteName())
}

func TestGuestAPIFallback_EmptySearch(t *testing.T) {
	client := &http.Client{Timeout: 100 * time.Millisecond}
	s := linkedin.New(client)
	_, err := s.Scrape(context.Background(), model.ScraperInput{
		SearchTerm: "",
	})
	assert.Error(t, err)
}

func TestGuestAPIFallback_ShortTimeout(t *testing.T) {
	client := &http.Client{Timeout: 100 * time.Millisecond}
	s := linkedin.New(client)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediately cancelled context
	_, err := s.Scrape(ctx, model.ScraperInput{
		SearchTerm:    "software engineer",
		ResultsWanted: 1,
	})
	assert.Error(t, err)
}
