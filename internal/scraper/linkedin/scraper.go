package linkedin

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const (
	baseURL        = "https://www.linkedin.com"
	jobsSearchPath = "/jobs-guest/jobs/api/seeMoreJobPostings/search"
)

var (
	reJobCardHref   = regexp.MustCompile(`href="([^"]*?/jobs/view/(\d+)[^"]*)"`)
	reTitleSR       = regexp.MustCompile(`<span class="sr-only">([^<]+)</span>`)
	reCompany       = regexp.MustCompile(`base-search-card__subtitle[\s\S]*?<a[^>]*>([^<]+)</a>`)
	reLocation      = regexp.MustCompile(`job-search-card__location">([^<]+)<`)
	reDateTime      = regexp.MustCompile(`<time[^>]*datetime="([0-9]{4}-[0-9]{2}-[0-9]{2})"`)
	reSalary        = regexp.MustCompile(`job-search-card__salary-info[^>]*>\s*([^<]+)<`)
	reApplyURLCode  = regexp.MustCompile(`<code id="applyUrl">([\s\S]*?)</code>`)
	reApplyURLParam = regexp.MustCompile(`\?url=([^"]+)`)
	reDescription   = regexp.MustCompile(`show-more-less-html__markup[\s\S]*?<span>([\s\S]*?)</span>`)
)

type Scraper struct {
	client  *http.Client
	baseURL string
	delay   time.Duration
}

func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 3, CookieResetEveryN: 80, Timeout: 15 * time.Second})
	}
	return &Scraper{client: client, baseURL: baseURL, delay: 0}
}

func NewWithBaseURL(client *http.Client, customBaseURL string) *Scraper {
	s := New(client)
	if strings.TrimSpace(customBaseURL) != "" {
		s.baseURL = strings.TrimRight(customBaseURL, "/")
	}
	return s
}

func (s *Scraper) SiteName() model.Site { return model.SiteLinkedIn }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	if input.ResultsWanted <= 0 {
		input.ResultsWanted = 15
	}
	if strings.EqualFold(strings.TrimSpace(input.LinkedInStrategy), "rotate") {
		return s.scrapeRotateStrategy(ctx, input)
	}
	return s.scrapeSinglePass(ctx, input)
}

func (s *Scraper) scrapeSinglePass(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	seen := map[string]struct{}{}
	jobs := make([]model.JobPost, 0, input.ResultsWanted)
	start := 0
	if input.Offset > 0 {
		start = (input.Offset / 10) * 10
	}

	for len(jobs) < input.Offset+input.ResultsWanted && start < 1000 {
		htmlBody, err := s.fetchSearchPage(ctx, input, start)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "status 429") {
				s.adaptiveBackoff(ctx, start)
				continue
			}
			return nil, err
		}
		parsed := parseJobCards(htmlBody, s.baseURL)
		if len(parsed) == 0 {
			break
		}

		for _, jp := range parsed {
			if _, ok := seen[jp.ID]; ok {
				continue
			}
			seen[jp.ID] = struct{}{}
			if input.LinkedInFetchDesc && jp.ID != "" {
				details, _ := s.fetchJobDetails(ctx, strings.TrimPrefix(jp.ID, "li-"))
				if details.Description != "" {
					jp.Description = details.Description
				}
				if details.JobURLDirect != "" {
					jp.JobURLDirect = details.JobURLDirect
				}
				if len(details.JobType) > 0 {
					jp.JobType = string(details.JobType[0])
				}
				if details.JobLevel != "" {
					jp.JobLevel = strings.ToLower(details.JobLevel)
				}
				if details.CompanyIndustry != "" {
					jp.CompanyIndustry = details.CompanyIndustry
				}
				if details.CompanyLogo != "" {
					jp.CompanyLogo = details.CompanyLogo
				}
			}
			jobs = append(jobs, jp)
			if len(jobs) >= input.Offset+input.ResultsWanted {
				break
			}
		}
		if len(parsed) < 25 {
			break
		}
		start += len(parsed)
		if s.delay > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(s.delay):
			}
		}
	}

	from := input.Offset
	if from > len(jobs) {
		from = len(jobs)
	}
	to := from + input.ResultsWanted
	if to > len(jobs) {
		to = len(jobs)
	}
	return jobs[from:to], nil
}

func (s *Scraper) scrapeRotateStrategy(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	passes := []struct {
		remote bool
		easy   bool
		hours  int
	}{
		{remote: input.IsRemote, easy: input.EasyApply, hours: input.HoursOld},
		{remote: true, easy: false, hours: input.HoursOld},
		{remote: false, easy: true, hours: input.HoursOld},
		{remote: false, easy: false, hours: input.HoursOld},
	}
	seen := map[string]struct{}{}
	all := make([]model.JobPost, 0, input.ResultsWanted)
	for _, pass := range passes {
		if len(all) >= input.ResultsWanted {
			break
		}
		cp := input
		cp.IsRemote = pass.remote
		cp.EasyApply = pass.easy
		cp.HoursOld = pass.hours
		cp.ResultsWanted = input.ResultsWanted - len(all)
		jobs, err := s.scrapeSinglePass(ctx, cp)
		if err != nil {
			continue
		}
		for _, j := range jobs {
			if _, ok := seen[j.ID]; ok {
				continue
			}
			seen[j.ID] = struct{}{}
			all = append(all, j)
			if len(all) >= input.ResultsWanted {
				break
			}
		}
	}
	if len(all) == 0 {
		return nil, nil
	}
	return all, nil
}

func (s *Scraper) adaptiveBackoff(ctx context.Context, start int) {
	base := 900 * time.Millisecond
	if start > 200 {
		base = 1300 * time.Millisecond
	}
	jitter := time.Duration(rand.Intn(700)) * time.Millisecond
	wait := base + jitter
	select {
	case <-ctx.Done():
		return
	case <-time.After(wait):
	}
}

func (s *Scraper) fetchSearchPage(ctx context.Context, input model.ScraperInput, start int) (string, error) {
	u, _ := url.Parse(s.baseURL + jobsSearchPath)
	q := u.Query()
	q.Set("start", strconv.Itoa(start))
	if input.SearchTerm != "" {
		q.Set("keywords", input.SearchTerm)
	}
	if input.Location != "" {
		q.Set("location", input.Location)
	}
	if input.DistanceMiles > 0 {
		q.Set("distance", strconv.Itoa(input.DistanceMiles))
	}
	if input.IsRemote {
		q.Set("f_WT", "2")
	}
	if jt := linkedInJobTypeCode(input.JobType); jt != "" {
		q.Set("f_JT", jt)
	}
	if input.EasyApply {
		q.Set("f_AL", "true")
	}
	if len(input.LinkedInCompanyIDs) > 0 {
		ids := make([]string, 0, len(input.LinkedInCompanyIDs))
		for _, id := range input.LinkedInCompanyIDs {
			ids = append(ids, strconv.Itoa(id))
		}
		q.Set("f_C", strings.Join(ids, ","))
	}
	if input.HoursOld > 0 {
		q.Set("f_TPR", "r"+strconv.Itoa(input.HoursOld*3600))
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", fmt.Errorf("create linkedin search request: %w", err)
	}
	req.Header.Set("accept", "text/html")
	req.Header.Set("user-agent", "Mozilla/5.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("execute linkedin search request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("linkedin search status %d", resp.StatusCode)
	}
	b, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return "", fmt.Errorf("read linkedin search response: %w", err)
	}
	return string(b), nil
}

type details struct {
	Description     string
	JobType         []model.JobType
	JobLevel        string
	CompanyIndustry string
	JobURLDirect    string
	CompanyLogo     string
}

func (s *Scraper) fetchJobDetails(ctx context.Context, jobID string) (details, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+"/jobs/view/"+jobID, nil)
	if err != nil {
		return details{}, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return details{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return details{}, fmt.Errorf("linkedin details status %d", resp.StatusCode)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return details{}, err
	}
	html := string(b)

	d := details{}
	if m := reDescription.FindStringSubmatch(html); len(m) > 1 {
		d.Description = stripTags(m[1])
	}
	if m := reApplyURLCode.FindStringSubmatch(html); len(m) > 1 {
		if u := reApplyURLParam.FindStringSubmatch(m[1]); len(u) > 1 {
			if decoded, err := url.QueryUnescape(u[1]); err == nil {
				d.JobURLDirect = decoded
			}
		}
	}
	return d, nil
}

func parseJobCards(htmlBody, base string) []model.JobPost {
	reCard := regexp.MustCompile(`(?s)<div class="base-search-card"[\s\S]*?</div>`)
	cards := reCard.FindAllString(htmlBody, -1)
	jobs := make([]model.JobPost, 0, len(cards))
	for _, c := range cards {
		href := firstGroup(reJobCardHref, c, 1)
		jobID := firstGroup(reJobCardHref, c, 2)
		if href == "" || jobID == "" {
			continue
		}
		title := htmlUnescape(strings.TrimSpace(firstGroup(reTitleSR, c, 1)))
		company := htmlUnescape(strings.TrimSpace(firstGroup(reCompany, c, 1)))
		locRaw := htmlUnescape(strings.TrimSpace(firstGroup(reLocation, c, 1)))
		dateRaw := strings.TrimSpace(firstGroup(reDateTime, c, 1))
		salaryRaw := htmlUnescape(strings.TrimSpace(firstGroup(reSalary, c, 1)))

		jp := model.JobPost{
			ID:          "li-" + jobID,
			Title:       title,
			CompanyName: company,
			JobURL:      cleanURL(href, base),
			Location:    parseLocation(locRaw),
			IsRemote:    isRemote(title, locRaw, ""),
		}
		if t, err := time.Parse("2006-01-02", dateRaw); err == nil {
			jp.DatePosted = &t
		}
		if comp := parseSalary(salaryRaw); comp != nil {
			jp.Compensation = comp
		}
		jobs = append(jobs, jp)
	}
	return jobs
}

func parseLocation(v string) model.Location {
	parts := strings.Split(v, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	switch len(parts) {
	case 1:
		return model.Location{City: parts[0]}
	case 2:
		return model.Location{City: parts[0], State: parts[1]}
	case 3:
		return model.Location{City: parts[0], State: parts[1], Country: parts[2]}
	default:
		return model.Location{}
	}
}

func parseSalary(v string) *model.Compensation {
	if v == "" {
		return nil
	}
	n := extractNumbers(v)
	if len(n) < 2 {
		return nil
	}
	min := float64(n[0])
	max := float64(n[1])
	return &model.Compensation{MinAmount: &min, MaxAmount: &max, Currency: "USD"}
}

func extractNumbers(v string) []int {
	re := regexp.MustCompile(`\d[\d,]*`)
	m := re.FindAllString(v, -1)
	out := make([]int, 0, len(m))
	for _, s := range m {
		n, err := strconv.Atoi(strings.ReplaceAll(s, ",", ""))
		if err == nil {
			out = append(out, n)
		}
	}
	return out
}

func linkedInJobTypeCode(t model.JobType) string {
	switch t {
	case model.JobTypeFullTime:
		return "F"
	case model.JobTypePartTime:
		return "P"
	case model.JobTypeInternship:
		return "I"
	case model.JobTypeContract:
		return "C"
	case model.JobTypeTemporary:
		return "T"
	default:
		return ""
	}
}

func isRemote(title, description, location string) bool {
	s := strings.ToLower(title + " " + description + " " + location)
	return strings.Contains(s, "remote") || strings.Contains(s, "work from home") || strings.Contains(s, "wfh")
}

func firstGroup(re *regexp.Regexp, s string, idx int) string {
	m := re.FindStringSubmatch(s)
	if len(m) <= idx {
		return ""
	}
	return m[idx]
}

func cleanURL(href, base string) string {
	href = strings.Split(href, "?")[0]
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}
	if strings.HasPrefix(href, "/") {
		return strings.TrimRight(base, "/") + href
	}
	return strings.TrimRight(base, "/") + "/" + href
}

func stripTags(s string) string {
	re := regexp.MustCompile(`<[^>]+>`)
	return strings.TrimSpace(htmlUnescape(re.ReplaceAllString(s, " ")))
}

func htmlUnescape(s string) string {
	r := strings.NewReplacer("&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", "\"", "&#39;", "'")
	return r.Replace(s)
}
