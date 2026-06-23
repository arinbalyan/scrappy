package scraper

import (
	"context"
	"fmt"

	"github.com/arinbalyan/scrappy/internal/config"
	"github.com/arinbalyan/scrappy/internal/types"
)

// ForSite returns a Run function for the given config entry.
func ForSite(s config.Site) (func(context.Context, string) ([]types.JobPosting, error), error) {
	switch s.ID {
	case "indeed":
		return runIndeed(s), nil
	case "linkedin":
		return runLinkedIn(s), nil
	}
	switch s.Type {
	case "html":
		return runHTML(s), nil
	case "api":
		return runAPI(s), nil
	case "graphql":
		return runGraphQL(s), nil
	case "rss":
		return runRSS(s), nil
	default:
		return nil, fmt.Errorf("unknown type %q for site %s", s.Type, s.ID)
	}
}
