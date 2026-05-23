package bdjobs

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

const defaultSearchURL = "https://jobs.bdjobs.com/jobsearch.asp"

var (
	reBDJobLink   = regexp.MustCompile(`(?is)<a[^>]*href=["']([^"']*jobdetail[^"']*)["'][^>]*>(.*?)</a>`)
	reBDCompany   = regexp.MustCompile(`(?is)<[^>]*class=["'][^"']*(?:comp-name|company)[^"']*["'][^>]*>(.*?)</[^>]+>`)
	reBDLocation  = regexp.MustCompile(`(?is)<[^>]*class=["'][^"']*(?:locon|location)[^"']*["'][^>]*>(.*?)</[^>]+>`)
	reBDBlockScan = regexp.MustCompile(`(?is)<(?:div|li)[^>]*>(.*?)</(?:div|li)>`)
)

type Scraper struct {
	client    *http.Client
	searchURL string
}

func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 3, Timeout: 18 * time.Second})
	}
	return &Scraper{client: client, searchURL: defaultSearchURL}
}

func NewWithSearchURL(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.searchURL = endpoint
	}
	return s
}

func (s *Scraper) SiteName() model.Site { return model.SiteBDJobs }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 15
	}
	base, _ := url.Parse(s.searchURL)
	seen := map[string]struct{}{}
	out := make([]model.JobPost, 0, wanted)

	for page := 1; len(out) < wanted && page <= 8; page++ {
		u := *base
		q := u.Query()
		q.Set("hidJobSearch", "jobsearch")
		q.Set("txtsearch", strings.TrimSpace(input.SearchTerm))
		if page > 1 {
			q.Set("pg", fmt.Sprintf("%d", page))
		}
		u.RawQuery = q.Encode()

		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		req.Header.Set("User-Agent", "Mozilla/5.0")
		resp, err := s.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("bdjobs request: %w", err)
		}
		body, readErr := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
		resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("bdjobs read: %w", readErr)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("bdjobs status %d", resp.StatusCode)
		}

		pageJobs := parseBDJobs(body)
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
		return nil, fmt.Errorf("bdjobs no parseable jobs")
	}
	return out, nil
}

func parseBDJobs(body []byte) []model.JobPost {
	raw := string(body)
	blocks := reBDBlockScan.FindAllStringSubmatch(raw, -1)
	out := make([]model.JobPost, 0, len(blocks))
	for i, b := range blocks {
		chunk := b[1]
		m := reBDJobLink.FindStringSubmatch(chunk)
		if len(m) < 3 {
			continue
		}
		href := strings.TrimSpace(m[1])
		title := cleanBDText(m[2])
		if href == "" || title == "" {
			continue
		}
		jobURL := href
		if !strings.HasPrefix(jobURL, "http") {
			jobURL = "https://jobs.bdjobs.com/" + strings.TrimPrefix(jobURL, "/")
		}
		company := ""
		if cm := reBDCompany.FindStringSubmatch(chunk); len(cm) > 1 {
			company = cleanBDText(cm[1])
		}
		location := ""
		if lm := reBDLocation.FindStringSubmatch(chunk); len(lm) > 1 {
			location = cleanBDText(lm[1])
		}
		if company == "" {
			company = "Unknown Employer"
		}
		out = append(out, model.JobPost{ID: fmt.Sprintf("bdjobs-%d", i+1), Title: title, CompanyName: company, JobURL: jobURL, Location: model.Location{City: location, Country: "Bangladesh"}})
	}
	return out
}

func cleanBDText(s string) string {
	tag := regexp.MustCompile(`<[^>]+>`)
	s = tag.ReplaceAllString(s, " ")
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}
