package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const defaultBaseURL = "https://builtin.com/jobs"

var (
	reNextData     = regexp.MustCompile(`(?is)<script[^>]*id=["']__NEXT_DATA__["'][^>]*>(.*?)</script>`)
	// Old HTML patterns (Next.js + CSS class selectors) -- kept for backward compat
	reBuiltinCardLegacy  = regexp.MustCompile(`(?is)<(?:article|div)[^>]*(?:job-card|JobCard|job-listing)[^>]*>(.*?)</(?:article|div)>`)
	reBuiltinTitleLegacy = regexp.MustCompile(`(?is)<a[^>]*href=["']([^"']+)["'][^>]*>(.*?)</a>`)
	reBuiltinCompLegacy  = regexp.MustCompile(`(?is)<[^>]*class=["'][^"']*(?:company-name|Company)[^"']*["'][^>]*>(.*?)</[^>]+>`)
	reBuiltinLocLegacy   = regexp.MustCompile(`(?is)<[^>]*class=["'][^"']*(?:location|job-location)[^"']*["'][^>]*>(.*?)</[^>]+>`)
	// New HTML patterns (Alpine.js + data-id attributes)
	reBuiltinTitle = regexp.MustCompile(`(?is)<a[^>]*href=["']([^"']+)["'][^>]*data-id=["']job-card-title["'][^>]*>([^<]+)`)
	reBuiltinComp  = regexp.MustCompile(`(?is)data-id=["']company-title["'][^>]*>\s*<span>([^<]+)</span>`)
	reBuiltinLoc   = regexp.MustCompile(`(?is)fa-location-dot[^>]*></i>(?:\s*<[^>]*>)*?\s*<span[^>]*class=["'][^"']*font-barlow[^"']*text-gray-04[^"']*["'][^>]*>([^<]+)</span>`)
	// Salary: match K-suffixed ranges like "155K-170K" (most common on the site).
	// The leading \$? and trailing [Kk]? provide flexibility for legacy formats.
	reBuiltinSal = regexp.MustCompile(`\$?([\d,]+)[Kk]\s*[-–]\s*\$?([\d,]+)[Kk]`)
)

type Scraper struct {
	client  *http.Client
	baseURL string
}

func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 3, Timeout: 18 * time.Second})
	}
	return &Scraper{client: client, baseURL: defaultBaseURL}
}

func NewWithBaseURL(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.baseURL = endpoint
	}
	return s
}

func (s *Scraper) SiteName() model.Site { return model.SiteBuiltin }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 15
	}
	out := make([]model.JobPost, 0, wanted)
	seen := map[string]struct{}{}
	for page := 0; len(out) < wanted && page < 6; page++ {
		u, _ := url.Parse(s.baseURL)
		q := u.Query()
		if strings.TrimSpace(input.SearchTerm) != "" {
			q.Set("search", strings.TrimSpace(input.SearchTerm))
		}
		if page > 0 {
			q.Set("page", strconv.Itoa(page))
		}
		u.RawQuery = q.Encode()

		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		req.Header.Set("User-Agent", "Mozilla/5.0")
		resp, err := s.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("builtin request: %w", err)
		}
		body, readErr := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
		resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("builtin read: %w", readErr)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("builtin status %d", resp.StatusCode)
		}

		if challenge := util.DetectAntiBotChallenge(body); challenge != "" {
			return nil, fmt.Errorf("builtin: blocked - %s challenge detected", challenge)
		}

		pageJobs := parseBuiltinNextData(body)
		if len(pageJobs) == 0 {
			pageJobs = parseBuiltinHTML(body)
		}
		if len(pageJobs) == 0 {
			pageJobs = util.ExtractJobPostingsJSONLD(body)
		}
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
		return nil, fmt.Errorf("builtin no parseable jobs")
	}
	if len(out) > wanted {
		out = out[:wanted]
	}
	return out, nil
}

func parseBuiltinNextData(body []byte) []model.JobPost {
	m := reNextData.FindStringSubmatch(string(body))
	if len(m) < 2 {
		return nil
	}
	var parsed struct {
		Props struct {
			PageProps struct {
				Jobs []struct {
					ID          interface{} `json:"id"`
					Title       string      `json:"title"`
					URL         string      `json:"url"`
					Alias       string      `json:"alias"`
					CompanyName string      `json:"company_name"`
					BodyTeaser  string      `json:"body_teaser"`
					CityName    string      `json:"city_name"`
					StateName   string      `json:"state_name"`
					CountryName string      `json:"country_name"`
					SalaryMin   *float64    `json:"salary_min"`
					SalaryMax   *float64    `json:"salary_max"`
					RemoteType  string      `json:"remote_type"`
					Created     string      `json:"created"`
				} `json:"jobs"`
			} `json:"pageProps"`
		} `json:"props"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(m[1])), &parsed); err != nil {
		return nil
	}
	rows := parsed.Props.PageProps.Jobs
	out := make([]model.JobPost, 0, len(rows))
	for i, r := range rows {
		title := strings.TrimSpace(r.Title)
		if title == "" {
			continue
		}
		jobURL := strings.TrimSpace(r.URL)
		if jobURL == "" && strings.TrimSpace(r.Alias) != "" {
			jobURL = "https://builtin.com/job/" + strings.TrimSpace(r.Alias)
		}
		if jobURL == "" {
			jobURL = "https://builtin.com/jobs"
		}
		if strings.HasPrefix(jobURL, "/") {
			jobURL = "https://builtin.com" + jobURL
		}
		post := model.JobPost{ID: fmt.Sprintf("builtin-%v", r.ID), Title: title, CompanyName: strings.TrimSpace(r.CompanyName), JobURL: jobURL, Description: strings.TrimSpace(cleanBuiltinText(r.BodyTeaser)), Location: model.Location{City: strings.TrimSpace(r.CityName), State: strings.TrimSpace(r.StateName), Country: strings.TrimSpace(r.CountryName)}, IsRemote: strings.Contains(strings.ToLower(r.RemoteType), "remote")}
		if post.ID == "builtin-<nil>" {
			post.ID = fmt.Sprintf("builtin-%d", i+1)
		}
		if r.SalaryMin != nil || r.SalaryMax != nil {
			post.Compensation = &model.Compensation{Interval: model.IntervalYearly, MinAmount: r.SalaryMin, MaxAmount: r.SalaryMax, Currency: "USD"}
		}
		if strings.TrimSpace(r.Created) != "" {
			if t, err := time.Parse(time.RFC3339, r.Created); err == nil {
				post.DatePosted = &t
			}
		}
		if post.CompanyName == "" {
			post.CompanyName = "Unknown Employer"
		}
		out = append(out, post)
	}
	return out
}

func parseBuiltinHTML(body []byte) []model.JobPost {
	raw := string(body)

	// Try index-based extraction using the new data-id HTML structure.
	if jobs := parseBuiltinNew(raw); len(jobs) > 0 {
		return jobs
	}

	// Fall back to legacy CSS-class-based patterns (old Next.js version).
	legacyCards := reBuiltinCardLegacy.FindAllStringSubmatch(raw, -1)
	if len(legacyCards) == 0 {
		return nil
	}
	return parseBuiltinCardsLegacy(legacyCards)
}

// parseBuiltinNew extracts jobs from the rebuilt builtin.com HTML which uses
// Alpine.js and data-id attributes instead of CSS class names.  Since card
// boundary detection via regex is fragile (nested </div> pairs cause early
// termination), this function extracts every field globally and aligns them
// by occurrence index — fields within a single card appear in a stable order.
func parseBuiltinNew(raw string) []model.JobPost {
	titleMatches := reBuiltinTitle.FindAllStringSubmatch(raw, -1)
	if len(titleMatches) == 0 {
		return nil
	}

	compMatches := reBuiltinComp.FindAllStringSubmatch(raw, -1)
	locMatches := reBuiltinLoc.FindAllStringSubmatch(raw, -1)

	out := make([]model.JobPost, 0, len(titleMatches))
	for i, tm := range titleMatches {
		href := strings.TrimSpace(tm[1])
		title := cleanBuiltinText(tm[2])
		if href == "" || title == "" {
			continue
		}
		jobURL := href
		if strings.HasPrefix(jobURL, "/") {
			jobURL = "https://builtin.com" + jobURL
		}

		company := ""
		if i < len(compMatches) && len(compMatches[i]) > 1 {
			company = cleanBuiltinText(compMatches[i][1])
		}

		loc := ""
		if i < len(locMatches) && len(locMatches[i]) > 1 {
			loc = cleanBuiltinText(locMatches[i][1])
		}

		post := model.JobPost{
			ID:          fmt.Sprintf("builtin-%d", i+1),
			Title:       title,
			CompanyName: fallback(company, "Unknown Employer"),
			JobURL:      jobURL,
			Location:    model.Location{City: loc},
			IsRemote:    strings.Contains(strings.ToLower(loc), "remote"),
		}

		// Find the first salary match appearing after this job's title in the
		// HTML document (salary always follows the title within each card).
		titleEnd := strings.Index(raw, tm[0]) + len(tm[0])
		if titleEnd > len(tm[0]) && titleEnd < len(raw) {
			if sm := reBuiltinSal.FindStringSubmatch(raw[titleEnd:]); len(sm) == 3 {
				min := parseKNumber(sm[1])
				max := parseKNumber(sm[2])
				if min > 0 || max > 0 {
					post.Compensation = &model.Compensation{Interval: model.IntervalYearly, MinAmount: floatPtr(min), MaxAmount: floatPtr(max), Currency: "USD"}
				}
			}
		}

		out = append(out, post)
	}
	return out
}

// parseBuiltinCardsLegacy handles the old Next.js HTML structure.
func parseBuiltinCardsLegacy(cards [][]string) []model.JobPost {
	out := make([]model.JobPost, 0, len(cards))
	for i, c := range cards {
		chunk := c[1]
		m := reBuiltinTitleLegacy.FindStringSubmatch(chunk)
		if len(m) < 3 {
			continue
		}
		href := strings.TrimSpace(m[1])
		title := cleanBuiltinText(m[2])
		if href == "" || title == "" {
			continue
		}
		jobURL := href
		if strings.HasPrefix(jobURL, "/") {
			jobURL = "https://builtin.com" + jobURL
		}
		company := ""
		if cm := reBuiltinCompLegacy.FindStringSubmatch(chunk); len(cm) > 1 {
			company = cleanBuiltinText(cm[1])
		}
		loc := ""
		if lm := reBuiltinLocLegacy.FindStringSubmatch(chunk); len(lm) > 1 {
			loc = cleanBuiltinText(lm[1])
		}
		post := model.JobPost{ID: fmt.Sprintf("builtin-%d", i+1), Title: title, CompanyName: fallback(company, "Unknown Employer"), JobURL: jobURL, Location: model.Location{City: loc}, IsRemote: strings.Contains(strings.ToLower(loc), "remote")}
		if sm := reBuiltinSal.FindStringSubmatch(chunk); len(sm) == 3 {
			min := parseKNumber(sm[1])
			max := parseKNumber(sm[2])
			if min > 0 || max > 0 {
				post.Compensation = &model.Compensation{Interval: model.IntervalYearly, MinAmount: floatPtr(min), MaxAmount: floatPtr(max), Currency: "USD"}
			}
		}
		out = append(out, post)
	}
	return out
}

func parseKNumber(raw string) float64 {
	n := strings.ReplaceAll(strings.TrimSpace(raw), ",", "")
	if n == "" {
		return 0
	}
	v, err := strconv.ParseFloat(n, 64)
	if err != nil {
		return 0
	}
	if v < 1000 {
		v *= 1000
	}
	return v
}

func cleanBuiltinText(s string) string {
	tag := regexp.MustCompile(`<[^>]+>`)
	s = tag.ReplaceAllString(s, " ")
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}

func fallback(v, d string) string {
	if strings.TrimSpace(v) == "" {
		return d
	}
	return strings.TrimSpace(v)
}

func floatPtr(v float64) *float64 { return &v }
