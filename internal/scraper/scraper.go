package scraper

import (
	"context"

	"github.com/arinbalyan/scrappy/internal/model"
)

type Scraper interface {
	Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error)
	SiteName() model.Site
}
