package remoteco

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const defaultFeedURL = "https://remote.co/remote-jobs/feed/"

type Scraper struct {
	client  *http.Client
	feedURL string
}

func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 3, Timeout: 15 * time.Second})
	}
	return &Scraper{client: client, feedURL: defaultFeedURL}
}

func NewWithFeedURL(client *http.Client, u string) *Scraper {
	s := New(client)
	if strings.TrimSpace(u) != "" {
		s.feedURL = strings.TrimSpace(u)
	}
	return s
}

func (s *Scraper) SiteName() model.Site { return model.SiteRemoteCo }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.feedURL, nil)
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("remoteco request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("remoteco status %d", resp.StatusCode)
	}
	var feed struct {
		Channel struct {
			Items []struct {
				Title       string `xml:"title"`
				Link        string `xml:"link"`
				Description string `xml:"description"`
				PubDate     string `xml:"pubDate"`
			} `xml:"item"`
		} `xml:"channel"`
	}
	if err := xml.NewDecoder(resp.Body).Decode(&feed); err != nil {
		return nil, fmt.Errorf("remoteco decode: %w", err)
	}
	limit := input.ResultsWanted
	if limit <= 0 || limit > len(feed.Channel.Items) {
		limit = len(feed.Channel.Items)
	}
	out := make([]model.JobPost, 0, limit)
	for i := 0; i < limit; i++ {
		it := feed.Channel.Items[i]
		post := model.JobPost{
			ID:          fmt.Sprintf("rco-%d-%s", i+1, util.NormalizeSlug(it.Title)),
			Title:       strings.TrimSpace(it.Title),
			JobURL:      strings.TrimSpace(it.Link),
			Description: strings.TrimSpace(it.Description),
			IsRemote:    true,
		}
		post.DatePosted = util.ParseDatePosted(it.PubDate)
		if post.JobURL == "" {
			post.JobURL = "https://remote.co/remote-jobs/"
		}
		out = append(out, post)
	}
	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("remoteco no parseable jobs")
	}
	return out, nil
}
