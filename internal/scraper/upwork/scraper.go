package upwork

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/arinbalyan/scrappy/internal/model"
	"github.com/arinbalyan/scrappy/internal/util"
)

const (
	graphqlURL    = "https://www.upwork.com/api/graphql/v1"
	tokenURL      = "https://www.upwork.com/api/v3/oauth/token"
	defaultWanted  = 20
)

// Scraper fetches jobs from the Upwork GraphQL API.
type Scraper struct {
	client       *http.Client
	graphqlURL   string
	clientID     string
	clientSecret string
	accessToken  string
}

// New creates a new Upwork scraper. Reads UPWORK_CLIENT_ID and UPWORK_CLIENT_SECRET from environment.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{
			Timeout:  30 * time.Second,
			Retries:  2,
		})
	}
	return &Scraper{client: client, graphqlURL: graphqlURL}
}

// NewWithGraphQLURL creates a scraper with a custom GraphQL endpoint (used in tests).
func NewWithGraphQLURL(client *http.Client, endpoint string) *Scraper {
	s := New(client)
	if strings.TrimSpace(endpoint) != "" {
		s.graphqlURL = strings.TrimSpace(endpoint)
	}
	return s
}

// NewWithCredentials creates a scraper with explicit credentials.
func NewWithCredentials(client *http.Client, clientID, clientSecret string) *Scraper {
	s := New(client)
	s.clientID = clientID
	s.clientSecret = clientSecret
	return s
}

// NewWithToken creates a scraper with a pre-existing access token.
func NewWithToken(client *http.Client, token string) *Scraper {
	s := New(client)
	s.accessToken = token
	return s
}

// SiteName returns the site identifier.
func (s *Scraper) SiteName() model.Site { return model.SiteUpwork }

// --- GraphQL types ---

type graphQLRequest struct {
	Query     string `json:"query"`
	Variables map[string]any `json:"variables"`
}

type graphQLResponse struct {
	Data *graphQLData `json:"data,omitempty"`
}

type graphQLData struct {
	MarketplaceJobPostings *jobPostingsConnection `json:"marketplaceJobPostings,omitempty"`
}

type jobPostingsConnection struct {
	Edges []jobEdge `json:"edges"`
}

type jobEdge struct {
	Node *jobNode `json:"node,omitempty"`
}

type jobNode struct {
	ID              string     `json:"id"`
	Ciphertext      string     `json:"ciphertext"`
	Title           string     `json:"title"`
	Description     string     `json:"description"`
	CreatedDateTime string     `json:"createdDateTime"`
	Duration        string     `json:"duration"`
	Engagement      string     `json:"engagement"`
	Amount          *money     `json:"amount,omitempty"`
	WeeklyBudget    *money     `json:"weeklyBudget,omitempty"`
	Category        *category  `json:"category,omitempty"`
	Subcategory     *category  `json:"subcategory,omitempty"`
	Skills          []skill    `json:"skills,omitempty"`
	ContractorTier  string     `json:"contractorTier"`
	Client          *clientInfo `json:"client,omitempty"`
}

type money struct {
	Amount       string `json:"amount"`
	CurrencyCode string `json:"currencyCode"`
}

type category struct {
	Name string `json:"name"`
}

type skill struct {
	Name string `json:"name"`
}

type clientInfo struct {
	TotalPostedJobs int `json:"totalPostedJobs"`
	TotalHires      int `json:"totalHires"`
}

// Scrape fetches jobs from Upwork using the GraphQL API.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{
		"site":           s.SiteName(),
		"results_wanted": input.ResultsWanted,
		"search_term":    input.SearchTerm,
	})

	// Credentials must be available via constructor
	if s.clientID == "" && s.accessToken == "" {
		return nil, fmt.Errorf("upwork: credentials required — set clientID+clientSecret or accessToken")
	}

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = defaultWanted
	}

	// Obtain access token if needed
	if s.accessToken == "" && s.clientID != "" && s.clientSecret != "" {
		token, err := s.obtainToken(ctx)
		if err != nil {
			return nil, fmt.Errorf("upwork: %w", err)
		}
		s.accessToken = token
	}

	searchTerm := strings.TrimSpace(input.SearchTerm)

	// Build GraphQL query
	query := `query JobSearch($searchTerm: String, $first: Int) {
		marketplaceJobPostings(
			marketPlaceJobFilter: {searchTerm_eq: {andTerms_all: $searchTerm}}
			searchType: USER_JOBS_SEARCH
			sortAttributes: {field: RECENCY, sortOrder: DESC}
			pagination: {first: $first}
		) {
			edges {
				node {
					id, ciphertext, title, description, createdDateTime
					duration, engagement
					amount { amount, currencyCode }
					weeklyBudget { amount, currencyCode }
					category { name }
					subcategory { name }
					skills { name }
					contractorTier
					client { totalPostedJobs, totalHires }
				}
			}
		}
	}`

	gqlReq := graphQLRequest{
		Query: query,
		Variables: map[string]any{
			"searchTerm": searchTerm,
			"first":      wanted,
		},
	}

	body, err := json.Marshal(gqlReq)
	if err != nil {
		return nil, fmt.Errorf("upwork: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.graphqlURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("upwork: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; scrappy/1.0)")
	if s.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+s.accessToken)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upwork: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("upwork: status %d", resp.StatusCode)
	}

	respBody, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("upwork: read: %w", err)
	}

	var gqlResp graphQLResponse
	if err := json.Unmarshal(respBody, &gqlResp); err != nil {
		return nil, fmt.Errorf("upwork: decode: %w", err)
	}

	if gqlResp.Data == nil || gqlResp.Data.MarketplaceJobPostings == nil {
		return nil, fmt.Errorf("upwork: no data in response")
	}

	jobs := make([]model.JobPost, 0, wanted)
	for _, edge := range gqlResp.Data.MarketplaceJobPostings.Edges {
		if len(jobs) >= wanted {
			break
		}
		job := processNode(edge.Node)
		if job != nil {
			jobs = append(jobs, *job)
		}
	}

	util.Debug("scraper_done", map[string]any{
		"site": s.SiteName(),
		"jobs": len(jobs),
	})

	if !util.HasMeaningfulJobs(jobs) {
		return nil, fmt.Errorf("upwork: no parseable jobs")
	}
	return jobs, nil
}

// obtainToken performs client_credentials OAuth2 token exchange.
func (s *Scraper) obtainToken(ctx context.Context) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", s.clientID)
	form.Set("client_secret", s.clientSecret)

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

	var tr struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
	}
	if err := json.Unmarshal(body, &tr); err != nil {
		return "", fmt.Errorf("token decode: %w", err)
	}
	return tr.AccessToken, nil
}

// processNode converts an Upwork GraphQL job node to a JobPost.
func processNode(node *jobNode) *model.JobPost {
	if node == nil || node.ID == "" || node.Title == "" {
		return nil
	}

	// Build job URL
	jobURL := "https://www.upwork.com/jobs/" + node.ID
	if node.Ciphertext != "" {
		jobURL = "https://www.upwork.com/jobs/" + node.Ciphertext
	}

	// Description with metadata
	desc := strings.TrimSpace(node.Description)
	meta := make([]string, 0, 5)
	if node.Category != nil && node.Category.Name != "" {
		meta = append(meta, "Category: "+node.Category.Name)
	}
	if node.Subcategory != nil && node.Subcategory.Name != "" {
		meta = append(meta, "Subcategory: "+node.Subcategory.Name)
	}
	if len(node.Skills) > 0 {
		skillNames := make([]string, len(node.Skills))
		for i, s := range node.Skills {
			skillNames[i] = s.Name
		}
		meta = append(meta, "Skills: "+strings.Join(skillNames, ", "))
	}
	if node.ContractorTier != "" {
		meta = append(meta, "Experience Level: "+humanizeTier(node.ContractorTier))
	}
	if node.Duration != "" {
		meta = append(meta, "Duration: "+node.Duration)
	}
	if node.Engagement != "" {
		meta = append(meta, "Engagement: "+node.Engagement)
	}
	if node.Client != nil {
		clientMeta := []string{}
		if node.Client.TotalPostedJobs > 0 {
			clientMeta = append(clientMeta, fmt.Sprintf("%d jobs posted", node.Client.TotalPostedJobs))
		}
		if node.Client.TotalHires > 0 {
			clientMeta = append(clientMeta, fmt.Sprintf("%d hires", node.Client.TotalHires))
		}
		if len(clientMeta) > 0 {
			meta = append(meta, "Client: "+strings.Join(clientMeta, ", "))
		}
	}
	if len(meta) > 0 {
		if desc != "" {
			desc = desc + "\n\n---\n" + strings.Join(meta, "\n")
		} else {
			desc = strings.Join(meta, "\n")
		}
	}

	// Compensation
	var comp *model.Compensation
	if node.Amount != nil && node.Amount.Amount != "" {
		amt := parseFloat(node.Amount.Amount)
		currency := node.Amount.CurrencyCode
		if currency == "" {
			currency = "USD"
		}
		comp = &model.Compensation{
			MinAmount: &amt,
			MaxAmount: &amt,
			Currency:  currency,
		}
	} else if node.WeeklyBudget != nil && node.WeeklyBudget.Amount != "" {
		amt := parseFloat(node.WeeklyBudget.Amount)
		currency := node.WeeklyBudget.CurrencyCode
		if currency == "" {
			currency = "USD"
		}
		comp = &model.Compensation{
			MinAmount: &amt,
			MaxAmount: &amt,
			Currency:  currency,
		}
	}

	// Date
	var datePosted *time.Time
	if node.CreatedDateTime != "" {
		if t, err := time.Parse(time.RFC3339, node.CreatedDateTime); err == nil {
			datePosted = &t
		} else if t, err := time.Parse("2006-01-02T15:04:05Z", node.CreatedDateTime); err == nil {
			datePosted = &t
		} else if t, err := time.Parse("2006-01-02", node.CreatedDateTime[:10]); err == nil {
			datePosted = &t
		}
	}

	// Detect remote
	titleAndDesc := strings.ToLower(node.Title + " " + node.Description)
	isRemote := strings.Contains(titleAndDesc, "remote") ||
		strings.Contains(titleAndDesc, "work from home") ||
		strings.Contains(titleAndDesc, "wfh")

	// Determine job type from engagement
	jobType := ""
	if node.Engagement != "" {
		eng := strings.ToLower(node.Engagement)
		if strings.Contains(eng, "full") {
			jobType = "fulltime"
		} else if strings.Contains(eng, "part") {
			jobType = "parttime"
		} else if strings.Contains(eng, "contract") || strings.Contains(eng, "hourly") || strings.Contains(eng, "project") {
			jobType = "contract"
		}
	}

	return &model.JobPost{
		ID:          "upwork-" + node.ID,
		Title:       node.Title,
		CompanyName: "Upwork Client",
		JobURL:      jobURL,
		Description: desc,
		Compensation: comp,
		IsRemote:    isRemote,
		DatePosted:  datePosted,
		JobType:     jobType,
		Site:        string(model.SiteUpwork),
	}
}

func humanizeTier(tier string) string {
	switch tier {
	case "ENTRY":
		return "Entry Level"
	case "INTERMEDIATE":
		return "Intermediate"
	case "EXPERT":
		return "Expert"
	default:
		return tier
	}
}

func parseFloat(s string) float64 {
	var f float64
	if _, err := fmt.Sscanf(s, "%f", &f); err == nil {
		return f
	}
	return 0
}
