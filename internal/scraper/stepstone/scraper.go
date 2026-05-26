package stepstone

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
	"golang.org/x/net/html"
)

const defaultDomain = "www.stepstone.de"

// Scraper fetches jobs from StepStone by scraping search result HTML.
type Scraper struct {
	client *http.Client
	domain string
}

// New creates a new StepStone scraper.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Timeout: 30 * time.Second})
	}
	return &Scraper{client: client, domain: defaultDomain}
}

// NewWithDomain creates a scraper with a custom domain (used in tests).
func NewWithDomain(client *http.Client, domain string) *Scraper {
	s := New(client)
	if strings.TrimSpace(domain) != "" {
		s.domain = domain
	}
	return s
}

// SiteName returns the site identifier.
func (s *Scraper) SiteName() model.Site { return model.SiteStepStone }

// Scrape fetches jobs from StepStone by scraping the search results page.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{
		"site":           s.SiteName(),
		"results_wanted": input.ResultsWanted,
		"search_term":    input.SearchTerm,
	})

	searchTerm := strings.TrimSpace(input.SearchTerm)
	if searchTerm == "" {
		searchTerm = "developer"
	}
	slug := strings.ReplaceAll(searchTerm, " ", "-")
	searchURL := s.buildSearchURL(slug)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("stepstone: build request: %w", err)
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("stepstone: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("stepstone: status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("stepstone: read: %w", err)
	}

	// Parse search results HTML and JSON-LD
	jobs := parseSearchResults(string(body), s.domain)
	util.Debug("stepstone: parsed jobs from HTML", map[string]any{"count": len(jobs)})

	if len(jobs) == 0 {
		return nil, fmt.Errorf("stepstone: no jobs parsed from page")
	}

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = len(jobs)
	}
	if wanted > len(jobs) {
		wanted = len(jobs)
	}

	out := jobs[:wanted]
	for i := range out {
		out[i].Site = string(s.SiteName())
		if out[i].ID == "" {
			out[i].ID = fmt.Sprintf("stepstone-%d", i)
		}
	}

	util.Debug("scraper_done", map[string]any{
		"site": s.SiteName(),
		"jobs": len(out),
	})

	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("stepstone: no parseable jobs")
	}
	return out, nil
}

// parseSearchResults extracts job postings from StepStone HTML.
func parseSearchResults(htmlContent, domain string) []model.JobPost {
	jobs := extractJobCards(htmlContent, domain)

	// Enrich with JSON-LD data if available
	enrichFromJSONLD(htmlContent, jobs)

	return jobs
}

// extractJobCards scrapes job cards from the StepStone search results HTML.
// It tries multiple selectors to find job cards.
func extractJobCards(htmlContent, domain string) []model.JobPost {
	// Use a broad regex approach to find job links with /jobs-- or /stellenangebote-- patterns
	jobLinkRe := regexp.MustCompile(`(?is)(/stellenangebote--[^"']+|/jobs--[^"']+)`)

	// Find all unique job URLs
	seen := make(map[string]bool)
	var jobs []model.JobPost

	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		// Fallback: regex-based extraction
		matches := jobLinkRe.FindAllStringSubmatch(htmlContent, -1)
		for _, m := range matches {
			href := m[1]
			if seen[href] {
				continue
			}
			seen[href] = true
			fullURL := href
			if strings.HasPrefix(href, "/") {
				fullURL = "https://" + domain + href
			}
			// Extract title from the anchor text via surrounding context
			title := extractTitleNearLink(htmlContent, href)
			if title == "" {
				title = extractCategoryFromURL(href)
			}
			if title == "" {
				continue
			}
			jobs = append(jobs, model.JobPost{
				ID:     fmt.Sprintf("stepstone-%d", len(jobs)),
				Title:  title,
				JobURL: fullURL,
			})
		}
		return jobs
	}

	// Use HTML node traversal to find job cards via text patterns
	var extract func(*html.Node)
	extract = func(n *html.Node) {
		if n.Type == html.ElementNode {
			// Look for <a> tags with job-related hrefs
			for _, a := range n.Attr {
				if a.Key == "href" {
					href := a.Val
					if (strings.Contains(href, "/stellenangebote--") || strings.Contains(href, "/jobs--")) && !seen[href] {
						seen[href] = true
						fullURL := href
						if strings.HasPrefix(href, "/") {
							fullURL = "https://" + domain + href
						}
						// Extract text from child nodes
						title := extractTextFromNode(n)
						if title == "" {
							// Try parent
							if n.Parent != nil {
								title = extractTextFromNode(n.Parent)
							}
						}
						if title != "" {
							jobs = append(jobs, model.JobPost{
								ID:     fmt.Sprintf("stepstone-%d", len(jobs)),
								Title:  title,
								JobURL: fullURL,
							})
						}
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extract(c)
		}
	}
	extract(doc)

	// If no jobs found via node traversal, fall back to regex
	if len(jobs) == 0 {
		matches := jobLinkRe.FindAllStringSubmatch(htmlContent, -1)
		for _, m := range matches {
			href := m[1]
			if seen[href] {
				continue
			}
			seen[href] = true
			fullURL := href
			if strings.HasPrefix(href, "/") {
				fullURL = "https://" + domain + href
			}
			title := extractTitleNearLink(htmlContent, href)
			if title == "" {
				continue
			}
			jobs = append(jobs, model.JobPost{
				ID:     fmt.Sprintf("stepstone-%d", len(jobs)),
				Title:  title,
				JobURL: fullURL,
			})
		}
	}

	return jobs
}

// extractTextFromNode extracts all text from a node.
func extractTextFromNode(n *html.Node) string {
	var buf strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			buf.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.TrimSpace(buf.String())
}

// extractTitleNearLink tries to find a job title near a given URL in the HTML.
func extractTitleNearLink(htmlContent, href string) string {
	// Find text near the link: look for anchor text or nearby heading
	idx := strings.Index(htmlContent, href)
	if idx < 0 {
		return ""
	}

	// Look backwards from the href to find a title
	pre := htmlContent[:idx]
	if lastBracket := strings.LastIndex(pre, ">"); lastBracket >= 0 {
		contextAround := pre[lastBracket+1:]
		contextAround = strings.TrimSpace(contextAround)
		if contextAround != "" && len(contextAround) < 200 {
			return cleanTitle(contextAround)
		}
	}

	// Look ahead after the href closing tag
	after := htmlContent[idx+len(href):]
	if closeBracket := strings.Index(after, ">"); closeBracket >= 0 {
		afterTag := after[closeBracket+1:]
		if nextOpen := strings.Index(afterTag, "<"); nextOpen > 0 {
			candidate := strings.TrimSpace(afterTag[:nextOpen])
			if candidate != "" && len(candidate) < 200 {
				return cleanTitle(candidate)
			}
		}
	}

	return ""
}

// cleanTitle removes common noise from titles.
func cleanTitle(s string) string {
	s = strings.TrimSpace(s)
	// Remove leading/trailing punctuation
	s = strings.Trim(s, " \t\n\r\"'<>")
	return s
}

// buildSearchURL constructs the full search URL from domain and slug.
func (s *Scraper) buildSearchURL(slug string) string {
	if strings.Contains(s.domain, "://") {
		return fmt.Sprintf("%s/jobs/%s", strings.TrimRight(s.domain, "/"), slug)
	}
	return fmt.Sprintf("https://%s/jobs/%s", s.domain, slug)
}

// extractCategoryFromURL extracts a display name from a StepStone URL slug.
func extractCategoryFromURL(href string) string {
	parts := strings.Split(strings.TrimRight(href, "/"), "--")
	if len(parts) > 0 {
		last := parts[len(parts)-1]
		return strings.ReplaceAll(last, "-", " ")
	}
	return ""
}

// enrichFromJSONLD extracts JSON-LD job postings from the page and merges them
// into the already-extracted jobs, matching by title.
func enrichFromJSONLD(htmlContent string, jobs []model.JobPost) {
	jsonLDRe := regexp.MustCompile(`(?is)<script\s+type=["']application/ld\+json["'][^>]*>(.*?)</script>`)
	blocks := jsonLDRe.FindAllStringSubmatch(htmlContent, -1)

	for _, m := range blocks {
		if len(m) < 2 {
			continue
		}
		var ld struct {
			Type      string `json:"@type"`
			Title     string `json:"title"`
			DatePosted string `json:"datePosted"`
			Description string `json:"description"`
			BaseSalary *struct {
				Currency string `json:"currency"`
				Value    *struct {
					MinValue float64 `json:"minValue"`
					MaxValue float64 `json:"maxValue"`
				} `json:"value"`
			} `json:"baseSalary"`
			HiringOrganization *struct {
				Name string `json:"name"`
			} `json:"hiringOrganization"`
			EmploymentType string `json:"employmentType"`
		}
		if err := json.Unmarshal([]byte(m[1]), &ld); err != nil {
			continue
		}
		if ld.Type != "JobPosting" || ld.Title == "" {
			continue
		}

		// Find matching job by title
		for i := range jobs {
			if strings.EqualFold(jobs[i].Title, ld.Title) {
				if ld.Description != "" && jobs[i].Description == "" {
					jobs[i].Description = util.StripHTML(ld.Description)
				}
				if ld.DatePosted != "" && jobs[i].DatePosted == nil {
					if t, err := time.Parse(time.RFC3339, ld.DatePosted); err == nil {
						jobs[i].DatePosted = &t
					}
				}
				if ld.BaseSalary != nil && jobs[i].Compensation == nil {
					curr := ld.BaseSalary.Currency
					if curr == "" {
						curr = "EUR"
					}
					if ld.BaseSalary.Value != nil {
						minAmt := ld.BaseSalary.Value.MinValue
						maxAmt := ld.BaseSalary.Value.MaxValue
						if minAmt > 0 || maxAmt > 0 {
							jobs[i].Compensation = &model.Compensation{
								Interval:  model.IntervalYearly,
								MinAmount: &minAmt,
								MaxAmount: &maxAmt,
								Currency:  curr,
							}
						}
					}
				}
				if ld.HiringOrganization != nil && ld.HiringOrganization.Name != "" && jobs[i].CompanyName == "" {
					jobs[i].CompanyName = ld.HiringOrganization.Name
				}
				break
			}
		}
	}
}
