package hackernews

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const (
	jobStoriesURL = "https://hacker-news.firebaseio.com/v0/jobstories.json"
	itemURLFmt    = "https://hacker-news.firebaseio.com/v0/item/%d.json"
	batchSize     = 15
	maxFetchIDs   = 100
)

type hnItem struct {
	ID    int64  `json:"id"`
	Type  string `json:"type"`
	Title string `json:"title"`
	Text  string `json:"text"`
	URL   string `json:"url"`
	Time  int64  `json:"time"`
}

var (
	sepCompany  = regexp.MustCompile(`^(.+?)\s*[-|]\s+`)
	hnStripTags = regexp.MustCompile(`(?is)<[^>]+>`)
)

type Scraper struct {
	client      *http.Client
	storiesURL  string
	itemURLTmpl string
}

func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 2, Timeout: 15 * time.Second})
	}
	return &Scraper{client: client, storiesURL: jobStoriesURL, itemURLTmpl: itemURLFmt}
}

func NewWithURLs(client *http.Client, storiesURL, itemFmt string) *Scraper {
	s := New(client)
	if strings.TrimSpace(storiesURL) != "" {
		s.storiesURL = strings.TrimSpace(storiesURL)
	}
	if strings.TrimSpace(itemFmt) != "" {
		s.itemURLTmpl = strings.TrimSpace(itemFmt)
	}
	return s
}

func (s *Scraper) SiteName() model.Site { return model.SiteHackerNews }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 15
	}
	term := strings.ToLower(strings.TrimSpace(input.SearchTerm))

	ids, err := s.fetchIDs(ctx)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("hackernews no job story ids")
	}

	fetchCount := wanted * 2
	if term != "" {
		fetchCount = maxFetchIDs
	}
	if fetchCount > len(ids) {
		fetchCount = len(ids)
	}
	ids = ids[:fetchCount]

	jobs := make([]model.JobPost, 0, wanted)
	for i := 0; i < len(ids) && len(jobs) < wanted; i += batchSize {
		end := i + batchSize
		if end > len(ids) {
			end = len(ids)
		}
		for _, id := range ids[i:end] {
			item, err := s.fetchItem(ctx, id)
			if err != nil || item == nil {
				continue
			}
			if term != "" {
				hay := strings.ToLower(item.Title + " " + item.Text)
				if !strings.Contains(hay, term) {
					continue
				}
			}
			if strings.TrimSpace(item.Title) == "" {
				continue
			}
			url := strings.TrimSpace(item.URL)
			if url == "" {
				url = fmt.Sprintf("https://news.ycombinator.com/item?id=%d", item.ID)
			}
			j := model.JobPost{
				ID:          fmt.Sprintf("hackernews-%d", item.ID),
				Title:       strings.TrimSpace(item.Title),
				CompanyName: extractCompanyName(item.Title),
				JobURL:      url,
				Description: hnHTMLToText(item.Text),
			}
			if item.Time > 0 {
				t := time.Unix(item.Time, 0).UTC()
				j.DatePosted = &t
			}
			jobs = append(jobs, j)
			if len(jobs) >= wanted {
				break
			}
		}
	}
	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("hackernews no parseable jobs")
	}
	return jobs, nil
}

func (s *Scraper) fetchIDs(ctx context.Context) ([]int64, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.storiesURL, nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("hackernews stories request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("hackernews stories status %d", resp.StatusCode)
	}
	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("hackernews stories read: %w", err)
	}
	var ids []int64
	if err := json.Unmarshal(body, &ids); err != nil {
		return nil, fmt.Errorf("hackernews stories decode: %w", err)
	}
	return ids, nil
}

func (s *Scraper) fetchItem(ctx context.Context, id int64) (*hnItem, error) {
	u := fmt.Sprintf(s.itemURLTmpl, id)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("hackernews item status %d", resp.StatusCode)
	}
	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, err
	}
	var item hnItem
	if err := json.Unmarshal(body, &item); err != nil {
		return nil, err
	}
	if item.ID == 0 {
		return nil, fmt.Errorf("hackernews item empty")
	}
	return &item, nil
}

func hnHTMLToText(v string) string {
	if strings.TrimSpace(v) == "" {
		return ""
	}
	v = hnStripTags.ReplaceAllString(v, " ")
	return strings.Join(strings.Fields(strings.TrimSpace(v)), " ")
}

func extractCompanyName(title string) string {
	t := strings.TrimSpace(title)
	if t == "" {
		return ""
	}
	if m := regexp.MustCompile(`(?i)^(.+?)\s+(?:is\s+hiring)`).FindStringSubmatch(t); len(m) == 2 {
		c := strings.TrimSpace(m[1])
		c = regexp.MustCompile(`\s*\(YC\s+\w+\)\s*`).ReplaceAllString(c, " ")
		return strings.TrimSpace(c)
	}
	if m := sepCompany.FindStringSubmatch(t); len(m) == 2 {
		c := strings.TrimSpace(m[1])
		lc := strings.ToLower(c)
		if len(c) < 60 && !strings.Contains(lc, "engineer") && !strings.Contains(lc, "developer") && !strings.Contains(lc, "designer") && !strings.Contains(lc, "manager") {
			return c
		}
	}
	return ""
}
