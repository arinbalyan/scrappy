package simplyhired

import (
	"context"
	"fmt"
	"hash/fnv"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const searchURL = "https://www.simplyhired.com/search"

var (
	// jobCardRe matches each job card in the HTML. SimplyHired renders listings as
	// <div> elements with data-testid="searchSerpJob" or similar card structures.
	jobCardRe = regexp.MustCompile(`(?is)<div[^>]*?(?:data-testid="searchSerpJob"|class="[^"]*SerpJob[^"]*")[^>]*>(.*?)</div>\s*</div>`)

	// titleRe extracts the job title from an <a> or heading inside a card.
	titleRe = regexp.MustCompile(`(?is)(?:<a[^>]*data-testid="searchSerpJobTitle"[^>]*>|<h[23][^>]*>|<a[^>]*class="[^"]*jobposting-title[^"]*"[^>]*>)\s*([^<]+?)\s*</`)

	// companyRe extracts the company name.
	companyRe = regexp.MustCompile(`(?is)(?:data-testid="companyName"[^>]*>|class="[^"]*jobposting-company[^"]*"[^>]*>|class="[^"]*SerpJob-link--company[^"]*"[^>]*>)\s*([^<]+?)\s*<`)

	// locationRe extracts the location.
	locationRe = regexp.MustCompile(`(?is)(?:data-testid="searchSerpJobLocation"[^>]*>|class="[^"]*jobposting-location[^"]*"[^>]*>|class="[^"]*SerpJob-location[^"]*"[^>]*>)\s*([^<]+?)\s*<`)

	// salaryRe extracts the salary estimate text.
	salaryRe = regexp.MustCompile(`(?is)(?:data-testid="searchSerpJobSalaryEst"[^>]*>|class="[^"]*jobposting-salary[^"]*"[^>]*>|class="[^"]*SerpJob-salary[^"]*"[^>]*>)\s*([^<]+?)\s*<`)

	// snippetRe extracts the description snippet.
	snippetRe = regexp.MustCompile(`(?is)(?:class="[^"]*jobposting-snippet[^"]*"[^>]*>|class="[^"]*SerpJob-snippet[^"]*"[^>]*>)\s*([^<]+?)\s*<`)

	// hrefRe extracts the first href from an anchor tag.
	hrefRe = regexp.MustCompile(`(?i)<a[^>]+href="([^"]+)"`)
)

// Scraper scrapes jobs from simplyhired.com.
type Scraper struct {
	client    *http.Client
	searchURL string
}

// New creates a SimplyHired scraper with default settings.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 3, Timeout: 25 * time.Second})
	}
	return &Scraper{client: client, searchURL: searchURL}
}

// NewWithURLs creates a SimplyHired scraper with a custom endpoint (used in tests).
func NewWithURLs(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.searchURL = strings.TrimSpace(endpoint)
	}
	return s
}

// SiteName returns the SimplyHired site identifier.
func (s *Scraper) SiteName() model.Site { return model.SiteSimplyHired }

// Scrape fetches jobs from SimplyHired. It uses page-based pagination via pn=N,
// respects context cancellation, and rate-limits to ~3 requests per second.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 15
	}
	searchTerm := strings.TrimSpace(input.SearchTerm)
	if searchTerm == "" {
		return nil, fmt.Errorf("simplyhired: search term required")
	}

	jobs := make([]model.JobPost, 0, wanted)
	page := 1
	maxPages := wanted/20 + 3 // Allow a few extra pages for padding

	for len(jobs) < wanted && page <= maxPages {
		select {
		case <-ctx.Done():
			return jobs, ctx.Err()
		default:
		}

		body, err := s.fetchPage(ctx, searchTerm, input.Location, page)
		if err != nil {
			return jobs, fmt.Errorf("simplyhired page %d: %w", page, err)
		}

		parsed := parseJobs(body, page)
		if len(parsed) == 0 {
			break
		}

		for _, j := range parsed {
			jobs = append(jobs, j)
			if len(jobs) >= wanted {
				break
			}
		}

		page++

		// Rate limit: ~333ms per request (3 req/s max), with jitter
		if len(jobs) < wanted && page <= maxPages {
			select {
			case <-ctx.Done():
				return jobs, ctx.Err()
			case <-time.After(300*time.Millisecond + time.Duration(time.Now().UnixNano()%300)*time.Millisecond):
			}
		}
	}

	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("simplyhired no parseable jobs")
	}
	if len(jobs) > wanted {
		jobs = jobs[:wanted]
	}
	return jobs, nil
}

// fetchPage retrieves a SimplyHired search results page.
func (s *Scraper) fetchPage(ctx context.Context, searchTerm, location string, page int) ([]byte, error) {
	u, _ := url.Parse(s.searchURL)
	q := u.Query()
	q.Set("q", searchTerm)
	if v := strings.TrimSpace(location); v != "" {
		q.Set("l", v)
	}
	if page > 1 {
		q.Set("pn", fmt.Sprintf("%d", page))
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("simplyhired request: %w", err)
	}
	defer resp.Body.Close()

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("simplyhired read: %w", err)
	}

	if challenge := util.DetectAntiBotChallenge(body); challenge != "" {
		return nil, fmt.Errorf("blocked - %s challenge detected", challenge)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("simplyhired status %d", resp.StatusCode)
	}

	return body, nil
}

// parseJobs extracts job postings from a SimplyHired search results page.
func parseJobs(raw []byte, page int) []model.JobPost {
	html := string(raw)
	cards := extractCards(html)
	jobs := make([]model.JobPost, 0, len(cards))

	for _, card := range cards {
		job := parseCard(card)
		if job.Title == "" || job.JobURL == "" {
			continue
		}
		jobs = append(jobs, job)
	}
	return jobs
}

// extractCards finds individual job card HTML blocks.
func extractCards(html string) []string {
	// Try primary selector first: data-testid="searchSerpJob"
	cards := jobCardRe.FindAllString(html, -1)
	if len(cards) > 0 {
		return cards
	}

	// Fallback: try splitting on known class-based card markers
	parts := strings.Split(html, `data-testid="searchSerpJob"`)
	if len(parts) > 1 {
		// We split on the attribute; re-add the prefix
		out := make([]string, 0, len(parts)-1)
		for i := 1; i < len(parts); i++ {
			// Each card starts right after the split marker
			// Find the enclosing div boundary
			card := `<div data-testid="searchSerpJob"` + parts[i]
			// Simple heuristic: take up to the next closing div that's far enough
			card = extractCardBoundary(card)
			if card != "" {
				out = append(out, card)
			}
		}
		return out
	}

	return nil
}

// extractCardBoundary extracts a job card by finding the matching closing div.
func extractCardBoundary(html string) string {
	// Find the deepest balanced div close. For simplicity, find the first
	// </div> after some minimum content length.
	const minCardLen = 80
	idx := strings.Index(html, "</div>")
	if idx < minCardLen {
		return ""
	}
	return html[:idx+6] // include the </div>
}

// parseCard extracts a single JobPost from a card HTML block.
func parseCard(card string) model.JobPost {
	title := extractMatch(card, titleRe)
	company := extractMatch(card, companyRe)
	location := extractMatch(card, locationRe)
	description := extractMatch(card, snippetRe)
	if description == "" {
		// If no snippet found, try a broader paragraph extraction
		description = extractFallbackSnippet(card)
	}

	jobURL := extractHref(card)

	// Build the ID from the URL
	id := "sh-" + hashID(jobURL)

	job := model.JobPost{
		ID:          id,
		Title:       title,
		CompanyName: company,
		JobURL:      jobURL,
		Description: description,
	}

	if loc := strings.TrimSpace(location); loc != "" {
		job.Location = model.Location{City: loc}
	}

	// Parse salary if present
	salaryText := extractMatch(card, salaryRe)
	if salaryText != "" {
		if comp := parseSalary(salaryText); comp != nil {
			job.Compensation = comp
		}
	}

	return job
}

// extractMatch returns the first submatch from the regex applied to html.
func extractMatch(html string, re *regexp.Regexp) string {
	matches := re.FindStringSubmatch(html)
	if len(matches) < 2 {
		return ""
	}
	return strings.TrimSpace(matches[1])
}

// extractHref extracts the first href from an anchor tag inside the card.
func extractHref(card string) string {
	matches := hrefRe.FindStringSubmatch(card)
	if len(matches) < 2 {
		return ""
	}
	href := strings.TrimSpace(matches[1])
	if href == "" {
		return ""
	}
	if strings.HasPrefix(href, "/") {
		href = "https://www.simplyhired.com" + href
	}
	return href
}

// extractFallbackSnippet extracts text from paragraph tags as a fallback.
func extractFallbackSnippet(card string) string {
	pRe := regexp.MustCompile(`(?is)<p[^>]*>\s*(.+?)\s*</p>`)
	matches := pRe.FindStringSubmatch(card)
	if len(matches) < 2 {
		return ""
	}
	return strings.TrimSpace(matches[1])
}

// parseSalary attempts to parse a salary string into a Compensation struct.
func parseSalary(text string) *model.Compensation {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	currency := "USD"
	if strings.Contains(text, "£") {
		currency = "GBP"
	} else if strings.Contains(text, "€") {
		currency = "EUR"
	}

	// Remove non-numeric characters except decimal and range separators
	clean := strings.NewReplacer(",", "", "$", "", "£", "", "€", "", " ", "").Replace(text)

	// Extract numbers (integer or decimal)
	numRe := regexp.MustCompile(`(\d+(?:\.\d+)?)`)
	nums := numRe.FindAllString(clean, -1)

	var minAmount, maxAmount *float64
	if len(nums) >= 2 {
		min := parseFloat(nums[0])
		max := parseFloat(nums[1])
		// If numbers are in thousands (e.g., 100K), assume annual
		if max < 1000 {
			// Could be hourly, weekly, or monthly - check the text
			textLower := strings.ToLower(text)
			if strings.Contains(textLower, "hour") || strings.Contains(textLower, "hr") {
				min *= 2080 // approximate annual from hourly
				max *= 2080
			} else if strings.Contains(textLower, "week") || strings.Contains(textLower, "wk") {
				min *= 52
				max *= 52
			} else if strings.Contains(textLower, "month") {
				min *= 12
				max *= 12
			}
		}
		minAmount = &min
		maxAmount = &max
	} else if len(nums) == 1 {
		v := parseFloat(nums[0])
		minAmount = &v
	}

	if minAmount == nil && maxAmount == nil {
		return nil
	}

	return &model.Compensation{
		Interval:  model.IntervalYearly,
		MinAmount: minAmount,
		MaxAmount: maxAmount,
		Currency:  currency,
	}
}

// parseFloat parses a string to float64.
func parseFloat(s string) float64 {
	var v float64
	fmt.Sscanf(s, "%f", &v)
	return v
}

// hashID produces a short hash for dedup identification.
func hashID(s string) string {
	h := fnv.New64a()
	h.Write([]byte(s))
	return fmt.Sprintf("%d", h.Sum64())
}
