package bdjobs

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
	"golang.org/x/net/html"
)

const defaultSearchURL = "https://jobs.bdjobs.com/jobsearch.asp"

// Scraper fetches jobs from BDJobs.com by scraping HTML search results.
type Scraper struct {
	client    *http.Client
	searchURL string
}

// New creates a new BDJobs scraper.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Timeout: 30 * time.Second})
	}
	return &Scraper{client: client, searchURL: defaultSearchURL}
}

// NewWithSearchURL creates a scraper with a custom search URL (used in tests).
func NewWithSearchURL(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.searchURL = endpoint
	}
	return s
}

// SiteName returns the site identifier.
func (s *Scraper) SiteName() model.Site { return model.SiteBDJobs }

// Scrape fetches jobs from BDJobs.com by paginating through search results.
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

	var jobs []model.JobPost
	seen := make(map[string]bool)
	page := 1

	for len(jobs) < wanted {
		select {
		case <-ctx.Done():
			return jobs, ctx.Err()
		default:
		}

		u := s.searchURL + "?hidJobSearch=jobsearch&txtsearch=" + strings.ReplaceAll(input.SearchTerm, " ", "+")
		if page > 1 {
			u += "&pg=" + fmt.Sprintf("%d", page)
		}

		util.Debug("bdjobs: fetching page", map[string]any{"page": page})

		pageJobs, err := s.fetchPage(ctx, u)
		if err != nil {
			return jobs, fmt.Errorf("bdjobs: page %d: %w", page, err)
		}

		if len(pageJobs) == 0 {
			break
		}

		newCount := 0
		for _, j := range pageJobs {
			if !seen[j.JobURL] {
				seen[j.JobURL] = true
				jobs = append(jobs, j)
				newCount++
			}
		}

		if newCount == 0 {
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
		return nil, fmt.Errorf("bdjobs: no parseable jobs")
	}
	return jobs, nil
}

func (s *Scraper) fetchPage(ctx context.Context, url string) ([]model.JobPost, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Referer", "https://jobs.bdjobs.com/")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}

	return parseJobs(body), nil
}

// parseJobs extracts job postings from BDJobs.com HTML search results.
func parseJobs(body []byte) []model.JobPost {
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil
	}

	var jobs []model.JobPost

	// Look for job detail links
	jobLinks := findJobLinks(doc)

	for _, link := range jobLinks {
		job := extractJobInfo(link)
		if job != nil {
			jobs = append(jobs, *job)
		}
	}

	return jobs
}

// jobLinkInfo holds href and surrounding context for a job posting link.
type jobLinkInfo struct {
	href  string
	title string
	card  *html.Node // parent card element for further extraction
}

// findJobLinks locates all job detail links on the page.
func findJobLinks(doc *html.Node) []jobLinkInfo {
	var links []jobLinkInfo
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, attr := range n.Attr {
				if attr.Key == "href" && strings.Contains(strings.ToLower(attr.Val), "jobdetail") {
					title := extractText(n)
					if strings.TrimSpace(title) == "" {
						title = "N/A"
					}
					links = append(links, jobLinkInfo{
						href:  strings.TrimSpace(attr.Val),
						title: strings.TrimSpace(title),
						card:  findParentCard(n),
					})
					break
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)
	return links
}

// findParentCard walks up the tree to find a container div for context.
func findParentCard(n *html.Node) *html.Node {
	for p := n.Parent; p != nil; p = p.Parent {
		if p.Type == html.ElementNode && p.Data == "div" {
			for _, attr := range p.Attr {
				class := attr.Val
				if strings.Contains(class, "job-item") ||
					strings.Contains(class, "norm-jobs") ||
					strings.Contains(class, "featured") ||
					strings.Contains(class, "sout-jobs") {
					return p
				}
			}
		}
	}
	return nil
}

// extractJobInfo pulls company name, location, and date from the card context.
func extractJobInfo(link jobLinkInfo) *model.JobPost {
	title := link.title
	if title == "" {
		return nil
	}

	jobURL := link.href
	if !strings.HasPrefix(jobURL, "http") {
		jobURL = "https://jobs.bdjobs.com/" + strings.TrimPrefix(jobURL, "/")
	}

	jobID := ""
	if idx := strings.Index(jobURL, "jobid="); idx >= 0 {
		rest := jobURL[idx+6:]
		if andIdx := strings.Index(rest, "&"); andIdx >= 0 {
			jobID = rest[:andIdx]
		} else {
			jobID = rest
		}
	}
	if jobID == "" {
		jobID = util.HashID("bdjobs-" + jobURL)
	} else {
		jobID = "bdjobs-" + jobID
	}

	companyName := ""
	locationText := "Dhaka, Bangladesh"
	dateStr := ""

	if link.card != nil {
		companyName = extractFirstClassText(link.card, "comp-name", "company")
		if loc := extractFirstClassText(link.card, "locon", "location"); loc != "" {
			locationText = loc
		}
		if d := extractFirstClassText(link.card, "date", "deadline"); d != "" {
			dateStr = d
		}
	}

	// Fallback: company name from the job link's parent siblings
	if companyName == "" {
		for p := link.card; p != nil; p = p.Parent {
			companyName = extractFirstClassText(p, "comp-name", "company")
			if companyName != "" {
				break
			}
		}
	}

	locParts := strings.SplitN(locationText, ",", 2)
	city := strings.TrimSpace(locParts[0])
	state := ""
	if len(locParts) > 1 {
		state = strings.TrimSpace(locParts[1])
	}

	var datePosted *time.Time
	if dateStr != "" {
		if parsed := parseDate(dateStr); parsed != nil {
			datePosted = parsed
		}
	}

	isRemote := strings.Contains(strings.ToLower(title+" "+locationText), "remote") ||
		strings.Contains(strings.ToLower(title+" "+locationText), "work from home")

	return &model.JobPost{
		ID:          jobID,
		Title:       title,
		CompanyName: companyName,
		JobURL:      jobURL,
		Location:    model.Location{City: city, State: state, Country: "Bangladesh"},
		DatePosted:  datePosted,
		IsRemote:    isRemote,
		Site:        string(model.SiteBDJobs),
	}
}

// extractFirstClassText finds the first element with one of the given class substrings and returns its text.
func extractFirstClassText(n *html.Node, classes ...string) string {
	var result string
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode {
			for _, attr := range n.Attr {
				if attr.Key == "class" {
					for _, cls := range classes {
						if strings.Contains(strings.ToLower(attr.Val), cls) {
							result = extractText(n)
							return
						}
					}
				}
			}
		}
		if result != "" {
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(n)
	return strings.TrimSpace(result)
}

// parseDate tries to parse a date string in various BDJobs formats.
func parseDate(s string) *time.Time {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "Deadline:")
	s = strings.TrimSpace(s)

	formats := []string{
		"2006-01-02",
		"02 Jan 2006",
		"2 Jan 2006",
		"January 2, 2006",
		"January 02, 2006",
		"02/01/2006",
		"2/1/2006",
		"2006-01-02T15:04:05",
		time.RFC3339,
	}
	for _, f := range formats {
		t, err := time.Parse(f, s)
		if err == nil {
			return &t
		}
	}
	return nil
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
	return strings.TrimSpace(b.String())
}

// Regex for jobid extraction fallback.
var jobIDRe = regexp.MustCompile(`jobid=(\d+)`)
