package personio

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/scraper/ats"
	"github.com/arinbalyan/scrappy/internal/util"
)

type Scraper struct {
	client *http.Client
	apiURL string
}

func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Timeout: 20 * time.Second})
	}
	return &Scraper{client: client}
}

func NewWithAPIURL(client *http.Client, apiURL string) *Scraper {
	s := New(client)
	if strings.TrimSpace(apiURL) != "" {
		s.apiURL = apiURL
	}
	return s
}

func (s *Scraper) SiteName() model.Site { return model.SitePersonio }

// XML feed parsing types
type personioXML struct {
	XMLName   xml.Name          `xml:"workzag-jobs"`
	Positions []personioXMLPosition `xml:"position"`
}

type personioXMLPosition struct {
	ID               string                     `xml:"id"`
	Name             string                     `xml:"name"`
	Office           string                     `xml:"office"`
	Department       string                     `xml:"department"`
	EmploymentType   string                     `xml:"employmentType"`
	Seniority        string                     `xml:"seniority"`
	Schedule         string                     `xml:"schedule"`
	Keywords         string                     `xml:"keywords"`
	CreatedAt        string                     `xml:"createdAt"`
	JobDescriptions  []personioXMLJobDescription `xml:"jobDescriptions>jobDescription"`
}

type personioXMLJobDescription struct {
	Name  string `xml:"name"`
	Value string `xml:"value"`
}

func (s *Scraper) buildURL(slug string) string {
	if s.apiURL != "" {
		return s.apiURL
	}
	return fmt.Sprintf("https://%s.jobs.personio.de/xml?language=en", url.PathEscape(slug))
}

func (s *Scraper) fallbackURL(slug string) string {
	return fmt.Sprintf("https://%s.jobs.personio.com/xml?language=en", url.PathEscape(slug))
}

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{"site": s.SiteName(), "results_wanted": input.ResultsWanted})

	seeds, src := ats.ResolveSeeds(input.SearchTerm, "SCRAPPY_PERSONIO_SEEDS")
	if len(seeds) == 0 {
		return nil, fmt.Errorf("personio no seeds: set SCRAPPY_PERSONIO_SEEDS or pass a company slug in --search (e.g. --search 'acme' resolves to acme.jobs.personio.de)")
	}
	util.Debug("personio_seeds", map[string]any{"seeds": seeds, "src": src})

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 100
	}

	out := make([]model.JobPost, 0, wanted)
	for _, slug := range seeds {
		if len(out) >= wanted {
			break
		}

		positions, err := s.fetchXML(ctx, slug)
		if err != nil {
			util.Warn("personio_fetch_fail", map[string]any{"slug": slug, "err": err.Error()})
			continue
		}

		for _, pos := range positions {
			if len(out) >= wanted {
				break
			}
			jp := s.mapPosition(pos, slug)
			if jp != nil {
				out = append(out, *jp)
			}
		}
	}

	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("personio no parseable jobs")
	}
	return out, nil
}

func (s *Scraper) fetchXML(ctx context.Context, slug string) ([]personioXMLPosition, error) {
	urls := []string{s.buildURL(slug)}
	// If using default URL, also try .com fallback
	if s.apiURL == "" {
		urls = append(urls, s.fallbackURL(slug))
	}

	var lastErr error
	for _, u := range urls {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("Accept", "application/xml, text/xml")
		req.Header.Set("User-Agent", "Mozilla/5.0")

		resp, err := s.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			lastErr = fmt.Errorf("status %d", resp.StatusCode)
			continue
		}

		var parsed personioXML
		if err := xml.Unmarshal(body, &parsed); err != nil {
			lastErr = err
			continue
		}

		if len(parsed.Positions) > 0 {
			return parsed.Positions, nil
		}
		lastErr = fmt.Errorf("no positions found")
	}
	return nil, lastErr
}

func (s *Scraper) mapPosition(pos personioXMLPosition, slug string) *model.JobPost {
	title := strings.TrimSpace(pos.Name)
	if title == "" {
		return nil
	}

	// Combine descriptions
	var descParts []string
	for _, d := range pos.JobDescriptions {
		if v := strings.TrimSpace(d.Value); v != "" {
			descParts = append(descParts, v)
		}
	}
	desc := strings.Join(descParts, "\n")

	loc := model.Location{}
	if pos.Office != "" {
		loc.City = strings.TrimSpace(pos.Office)
	}

	jobURL := fmt.Sprintf("https://%s.jobs.personio.de/job/%s", url.PathEscape(slug), pos.ID)

	skills := strings.Split(pos.Keywords, ",")
	var cleanSkills []string
	for _, s := range skills {
		if v := strings.TrimSpace(s); v != "" {
			cleanSkills = append(cleanSkills, v)
		}
	}

	jp := &model.JobPost{
		ID:          "personio-" + pos.ID,
		Title:       title,
		CompanyName: slug,
		JobURL:      jobURL,
		Location:    loc,
		Description: util.StripHTML(desc),
		Site:        string(model.SitePersonio),
		Department:  strings.TrimSpace(pos.Department),
		Skills:      cleanSkills,
	}

	if pos.CreatedAt != "" {
		jp.DatePosted = util.ParseDatePosted(pos.CreatedAt)
	}

	return jp
}
