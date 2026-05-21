package internshala

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

const (
	baseURL       = "https://internshala.com"
	defaultJobs   = "https://internshala.com/jobs"
	defaultIntern = "https://internshala.com/internships"
)

var (
	reISCard      = regexp.MustCompile(`(?is)<(?:div|article)[^>]*(?:individual_internship|individual_job|internship_meta|job-listing-card)[^>]*>(.*?)</(?:div|article)>`)
	reISTitleLink = regexp.MustCompile(`(?is)<a[^>]*(?:job-title-href|view_detail_button|href)[^>]*href=["']([^"']+)["'][^>]*>(.*?)</a>`)
	reISCompany   = regexp.MustCompile(`(?is)<[^>]*class=["'][^"']*(?:company-name|company_name|link_display_like_text)[^"']*["'][^>]*>(.*?)</[^>]+>`)
	reISLocation  = regexp.MustCompile(`(?is)<[^>]*class=["'][^"']*(?:location_link|individual_location_name|locations)[^"']*["'][^>]*>(.*?)</[^>]+>`)
)

type Scraper struct {
	client *http.Client
	jobs   string
	intern string
}

func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 3, Timeout: 18 * time.Second})
	}
	return &Scraper{client: client, jobs: defaultJobs, intern: defaultIntern}
}

func NewWithURLs(client *http.Client, jobsURL, internURL string) *Scraper {
	s := New(client)
	if strings.TrimSpace(jobsURL) != "" {
		s.jobs = jobsURL
	}
	if strings.TrimSpace(internURL) != "" {
		s.intern = internURL
	}
	return s
}

func (s *Scraper) SiteName() model.Site { return model.SiteInternshala }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 15
	}
	seen := map[string]struct{}{}
	out := make([]model.JobPost, 0, wanted)

	for page := 1; len(out) < wanted && page <= 8; page++ {
		rawURL := s.buildSearchURL(input, page)
		u, _ := url.Parse(rawURL)
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		req.Header.Set("User-Agent", "Mozilla/5.0")
		resp, err := s.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("internshala request: %w", err)
		}
		body, readErr := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
		resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("internshala read: %w", readErr)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("internshala status %d", resp.StatusCode)
		}

		pageJobs := parseInternshalaJobs(body)
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
		return nil, fmt.Errorf("internshala no parseable jobs")
	}
	return out, nil
}

func (s *Scraper) buildSearchURL(input model.ScraperInput, page int) string {
	searchSlug := strings.ReplaceAll(strings.TrimSpace(strings.ToLower(input.SearchTerm)), " ", "-")
	base := s.jobs
	if input.JobType == model.JobTypeInternship {
		base = s.intern
	}
	if searchSlug != "" {
		base = strings.TrimRight(base, "/") + "/" + searchSlug
	}
	if strings.TrimSpace(input.Location) != "" {
		locSlug := strings.ReplaceAll(strings.TrimSpace(strings.ToLower(input.Location)), " ", "-")
		base += "-in-" + locSlug
	}
	if input.IsRemote {
		base = strings.TrimRight(base, "/") + "/work-from-home"
	}
	if page > 1 {
		base = strings.TrimRight(base, "/") + fmt.Sprintf("/page-%d", page)
	}
	return base
}

func parseInternshalaJobs(body []byte) []model.JobPost {
	raw := string(body)
	cards := reISCard.FindAllStringSubmatch(raw, -1)
	out := make([]model.JobPost, 0, len(cards))
	for i, c := range cards {
		chunk := c[1]
		m := reISTitleLink.FindStringSubmatch(chunk)
		if len(m) < 3 {
			continue
		}
		href := strings.TrimSpace(m[1])
		title := cleanISText(m[2])
		if href == "" || title == "" {
			continue
		}
		jobURL := href
		if !strings.HasPrefix(jobURL, "http") {
			jobURL = baseURL + "/" + strings.TrimPrefix(jobURL, "/")
		}
		company := ""
		if cm := reISCompany.FindStringSubmatch(chunk); len(cm) > 1 {
			company = cleanISText(cm[1])
		}
		loc := ""
		if lm := reISLocation.FindStringSubmatch(chunk); len(lm) > 1 {
			loc = cleanISText(lm[1])
		}
		if company == "" {
			company = "Unknown Employer"
		}
		full := strings.ToLower(chunk)
		isRemote := strings.Contains(full, "work from home") || strings.Contains(full, "wfh")
		out = append(out, model.JobPost{ID: fmt.Sprintf("is-%d", i+1), Title: title, CompanyName: company, JobURL: jobURL, Location: model.Location{City: loc, Country: "India"}, IsRemote: isRemote, DatePosted: parseInternDate(chunk)})
	}
	return out
}

func parseInternDate(raw string) *time.Time { return nil }

func cleanISText(s string) string {
	tag := regexp.MustCompile(`<[^>]+>`)
	s = tag.ReplaceAllString(s, " ")
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}
