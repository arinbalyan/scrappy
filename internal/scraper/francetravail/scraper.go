package francetravail

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const (
	apiURL     = "https://api.francetravail.io/partenaire/offresdemploi/v2/offres/search"
	tokenURL   = "https://entreprise.francetravail.fr/connexion/oauth2/access_token?realm=/partenaire"
	maxResults = 50
)

// Scraper fetches jobs from the France Travail (Pôle emploi) API.
type Scraper struct {
	client       *http.Client
	apiURL       string
	clientID     string
	clientSecret string
	mu           sync.Mutex
	accessToken  string
	tokenExpires *time.Time
}

// New creates a new France Travail scraper. If client is nil a default one is used.
// Reads FRANCETRAVAIL_CLIENT_ID and FRANCETRAVAIL_CLIENT_SECRET from environment.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Timeout: 20 * time.Second})
	}
	return &Scraper{
		client:       client,
		apiURL:       apiURL,
		clientID:     os.Getenv("FRANCETRAVAIL_CLIENT_ID"),
		clientSecret: os.Getenv("FRANCETRAVAIL_CLIENT_SECRET"),
	}
}

// NewWithAPIURL creates a new scraper with a custom endpoint (used in tests).
func NewWithAPIURL(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.apiURL = endpoint
	}
	return s
}

// SiteName returns the site identifier.
func (s *Scraper) SiteName() model.Site { return model.SiteFranceTravail }

// --- API response types ---

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

type searchResponse struct {
	Resultats []offer `json:"resultats"`
}

type offer struct {
	ID          string        `json:"id"`
	Intitule    string        `json:"intitule"`
	Description string        `json:"description"`
	DateCreation string       `json:"dateCreation"`
	LieuTravail *lieuTravail  `json:"lieuTravail,omitempty"`
	Entreprise  *entreprise   `json:"entreprise,omitempty"`
	OrigineOffre *origineOffre `json:"origineOffre,omitempty"`
}

type lieuTravail struct {
	Libelle string `json:"libelle"`
}

type entreprise struct {
	Nom  string `json:"nom"`
	Logo string `json:"logo"`
}

type origineOffre struct {
	URLOrigine string `json:"urlOrigine"`
}

// Scrape fetches jobs from France Travail.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{
		"site":           s.SiteName(),
		"results_wanted": input.ResultsWanted,
		"search_term":    input.SearchTerm,
	})

	if s.clientID == "" || s.clientSecret == "" {
		return nil, fmt.Errorf("francetravail: FRANCETRAVAIL_CLIENT_ID and FRANCETRAVAIL_CLIENT_SECRET must be set")
	}

	token, err := s.getAccessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("francetravail: %w", err)
	}

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = 25
	}
	if wanted > maxResults {
		wanted = maxResults
	}

	u, _ := url.Parse(s.apiURL)
	q := url.Values{}
	q.Set("range", fmt.Sprintf("0-%d", wanted-1))
	if input.SearchTerm != "" {
		q.Set("motsCles", input.SearchTerm)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("francetravail: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "EverJobs/1.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("francetravail: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("francetravail: status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("francetravail: read: %w", err)
	}

	var searchResp searchResponse
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return nil, fmt.Errorf("francetravail: decode: %w", err)
	}

	if len(searchResp.Resultats) == 0 {
		return nil, fmt.Errorf("francetravail: no jobs returned")
	}

	out := make([]model.JobPost, 0, wanted)
	for _, o := range searchResp.Resultats {
		if len(out) >= wanted {
			break
		}
		title := strings.TrimSpace(o.Intitule)
		if title == "" {
			continue
		}

		job := model.JobPost{
			ID:          "francetravail-" + o.ID,
			Title:       title,
			Description: strings.TrimSpace(o.Description),
			Site:        string(s.SiteName()),
			Location:    model.Location{Country: "France"},
		}

		// Company
		if o.Entreprise != nil {
			job.CompanyName = strings.TrimSpace(o.Entreprise.Nom)
			job.CompanyLogo = strings.TrimSpace(o.Entreprise.Logo)
		}

		// Job URL
		if o.OrigineOffre != nil && strings.TrimSpace(o.OrigineOffre.URLOrigine) != "" {
			job.JobURL = strings.TrimSpace(o.OrigineOffre.URLOrigine)
		} else {
			job.JobURL = fmt.Sprintf("https://candidat.francetravail.fr/offres/recherche/detail/%s", o.ID)
		}

		// Location city
		if o.LieuTravail != nil && strings.TrimSpace(o.LieuTravail.Libelle) != "" {
			job.Location.City = strings.TrimSpace(o.LieuTravail.Libelle)
		}

		// DatePosted
		if strings.TrimSpace(o.DateCreation) != "" {
			job.DatePosted = parseDate(o.DateCreation)
		}

		out = append(out, job)
	}

	util.Debug("scraper_done", map[string]any{
		"site": s.SiteName(),
		"jobs": len(out),
	})

	if !util.HasMeaningfulJobs(out) {
		return nil, fmt.Errorf("francetravail: no parseable jobs")
	}
	return out, nil
}

// getAccessToken obtains or refreshes the OAuth2 access token.
func (s *Scraper) getAccessToken(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.accessToken != "" && s.tokenExpires != nil && time.Now().Before(*s.tokenExpires) {
		return s.accessToken, nil
	}

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", s.clientID)
	form.Set("client_secret", s.clientSecret)
	form.Set("scope", "api_offresdemploiv2 o2dsoffre")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("token request: status %d", resp.StatusCode)
	}

	body, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return "", fmt.Errorf("token read: %w", err)
	}

	var tr tokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return "", fmt.Errorf("token decode: %w", err)
	}

	s.accessToken = tr.AccessToken
	exp := time.Now().Add(time.Duration(tr.ExpiresIn-60) * time.Second)
	s.tokenExpires = &exp
	return s.accessToken, nil
}

// parseDate parses an RFC3339 or ISO date string.
func parseDate(v string) *time.Time {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return &t
	}
	if t, err := time.Parse("2006-01-02T15:04:05", v); err == nil {
		return &t
	}
	if t, err := time.Parse("2006-01-02", v); err == nil {
		return &t
	}
	return nil
}
