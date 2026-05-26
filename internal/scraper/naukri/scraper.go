package naukri

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

	"github.com/arinbalyan/scrappy/internal/browser"
	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

var reNaukriSalary = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*-\s*(\d+(?:\.\d+)?)\s*(lacs|lakh|cr)`)

const defaultAPI = "https://www.naukri.com/jobapi/v3/search"

type placeholder struct {
	Type  string `json:"type"`
	Label string `json:"label"`
}

type Scraper struct {
	client         *http.Client
	apiURL         string
	browserCookies []browser.Cookie // cached from browser fallback
}

func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 3, Timeout: 18 * time.Second})
	}
	return &Scraper{client: client, apiURL: defaultAPI}
}

func NewWithAPIURL(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.apiURL = endpoint
	}
	return s
}

func (s *Scraper) SiteName() model.Site { return model.SiteNaukri }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 15
	}
	startPage := input.Offset/20 + 1
	if startPage <= 0 {
		startPage = 1
	}
	seen := map[string]struct{}{}
	out := make([]model.JobPost, 0, wanted)

	for page := startPage; len(out) < wanted && page <= startPage+8; page++ {
		u, _ := url.Parse(s.apiURL)
		q := u.Query()
		search := strings.TrimSpace(input.SearchTerm)
		q.Set("noOfResults", "20")
		q.Set("urlType", "search_by_keyword")
		q.Set("searchType", "adv")
		q.Set("keyword", search)
		q.Set("k", search)
		q.Set("pageNo", strconv.Itoa(page))
		q.Set("src", "jobsearchDesk")
		q.Set("latLong", "")
		if search != "" {
			q.Set("seoKey", strings.ReplaceAll(strings.ToLower(search), " ", "-")+"-jobs")
		}
		if strings.TrimSpace(input.Location) != "" {
			q.Set("location", strings.TrimSpace(input.Location))
		}
		if input.IsRemote {
			q.Set("remote", "true")
		}
		if input.HoursOld > 0 {
			q.Set("days", strconv.Itoa((input.HoursOld+23)/24))
		}
		u.RawQuery = q.Encode()

		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		applyNaukriHeaders(req)

		// If we have pre-fetched browser cookies, attach them.
		if s.browserCookies != nil {
			for _, c := range s.browserCookies {
				req.AddCookie(&http.Cookie{Name: c.Name, Value: c.Value})
			}
		}

		resp, err := s.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("naukri request: %w", err)
		}

		// On 406 (recaptcha required), try browser to get cookies and retry once.
		if resp.StatusCode == 406 && s.browserCookies == nil && browser.IsAvailable() {
			resp.Body.Close()
			if result, bErr := browser.FetchPage(ctx, "https://www.naukri.com", ""); bErr == nil && result.Status == 200 {
				s.browserCookies = result.Cookies
				// Retry this page with cookies.
				req2, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
				applyNaukriHeaders(req2)
				for _, c := range result.Cookies {
					req2.AddCookie(&http.Cookie{Name: c.Name, Value: c.Value})
				}
				resp2, retryErr := s.client.Do(req2)
				if retryErr != nil {
					return nil, fmt.Errorf("naukri request (with cookies): %w", retryErr)
				}
				resp = resp2
			}
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			resp.Body.Close()
			return nil, fmt.Errorf("naukri status %d", resp.StatusCode)
		}
		var parsed struct {
			JobDetails []struct {
				JobID               string `json:"jobId"`
				Title               string `json:"title"`
				CompanyName         string `json:"companyName"`
				JDURL               string `json:"jdURL"`
				JobDescription      string `json:"jobDescription"`
				CreatedDate         int64  `json:"createdDate"`
				FooterPlaceholder   string `json:"footerPlaceholderLabel"`
				ExperienceText      string `json:"experienceText"`
				TagsAndSkills       string `json:"tagsAndSkills"`
				LogoPath            string `json:"logoPath"`
				LogoPathV3          string `json:"logoPathV3"`
				Vacancy             int    `json:"vacancy"`
				Placeholders        []placeholder `json:"placeholders"`
			} `json:"jobDetails"`
		}
		err = json.NewDecoder(resp.Body).Decode(&parsed)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("naukri decode: %w", err)
		}
		if len(parsed.JobDetails) == 0 {
			break
		}
		for _, r := range parsed.JobDetails {
			if len(out) >= wanted {
				break
			}
			id := strings.TrimSpace(r.JobID)
			if id == "" {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			title := strings.TrimSpace(r.Title)
			company := strings.TrimSpace(r.CompanyName)
			if title == "" || company == "" {
				continue
			}
			city, state := naukriLocation(r.Placeholders)
			min, max, hasComp := naukriSalary(r.Placeholders)
			post := model.JobPost{ID: "nk-" + id, Title: title, CompanyName: company, JobURL: makeNaukriURL(r.JDURL, id), Description: strings.TrimSpace(r.JobDescription), Location: model.Location{City: city, State: state, Country: "India"}, ExperienceRange: strings.TrimSpace(r.ExperienceText), VacancyCount: r.Vacancy}
			if r.LogoPathV3 != "" {
				post.CompanyLogo = strings.TrimSpace(r.LogoPathV3)
			} else {
				post.CompanyLogo = strings.TrimSpace(r.LogoPath)
			}
			if hasComp {
				post.Compensation = &model.Compensation{Interval: model.IntervalYearly, MinAmount: &min, MaxAmount: &max, Currency: "INR"}
			}
			if t := parseNaukriDate(r.FooterPlaceholder, r.CreatedDate); t != nil {
				post.DatePosted = t
			}
			fullText := strings.ToLower(title + " " + post.Description + " " + city)
			post.IsRemote = strings.Contains(fullText, "remote") || strings.Contains(fullText, "work from home") || strings.Contains(fullText, "wfh")
			if strings.TrimSpace(r.TagsAndSkills) != "" {
				parts := strings.Split(r.TagsAndSkills, ",")
				skills := make([]string, 0, len(parts))
				for _, p := range parts {
					v := strings.TrimSpace(p)
					if v != "" {
						skills = append(skills, v)
					}
				}
				post.Skills = skills
			}
			out = append(out, post)
		}
	}
	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("naukri no parseable jobs")
	}
	return out, nil
}

func applyNaukriHeaders(req *http.Request) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36")
	req.Header.Set("appid", "109")
	req.Header.Set("systemid", "Naukri")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Origin", "https://www.naukri.com")
	req.Header.Set("Referer", "https://www.naukri.com/")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
}

func makeNaukriURL(jdURL, id string) string {
	jdURL = strings.TrimSpace(jdURL)
	if jdURL == "" {
		return "https://www.naukri.com/job/" + id
	}
	if strings.HasPrefix(jdURL, "http") {
		return jdURL
	}
	return "https://www.naukri.com" + jdURL
}

func naukriLocation(placeholders []placeholder) (string, string) {
	for _, p := range placeholders {
		if strings.EqualFold(strings.TrimSpace(p.Type), "location") {
			parts := strings.Split(strings.TrimSpace(p.Label), ",")
			city := strings.TrimSpace(parts[0])
			state := ""
			if len(parts) > 1 {
				state = strings.TrimSpace(parts[1])
			}
			return city, state
		}
	}
	return "", ""
}

func naukriSalary(placeholders []placeholder) (float64, float64, bool) {
	re := reNaukriSalary
	for _, p := range placeholders {
		if !strings.EqualFold(strings.TrimSpace(p.Type), "salary") {
			continue
		}
		m := re.FindStringSubmatch(strings.TrimSpace(p.Label))
		if len(m) != 4 {
			continue
		}
		min, _ := strconv.ParseFloat(m[1], 64)
		max, _ := strconv.ParseFloat(m[2], 64)
		unit := strings.ToLower(strings.TrimSpace(m[3]))
		switch unit {
		case "lacs", "lakh":
			min *= 100000
			max *= 100000
		case "cr":
			min *= 10000000
			max *= 10000000
		}
		return min, max, true
	}
	return 0, 0, false
}

func parseNaukriDate(label string, created int64) *time.Time {
	now := time.Now()
	l := strings.ToLower(strings.TrimSpace(label))
	if l != "" {
		if strings.Contains(l, "today") || strings.Contains(l, "just now") || strings.Contains(l, "few hours") {
			return &now
		}
		re := regexp.MustCompile(`(\d+)\s*day`)
		if m := re.FindStringSubmatch(l); len(m) > 1 {
			d, _ := strconv.Atoi(m[1])
			t := now.Add(-time.Duration(d) * 24 * time.Hour)
			return &t
		}
	}
	if created > 0 {
		t := time.UnixMilli(created)
		return &t
	}
	return nil
}
