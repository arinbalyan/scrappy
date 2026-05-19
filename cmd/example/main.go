package main

import (
	"context"
	"fmt"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/pkg/scrappy"
)

func main() {
	engine := scrappy.NewEngine()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	jobs, err := engine.Scrape(ctx, model.ScraperInput{
		Sites:         []model.Site{model.SiteIndeed, model.SiteLinkedIn},
		SearchTerm:    "software engineer",
		Location:      "Remote",
		ResultsWanted: 10,
		Dedup:         true,
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf("fetched %d jobs\n", len(jobs))
}
