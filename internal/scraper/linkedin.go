package scraper

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/arinbalyan/scrappy/internal/config"
	"github.com/arinbalyan/scrappy/internal/types"
)

type linkedInScraper struct {
	id        string
	search    string
	location  string
	results   int
	fetchDesc bool
	client    *http.Client
}

func NewLinkedIn(s config.Site) *linkedInScraper {
	r := 50
	if s.Results > 0 {
		r = s.Results
	}
	fd := false
	if s.FetchDescription != nil {
		fd = *s.FetchDescription
	}
	return &linkedInScraper{
		id:        s.ID,
		search:    s.Search,
		location:  s.Location,
		results:   r,
		fetchDesc: fd,
		client:    &http.Client{Timeout: 30 * time.Second},
	}
}

func runLinkedIn(s config.Site) func(context.Context, string) ([]types.JobPosting, error) {
	sc := NewLinkedIn(s)
	return func(ctx context.Context, query string) ([]types.JobPosting, error) {
		return sc.Scrape(ctx, query)
	}
}

func (l *linkedInScraper) Name() string { return l.id }

func (l *linkedInScraper) Scrape(ctx context.Context, query string) ([]types.JobPosting, error) {
	if query != "" && l.search == "" {
		l.search = query
	}
	if l.search == "" {
		return nil, nil
	}

	searchURL := "https://www.linkedin.com/jobs/search/?keywords=" +
		urlEncode(l.search)
	if l.location != "" {
		searchURL += "&location=" + urlEncode(l.location)
	}
	searchURL += "&start=0"

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := l.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("linkedin search request: %w", err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("linkedin search HTTP %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(b)))
	if err != nil {
		return nil, fmt.Errorf("linkedin parse html: %w", err)
	}

	var jobs []types.JobPosting

	// LinkedIn job cards — class is base-search-card, not job-search-card
	doc.Find(".base-search-card, .job-search-card").Each(func(i int, s *goquery.Selection) {
		if len(jobs) >= l.results {
			return
		}

		job := types.JobPosting{Source: l.id}

		// Title
		job.Title = strings.TrimSpace(s.Find(".base-search-card__title").Text())

		// Company + URL
		subtitleSel := s.Find(".base-search-card__subtitle")
		job.Company = strings.TrimSpace(subtitleSel.Text())
		if href, ok := subtitleSel.Find("a").Attr("href"); ok && !strings.HasPrefix(href, "http") {
			job.CompanyURL = "https://www.linkedin.com" + href
		}

		// Location
		job.Location = strings.TrimSpace(s.Find(".job-search-card__location").Text())

		// Posted time
		dateText := strings.TrimSpace(s.Find(".job-search-card__listdate").Text())
		if pt, ok := parseLinkedInDate(dateText); ok {
			job.PostedAt = &pt
		}

		// Job URL from the card link
		if href, ok := s.Find("a.base-card__full-link, .job-search-card__title-link").Attr("href"); ok {
			if !strings.HasPrefix(href, "http") {
				href = "https://www.linkedin.com" + href
			}
			job.URL = href
			if idx := strings.LastIndex(href, "/"); idx >= 0 {
				job.ID = href[idx+1:]
			}
		}
		if job.ID == "" {
			job.ID = fmt.Sprintf("%s-%d", l.id, i)
		}

		jobs = append(jobs, job)
	})

	// Fallback: alternate card structure
	if len(jobs) == 0 {
		doc.Find("li.jobs-search-results__list-item").Each(func(i int, s *goquery.Selection) {
			if len(jobs) >= l.results {
				return
			}
			job := types.JobPosting{Source: l.id}
			job.Title = strings.TrimSpace(s.Find("h3").Text())
			job.Company = strings.TrimSpace(s.Find("h4").Text())
			job.Location = strings.TrimSpace(s.Find(".job-search-card__location").Text())
			href, _ := s.Find("a").Attr("href")
			if href != "" && !strings.HasPrefix(href, "http") {
				href = "https://www.linkedin.com" + href
			}
			job.URL = href
			if idx := strings.LastIndex(job.URL, "/"); idx >= 0 {
				job.ID = job.URL[idx+1:]
			}
			if job.ID == "" {
				job.ID = fmt.Sprintf("%s-%d", l.id, i)
			}
			jobs = append(jobs, job)
		})
	}

	// Fetch full descriptions if requested
	if l.fetchDesc && len(jobs) > 0 {
		for i := range jobs {
			desc, err := fetchLinkedInJobDesc(ctx, l.client, jobs[i].URL)
			if err == nil && desc != "" {
				jobs[i].Description = desc
			}
			// Throttle
			if i > 0 && i%5 == 0 {
				select {
				case <-time.After(500 * time.Millisecond):
				case <-ctx.Done():
					return jobs, ctx.Err()
				}
			}
		}
	}

	return jobs, nil
}

func fetchLinkedInJobDesc(ctx context.Context, client *http.Client, url string) (string, error) {
	if url == "" {
		return "", nil
	}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(b)))
	if err != nil {
		return "", err
	}

	var desc string
	doc.Find(".description__text, .show-more-less-html__markup, .jobs-description-content__text").Each(func(i int, s *goquery.Selection) {
		if i == 0 {
			desc = strings.TrimSpace(s.Text())
		}
	})
	return desc, nil
}

func parseLinkedInDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	if strings.Contains(s, "hour") || strings.Contains(s, "minute") {
		return time.Now(), true
	}
	if strings.Contains(s, "day") {
		var n int
		fmt.Sscanf(s, "%d", &n)
		if n > 0 {
			return time.Now().AddDate(0, 0, -n), true
		}
	}
	if strings.Contains(s, "week") {
		var n int
		fmt.Sscanf(s, "%d", &n)
		if n > 0 {
			return time.Now().AddDate(0, 0, -n*7), true
		}
	}
	if strings.Contains(s, "month") {
		var n int
		fmt.Sscanf(s, "%d", &n)
		if n > 0 {
			return time.Now().AddDate(0, -n, 0), true
		}
	}
	return time.Time{}, false
}

func urlEncode(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, " ", "%20"), ",", "%2C")
}
