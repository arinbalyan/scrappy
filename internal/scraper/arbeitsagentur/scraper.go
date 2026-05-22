package arbeitsagentur

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const defaultAPIURL = "https://rest.arbeitsagentur.de/jobboerse/jobsuche-service/pc/v4/jobs"

// apiResponse is the top-level response from the Arbeitsagentur Job Search API.
type apiResponse struct {
	Stellenangebote []apiJob `json:"stellenangebote"`
	MaxErgebnisse   int      `json:"maxErgebnisse"`
	Seite           int      `json:"seite"`
}

// apiJob represents a single Stellenangebot returned by the API.
type apiJob struct {
	RefNr                     string           `json:"refnr"`
	Titel                     string           `json:"titel"`
	Arbeitgeber               string           `json:"arbeitgeber"`
	Beruf                     string           `json:"beruf"`
	Arbeitsort                apiArbeitsort    `json:"arbeitsort"`
	Eintrittsdatum            string           `json:"eintrittsdatum"`
	AktuelleVeroeffentlichungsdatum string       `json:"aktuelleVeroeffentlichungsdatum"`
	ModifikationsTimestamp    int64            `json:"modifikationsTimestamp"`
	UepiAnpiLogo              string           `json:"uepiAnpiLogo"`
	ExterneUrl                string           `json:"externeUrl"`
	HomeOffice                bool             `json:"homeOffice"`
}

// apiArbeitsort represents the workplace location.
type apiArbeitsort struct {
	Ort      string  `json:"ort"`
	PLZ      string  `json:"plz"`
	Region   string  `json:"region"`
	Land     string  `json:"land"`
	Koordinaten struct {
		Lat float64 `json:"lat"`
		Lon float64 `json:"lon"`
	} `json:"koordinaten"`
}

type Scraper struct {
	client  *http.Client
	apiURL  string
	apiKey  string
}

func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Retries: 2, Timeout: 20 * time.Second})
	}
	return &Scraper{client: client, apiURL: defaultAPIURL, apiKey: strings.TrimSpace(os.Getenv("ARBEITSAGENTUR_API_KEY"))}
}

func NewWithURLs(client *http.Client, apiURL, apiKey string) *Scraper {
	s := New(client)
	if strings.TrimSpace(apiURL) != "" {
		s.apiURL = strings.TrimSpace(apiURL)
	}
	if strings.TrimSpace(apiKey) != "" {
		s.apiKey = strings.TrimSpace(apiKey)
	}
	return s
}

func (s *Scraper) SiteName() model.Site { return model.SiteArbeitsagentur }

func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 25
	}

	if s.apiKey == "" {
		return nil, fmt.Errorf("arbeitsagentur: ARBEITSAGENTUR_API_KEY not configured")
	}

	term := strings.TrimSpace(input.SearchTerm)
	loc := strings.TrimSpace(input.Location)
	jobs := make([]model.JobPost, 0, wanted)
	page := 0

	for len(jobs) < wanted {
		select {
		case <-ctx.Done():
			return jobs, ctx.Err()
		default:
		}

		page++
		pageJobs, hasMore, err := s.fetchPage(ctx, page, term, loc)
		if err != nil {
			if len(jobs) > 0 {
				break
			}
			return nil, fmt.Errorf("arbeitsagentur page %d: %w", page, err)
		}
		if len(pageJobs) == 0 {
			break
		}
		for _, j := range pageJobs {
			if len(jobs) >= wanted {
				break
			}
			jobs = append(jobs, j)
		}
		if !hasMore {
			break
		}
	}

	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("arbeitsagentur no parseable jobs")
	}
	return jobs, nil
}

func (s *Scraper) fetchPage(ctx context.Context, page int, term, loc string) ([]model.JobPost, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.apiURL, nil)
	if err != nil {
		return nil, false, fmt.Errorf("request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129 Safari/537.36")
	req.Header.Set("X-API-Key", s.apiKey)

	// Build query params per interop spec: was=search, wo=location, page, size
	qParams := req.URL.Query()
	if term != "" {
		qParams.Set("was", term)
	}
	if loc != "" {
		qParams.Set("wo", loc)
	}
	qParams.Set("seite", fmt.Sprintf("%d", page))
	qParams.Set("size", "100")
	req.URL.RawQuery = qParams.Encode()

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, false, fmt.Errorf("status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, false, fmt.Errorf("read: %w", err)
	}

	var parsed apiResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, false, fmt.Errorf("decode: %w", err)
	}
	return mapJobs(parsed.Stellenangebote), parsed.Seite*100 < parsed.MaxErgebnisse, nil
}

func mapJobs(raw []apiJob) []model.JobPost {
	out := make([]model.JobPost, 0, len(raw))
	for _, j := range raw {
		if strings.TrimSpace(j.RefNr) == "" {
			continue
		}
		jobURL := fmt.Sprintf("https://www.arbeitsagentur.de/jobsuche/suche?id=%s", j.RefNr)

		loc := model.Location{
			City:    strings.TrimSpace(j.Arbeitsort.Ort),
			State:   strings.TrimSpace(j.Arbeitsort.Region),
			Country: strings.TrimSpace(j.Arbeitsort.Land),
		}

		var datePosted *time.Time
		rawDate := j.AktuelleVeroeffentlichungsdatum
		if rawDate == "" {
			rawDate = j.Eintrittsdatum
		}
		if rawDate != "" {
			if t, err := parseGermanDate(rawDate); err == nil {
				datePosted = &t
			}
		}

		job := model.JobPost{
			ID:          "aa-" + strings.TrimSpace(j.RefNr),
			Title:       strings.TrimSpace(j.Titel),
			CompanyName: strings.TrimSpace(j.Arbeitgeber),
			JobURL:      jobURL,
			Location:    loc,
			IsRemote:    j.HomeOffice,
			DatePosted:  datePosted,
		}
		out = append(out, job)
	}
	return out
}

// parseGermanDate tries common German date formats used by the API.
func parseGermanDate(v string) (time.Time, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return time.Time{}, fmt.Errorf("empty date")
	}
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04:05.999",
		"2006-01-02T15:04:05",
		"02.01.2006",
		"02/01/2006",
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, v); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unknown date format: %q", v)
}
