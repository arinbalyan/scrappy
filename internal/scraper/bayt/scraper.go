package bayt

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const defaultListURL = "https://www.bayt.com/en/international/jobs/"

var (
	reBaytCard   = regexp.MustCompile(`(?is)<li[^>]*data-js-job[^>]*>(.*?)</li>`)
	reBaytH2Href = regexp.MustCompile(`(?is)<h2[^>]*>\s*<a[^>]*href=["']([^"']+)["'][^>]*>(.*?)</a>`)
	reBaytCo     = regexp.MustCompile(`(?is)<div[^>]*class=["'][^"']*t-nowrap\s+p10l[^"']*["'][^>]*>.*?<span[^>]*>(.*?)</span>`)
	reBaytLoc    = regexp.MustCompile(`(?is)<div[^>]*class=["'][^"']*t-mute\s+t-small[^"']*["'][^>]*>(.*?)</div>`)
)

type Scraper struct {
	client  *http.Client
	listURL string
}

func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 3, Timeout: 18 * time.Second})
	}
	return &Scraper{client: client, listURL: defaultListURL}
}

func NewWithListURL(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.listURL = endpoint
	}
	return s
}

func (s *Scraper) SiteName() model.Site { return model.SiteBayt }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 15
	}
	searchSlug := strings.ReplaceAll(strings.TrimSpace(strings.ToLower(input.SearchTerm)), " ", "-")
	if searchSlug == "" {
		searchSlug = "jobs"
	}
	base, _ := url.Parse(s.listURL)
	seen := map[string]struct{}{}
	out := make([]model.JobPost, 0, wanted)

	for page := 1; len(out) < wanted && page <= 8; page++ {
		u := *base
		q := u.Query()
		q.Set("page", fmt.Sprintf("%d", page))
		u.RawQuery = q.Encode()
		if strings.Contains(u.Path, "/jobs/") && !strings.Contains(u.Path, "-jobs") {
			u.Path = strings.TrimRight(u.Path, "/") + "/" + searchSlug + "-jobs/"
		}
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		req.Header.Set("User-Agent", "Mozilla/5.0")
		resp, err := s.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("bayt request: %w", err)
		}
		body, readErr := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
		resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("bayt read: %w", readErr)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("bayt status %d", resp.StatusCode)
		}

		pageJobs := parseBaytJobs(body)
		if len(pageJobs) == 0 {
			break
		}
		for _, j := range pageJobs {
			if _, ok := seen[j.JobURL]; ok {
				continue
			}
			seen[j.JobURL] = struct{}{}
			out = append(out, j)
			if len(out) >= wanted {
				break
			}
		}
	}
	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("bayt no parseable jobs")
	}
	return out, nil
}

func parseBaytJobs(body []byte) []model.JobPost {
	raw := string(body)
	cards := reBaytCard.FindAllStringSubmatch(raw, -1)
	out := make([]model.JobPost, 0, len(cards))
	for i, c := range cards {
		chunk := c[1]
		m := reBaytH2Href.FindStringSubmatch(chunk)
		if len(m) < 3 {
			continue
		}
		href := strings.TrimSpace(m[1])
		title := cleanHTMLText(m[2])
		if title == "" || href == "" {
			continue
		}
		jobURL := href
		if strings.HasPrefix(jobURL, "/") {
			jobURL = "https://www.bayt.com" + jobURL
		}
		company := ""
		if cm := reBaytCo.FindStringSubmatch(chunk); len(cm) > 1 {
			company = cleanHTMLText(cm[1])
		}
		loc := ""
		if lm := reBaytLoc.FindStringSubmatch(chunk); len(lm) > 1 {
			loc = cleanHTMLText(lm[1])
		}
		if company == "" {
			company = "Unknown Employer"
		}
		out = append(out, model.JobPost{ID: fmt.Sprintf("bayt-%d", i+1), Title: title, CompanyName: company, JobURL: jobURL, Location: model.Location{City: loc, Country: "Worldwide"}})
	}
	return out
}

func cleanHTMLText(s string) string {
	tag := regexp.MustCompile(`<[^>]+>`)
	s = tag.ReplaceAllString(s, " ")
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}
