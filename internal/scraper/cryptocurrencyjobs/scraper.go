package cryptocurrencyjobs

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

const feedURL = "https://cryptocurrencyjobs.co/index.xml"

var (
	rssItemRe  = regexp.MustCompile(`(?is)<item>(.*?)</item>`)
	stripTagRe = regexp.MustCompile(`(?is)<[^>]+>`)
)

type rssItem struct {
	Title       string
	Link        string
	GUID        string
	Description string
	PubDate     string
	Category    string
}

type Scraper struct {
	client  *http.Client
	feedURL string
}

func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 2, Timeout: 15 * time.Second})
	}
	return &Scraper{client: client, feedURL: feedURL}
}

func NewWithFeedURL(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.feedURL = endpoint
	}
	return s
}

func (s *Scraper) SiteName() model.Site { return model.SiteCryptocurrencyJobs }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 25
	}
	terms := parseSearchTerms(input.SearchTerm)

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.feedURL, nil)
	req.Header.Set("Accept", "application/rss+xml, application/xml, text/xml")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cryptocurrencyjobs request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("cryptocurrencyjobs status %d", resp.StatusCode)
	}
	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("cryptocurrencyjobs read: %w", err)
	}

	items := parseItems(string(body))
	jobs := make([]model.JobPost, 0, wanted)
	for _, it := range items {
		if len(jobs) >= wanted {
			break
		}
		if strings.TrimSpace(it.Title) == "" || strings.TrimSpace(it.Link) == "" {
			continue
		}
		if len(terms) > 0 {
			hay := strings.ToLower(it.Title + " " + it.Description + " " + it.Category)
			if !matchAny(hay, terms) {
				continue
			}
		}
		company := ""
		if m := regexp.MustCompile(`(?i)\bat\s+(.+)$`).FindStringSubmatch(it.Title); len(m) == 2 {
			company = strings.TrimSpace(m[1])
		}
		j := model.JobPost{
			ID:          "cryptocurrencyjobs-" + idFromURL(firstNonEmpty(it.GUID, it.Link)),
			Title:       strings.TrimSpace(it.Title),
			CompanyName: company,
			JobURL:      strings.TrimSpace(it.Link),
			Description: cjHTMLToText(it.Description),
		}
		if t := parseRSSDate(it.PubDate); t != nil {
			j.DatePosted = t
		}
		jobs = append(jobs, j)
	}

	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("cryptocurrencyjobs no parseable jobs")
	}
	return jobs, nil
}

func parseItems(xml string) []rssItem {
	blocks := rssItemRe.FindAllStringSubmatch(xml, -1)
	out := make([]rssItem, 0, len(blocks))
	for _, b := range blocks {
		chunk := b[1]
		out = append(out, rssItem{
			Title:       extractTag(chunk, "title"),
			Link:        extractTag(chunk, "link"),
			GUID:        extractTag(chunk, "guid"),
			Description: extractTag(chunk, "description"),
			PubDate:     extractTag(chunk, "pubDate"),
			Category:    extractTag(chunk, "category"),
		})
	}
	return out
}

func extractTag(xml, tag string) string {
	esc := regexp.QuoteMeta(tag)
	cdata := regexp.MustCompile(`(?is)<` + esc + `[^>]*>\s*<!\[CDATA\[([\s\S]*?)\]\]>\s*</` + esc + `>`)
	if m := cdata.FindStringSubmatch(xml); len(m) == 2 {
		return strings.TrimSpace(m[1])
	}
	plain := regexp.MustCompile(`(?is)<` + esc + `[^>]*>([\s\S]*?)</` + esc + `>`)
	if m := plain.FindStringSubmatch(xml); len(m) == 2 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func cjHTMLToText(v string) string {
	if strings.TrimSpace(v) == "" {
		return ""
	}
	v = stripTagRe.ReplaceAllString(v, " ")
	return strings.Join(strings.Fields(strings.TrimSpace(v)), " ")
}

// parseSearchTerms splits a search term on " OR " and returns lowercase terms.
func parseSearchTerms(raw string) []string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, " OR ")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.Trim(strings.TrimSpace(p), `"`)
		if p != "" {
			out = append(out, strings.ToLower(p))
		}
	}
	return out
}

// matchAny returns true if the haystack contains any of the terms.
func matchAny(hay string, terms []string) bool {
	for _, t := range terms {
		if strings.Contains(hay, t) {
			return true
		}
	}
	return false
}

func parseRSSDate(v string) *time.Time {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	if t, err := time.Parse(time.RFC1123Z, v); err == nil {
		return &t
	}
	if t, err := time.Parse(time.RFC1123, v); err == nil {
		return &t
	}
	return nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func idFromURL(raw string) string {
	u := strings.TrimSpace(raw)
	if u == "" {
		return "unknown"
	}
	parts := strings.Split(u, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		p := strings.TrimSpace(parts[i])
		if p != "" {
			return p
		}
	}
	return "unknown"
}
