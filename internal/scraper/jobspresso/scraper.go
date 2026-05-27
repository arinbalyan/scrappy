package jobspresso

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

const feedURL = "https://jobspresso.co/feed/?post_type=job_listing"

var (
	jpItem  = regexp.MustCompile(`(?is)<item>(.*?)</item>`)
	jpTag   = regexp.MustCompile(`(?is)<%s[^>]*>(.*?)</%s>`)
	jpCDATA = regexp.MustCompile(`(?is)^\s*<!\[CDATA\[(.*?)\]\]>\s*$`)
	jpHTML  = regexp.MustCompile(`(?is)<[^>]+>`)
	jpTitle = regexp.MustCompile(`(?is)^\s*([^:]+):\s*(.+?)\s*$`)
)

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

func (s *Scraper) SiteName() model.Site { return model.SiteJobspresso }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.feedURL, nil)
	req.Header.Set("Accept", "application/rss+xml, application/xml, text/xml")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jobspresso request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("jobspresso status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("jobspresso read: %w", err)
	}

	items := jpItem.FindAllStringSubmatch(string(body), -1)
	limit := input.ResultsWanted
	if limit <= 0 || limit > len(items) {
		limit = len(items)
	}
	terms := parseSearchTerms(input.SearchTerm)
	jobs := make([]model.JobPost, 0, limit)

	for _, m := range items {
		if len(jobs) >= limit {
			break
		}
		chunk := m[1]
		titleRaw := extractJPTag(chunk, "title")
		link := strings.TrimSpace(extractJPTag(chunk, "link"))
		desc := extractJPTag(chunk, "description")
		pubDate := extractJPTag(chunk, "pubDate")
		category := extractJPTag(chunk, "category")

		if titleRaw == "" || link == "" {
			continue
		}
		if len(terms) > 0 {
			hay := strings.ToLower(titleRaw + " " + desc + " " + category)
			if !matchAny(hay, terms) {
				continue
			}
		}

		company := ""
		title := titleRaw
		if mt := jpTitle.FindStringSubmatch(titleRaw); len(mt) == 3 {
			company = strings.TrimSpace(mt[1])
			title = strings.TrimSpace(mt[2])
		}

		jobs = append(jobs, model.JobPost{
			ID:          "jobspresso-" + jidFromURL(link),
			Title:       strings.TrimSpace(title),
			CompanyName: strings.TrimSpace(company),
			JobURL:      link,
			Description: jpHTMLToText(desc),
			DatePosted:  jparseDate(pubDate),
			IsRemote:    true,
		})
	}

	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("jobspresso no parseable jobs")
	}
	return jobs, nil
}

func extractJPTag(xml, tag string) string {
	rx := regexp.MustCompile(fmt.Sprintf(jpTag.String(), tag, tag))
	m := rx.FindStringSubmatch(xml)
	if len(m) < 2 {
		return ""
	}
	v := strings.TrimSpace(m[1])
	if cm := jpCDATA.FindStringSubmatch(v); len(cm) == 2 {
		return strings.TrimSpace(cm[1])
	}
	return strings.TrimSpace(v)
}

func jpHTMLToText(v string) string {
	if strings.TrimSpace(v) == "" {
		return ""
	}
	v = jpHTML.ReplaceAllString(v, " ")
	return strings.Join(strings.Fields(strings.TrimSpace(v)), " ")
}

func jparseDate(v string) *time.Time {
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

func jidFromURL(raw string) string {
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
