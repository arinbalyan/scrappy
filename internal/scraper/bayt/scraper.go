package bayt

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
	"golang.org/x/net/html"
)

const baseURL = "https://www.bayt.com"

// Scraper fetches jobs from Bayt.com by scraping HTML pages.
type Scraper struct {
	client  *http.Client
	baseURL string
}

// New creates a new Bayt scraper.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Timeout: 20 * time.Second})
	}
	return &Scraper{client: client, baseURL: baseURL}
}

// NewWithBaseURL creates a scraper with a custom base URL (used in tests).
func NewWithBaseURL(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.baseURL = endpoint
	}
	return s
}

// SiteName returns the site identifier.
func (s *Scraper) SiteName() model.Site { return model.SiteBayt }

// Scrape fetches jobs from Bayt.com by paginating through HTML search results.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{
		"site":           s.SiteName(),
		"results_wanted": input.ResultsWanted,
		"search_term":    input.SearchTerm,
	})

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 25
	}

	term := strings.ToLower(strings.TrimSpace(input.SearchTerm))
	searchSlug := strings.ReplaceAll(term, " ", "-")

	var jobs []model.JobPost
	page := 1

	for len(jobs) < wanted {
		select {
		case <-ctx.Done():
			return jobs, ctx.Err()
		default:
		}

		url := fmt.Sprintf("%s/en/international/jobs/%s-jobs/?page=%d", s.baseURL, searchSlug, page)
		util.Debug("bayt: fetching page", map[string]any{"page": page, "url": url})

		pageJobs, err := s.fetchPage(ctx, url)
		if err != nil {
			return jobs, fmt.Errorf("bayt: page %d: %w", page, err)
		}

		if len(pageJobs) == 0 {
			break
		}

		jobs = append(jobs, pageJobs...)

		if len(pageJobs) < 20 {
			break
		}

		page++
		if page > 10 {
			break
		}
	}

	if len(jobs) > wanted {
		jobs = jobs[:wanted]
	}

	util.Debug("scraper_done", map[string]any{
		"site": s.SiteName(),
		"jobs": len(jobs),
	})

	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("bayt: no parseable jobs")
	}
	return jobs, nil
}

func (s *Scraper) fetchPage(ctx context.Context, url string) ([]model.JobPost, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("status %d — try using --proxy with a residential proxy", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}

	return parseJobs(body, s.baseURL), nil
}

// parseJobs extracts job postings from Bayt.com HTML search results.
func parseJobs(body []byte, baseURL string) []model.JobPost {
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil
	}

	var jobs []model.JobPost

	// Bayt uses <li data-js-job> elements for job cards
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "li" {
			isJobCard := false
			for _, attr := range n.Attr {
				if attr.Key == "data-js-job" {
					isJobCard = true
					break
				}
			}
			if isJobCard {
				if job := extractJobCard(n, baseURL); job != nil {
					jobs = append(jobs, *job)
				}
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	return jobs
}

// extractJobCard extracts a single job posting from a <li data-js-job> element.
func extractJobCard(n *html.Node, baseURL string) *model.JobPost {
	var title, href, companyName, locationText string

	// <h2> contains title and link
	var findTitle func(*html.Node)
	findTitle = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "h2" {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && c.Data == "a" {
					for _, attr := range c.Attr {
						if attr.Key == "href" {
							href = strings.TrimSpace(attr.Val)
						}
					}
					title = extractText(c)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findTitle(c)
		}
	}
	findTitle(n)

	if title == "" || href == "" {
		return nil
	}

	// Job URL
	jobURL := href
	if !strings.HasPrefix(jobURL, "http") {
		jobURL = baseURL + href
	}

	// Company name from div.t-nowrap.p10l > span
	var findCompany func(*html.Node)
	findCompany = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "div" {
			hasClass := false
			for _, attr := range n.Attr {
				if attr.Key == "class" && strings.Contains(attr.Val, "t-nowrap") {
					hasClass = true
					break
				}
			}
			if hasClass {
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					if c.Type == html.ElementNode && c.Data == "span" {
						companyName = extractText(c)
						return
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findCompany(c)
		}
	}
	findCompany(n)

	// Location from div.t-mute.t-small
	var findLocation func(*html.Node)
	findLocation = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "div" {
			hasClass := false
			for _, attr := range n.Attr {
				if attr.Key == "class" && strings.Contains(attr.Val, "t-mute") && strings.Contains(attr.Val, "t-small") {
					hasClass = true
					break
				}
			}
			if hasClass {
				locationText = extractText(n)
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findLocation(c)
		}
	}
	findLocation(n)

	jobID := util.HashID("bayt-" + jobURL)

	return &model.JobPost{
		ID:          jobID,
		Title:       strings.TrimSpace(title),
		CompanyName: strings.TrimSpace(companyName),
		JobURL:      jobURL,
		Location:    model.Location{City: strings.TrimSpace(locationText)},
		Site:        string(model.SiteBayt),
	}
}

// extractText extracts all text content from a node subtree.
func extractText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}
