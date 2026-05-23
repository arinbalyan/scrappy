package academiccareers

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const defaultRSSURL = "https://www.academiccareers.com/rss"

// rssReItem matches <item>...</item> blocks (non-greedy).
var rssReItem = regexp.MustCompile(`(?is)<item>(.*?)</item>`)

type Scraper struct {
	client  *http.Client
	rssURL  string
}

func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 2, Timeout: 20 * time.Second})
	}
	return &Scraper{client: client, rssURL: defaultRSSURL}
}

func NewWithURLs(client *http.Client, rssURL string) *Scraper {
	s := New(client)
	if strings.TrimSpace(rssURL) != "" {
		s.rssURL = strings.TrimSpace(rssURL)
	}
	return s
}

func (s *Scraper) SiteName() model.Site { return model.SiteAcademicCareers }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 15
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.rssURL, nil)
	if err != nil {
		return nil, fmt.Errorf("academiccareers request: %w", err)
	}
	req.Header.Set("Accept", "application/rss+xml, application/xml, text/xml")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129 Safari/537.36")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("academiccareers request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("academiccareers status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("academiccareers read: %w", err)
	}

	items, err := parseRSSItems(string(body))
	if err != nil {
		return nil, fmt.Errorf("academiccareers parse: %w", err)
	}

	term := strings.ToLower(strings.TrimSpace(input.SearchTerm))
	jobs := make([]model.JobPost, 0, wanted)

	for _, item := range items {
		if len(jobs) >= wanted {
			break
		}
		if term != "" {
			hay := strings.ToLower(item.title + " " + item.description)
			if !strings.Contains(hay, term) {
				continue
			}
		}
		job, err := mapJob(item)
		if err != nil {
			continue
		}
		if strings.TrimSpace(job.Title) == "" || strings.TrimSpace(job.JobURL) == "" {
			continue
		}
		jobs = append(jobs, *job)
	}

	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("academiccareers no parseable jobs")
	}
	return jobs, nil
}

type rssItem struct {
	title       string
	link        string
	guid        string
	description string
	pubDate     string
	creator     string
}

func parseRSSItems(xml string) ([]rssItem, error) {
	blocks := rssReItem.FindAllStringSubmatch(xml, -1)
	items := make([]rssItem, 0, len(blocks))
	for _, b := range blocks {
		content := b[1]
		item := rssItem{
			title:       extractRSSField(content, "title"),
			link:        extractRSSField(content, "link"),
			guid:        extractRSSField(content, "guid"),
			description: extractRSSField(content, "description"),
			pubDate:     extractRSSField(content, "pubDate"),
			creator:     extractRSSField(content, "dc:creator"),
		}
		items = append(items, item)
	}
	return items, nil
}

func extractRSSField(content, tag string) string {
	re := regexp.MustCompile(`(?is)<(?:dc:)?` + regexp.QuoteMeta(tag) + `[^>]*>\s*(?:<!\[CDATA\[([\s\S]*?)\]\]>\s*|([\s\S]*?))\s*</`)
	m := re.FindStringSubmatch(content)
	if m == nil {
		return ""
	}
	if m[1] != "" {
		return strings.TrimSpace(m[1])
	}
	return strings.TrimSpace(m[2])
}

func mapJob(item rssItem) (*model.JobPost, error) {
	if item.link == "" {
		return nil, fmt.Errorf("no link")
	}
	title := item.title
	companyName := item.creator

	var description string
	if input := strings.TrimSpace(item.description); input != "" {
		description = cleanRSSValue(input)
	}

	var datePosted *time.Time
	if item.pubDate != "" {
		if t, err := time.Parse(time.RFC1123Z, item.pubDate); err == nil {
			datePosted = &t
		} else if t, err := time.Parse(time.RFC3339, item.pubDate); err == nil {
			datePosted = &t
		}
	}

	jobID := item.guid
	if jobID == "" {
		jobID = item.link
	}

	return &model.JobPost{
		ID:          "ac-" + cleanRSSValue(jobID),
		Title:       cleanRSSValue(title),
		CompanyName: cleanRSSValue(companyName),
		JobURL:      strings.TrimSpace(item.link),
		Description: description,
		DatePosted:  datePosted,
		IsRemote:    false,
	}, nil
}

func cleanRSSValue(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	// Decode common HTML entities that may appear in RSS feeds.
	v = strings.ReplaceAll(v, "&amp;", "&")
	v = strings.ReplaceAll(v, "&lt;", "<")
	v = strings.ReplaceAll(v, "&gt;", ">")
	v = strings.ReplaceAll(v, "&quot;", `"`)
	v = strings.ReplaceAll(v, "&#39;", "'")
	// Strip any remaining HTML tags.
	v = regexp.MustCompile(`(?is)<[^>]+>`).ReplaceAllString(v, " ")
	return strings.Join(strings.Fields(v), " ")
}
