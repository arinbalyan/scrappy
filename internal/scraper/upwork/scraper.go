package upwork

import (
	"bytes"
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

const (
	defaultWanted  = 20
	apiBase        = "https://www.upwork.com"
	tokenPath      = "/api/auth/v3/oauth/token"
	graphqlPath    = "/api/graphql/v1"
	scope          = "hr_skills_cw_jobs_search"
	grantTypeCC    = "client_credentials"
	defaultSort    = "RECENCY"
	jsonContentType = "application/json"
)

// ---- Auth types ----

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
}

// ---- GraphQL types ----

type graphqlRequest struct {
	Query     string `json:"query"`
	Variables string `json:"variables"`
}

type graphqlResponse struct {
	Data struct {
		MarketplaceJobPostings *marketplaceConnection `json:"marketplaceJobPostings,omitempty"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors,omitempty"`
}

type marketplaceConnection struct {
	TotalCount int             `json:"totalCount"`
	Edges      []jobPostingEdge `json:"edges"`
}

type jobPostingEdge struct {
	Node jobPostingNode `json:"node"`
}

type jobPostingNode struct {
	ID          string        `json:"id"`
	Ciphertext  string        `json:"ciphertext"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	CreatedDateTime string    `json:"createdDateTime"`
	Duration    string        `json:"duration"`
	Engagement  string        `json:"engagement"`
	Amount      *amountField  `json:"amount,omitempty"`
	WeeklyBudget *amountField `json:"weeklyBudget,omitempty"`
	Category    *namedField   `json:"category,omitempty"`
	Subcategory *namedField   `json:"subcategory,omitempty"`
	Skills      []skillField  `json:"skills,omitempty"`
	Client      *clientField  `json:"client,omitempty"`
	ContractorTier string     `json:"contractorTier"`
}

type amountField struct {
	Amount       string `json:"amount"`
	CurrencyCode string `json:"currencyCode"`
}

type namedField struct {
	Name string `json:"name"`
}

type skillField struct {
	Name string `json:"name"`
}

type clientField struct {
	TotalPostedJobs *int `json:"totalPostedJobs,omitempty"`
	TotalHires      *int `json:"totalHires,omitempty"`
}

const jobSearchQuery = `
query JobSearch($searchTerm: String, $first: Int, $sortField: MarketplaceJobPostingSortField) {
  marketplaceJobPostings(
    marketPlaceJobFilter: {
      searchTerm_eq: { andTerms_all: $searchTerm }
    }
    searchType: USER_JOBS_SEARCH
    sortAttributes: { field: $sortField, sortOrder: DESC }
    pagination: { first: $first }
  ) {
    totalCount
    edges {
      node {
        id
        ciphertext
        title
        description
        createdDateTime
        duration
        engagement
        amount { amount currencyCode }
        weeklyBudget { amount currencyCode }
        category { name }
        subcategory { name }
        skills { name }
        client { totalPostedJobs totalHires }
        contractorTier
      }
    }
  }
}`

// Scraper fetches jobs from the Upwork GraphQL API via OAuth2 client_credentials.
type Scraper struct {
	client       *http.Client
	clientID     string
	clientSecret string
	apiURL       string
}

// New creates a new Upwork scraper. Requires env vars UPWORK_CLIENT_ID and UPWORK_CLIENT_SECRET.
func New(client *http.Client) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Timeout: 30 * time.Second})
	}
	return &Scraper{
		client:       client,
		clientID:     os.Getenv("UPWORK_CLIENT_ID"),
		clientSecret: os.Getenv("UPWORK_CLIENT_SECRET"),
		apiURL:       apiBase,
	}
}

// NewWithClient creates a new scraper with explicit credentials (used in tests).
func NewWithClient(client *http.Client, clientID, clientSecret string) *Scraper {
	if client == nil {
		client = util.NewHTTPClient(util.ClientOptions{Timeout: 30 * time.Second})
	}
	return &Scraper{
		client:       client,
		clientID:     clientID,
		clientSecret: clientSecret,
		apiURL:       apiBase,
	}
}

// NewWithBaseURL creates a new scraper with a custom API base URL (used in tests).
func NewWithBaseURL(client *http.Client, baseURL, clientID, clientSecret string) *Scraper {
	s := NewWithClient(client, clientID, clientSecret)
	if strings.TrimSpace(baseURL) != "" {
		s.apiURL = strings.TrimSpace(baseURL)
	}
	return s
}

// SiteName returns the site identifier.
func (s *Scraper) SiteName() model.Site { return model.SiteUpwork }

// SiteNameWithStatus returns the site identifier and whether the scraper is configured.
func (s *Scraper) IsConfigured() bool {
	return s.clientID != "" && s.clientSecret != ""
}

// Scrape fetches jobs from the Upwork GraphQL API.
func (s *Scraper) Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error) {
	util.Debug("scraper_start", map[string]any{
		"site":           s.SiteName(),
		"results_wanted": input.ResultsWanted,
		"search_term":    input.SearchTerm,
	})

	if !s.IsConfigured() {
		return nil, fmt.Errorf("upwork: not configured; set UPWORK_CLIENT_ID and UPWORK_CLIENT_SECRET env vars")
	}

	wanted := input.ResultsWanted
	if wanted <= 0 {
		wanted = defaultWanted
	}

	// Obtain an access token
	token, err := s.obtainToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("upwork: token: %w", err)
	}

	// Build GraphQL request
	vars := map[string]any{
		"searchTerm": input.SearchTerm,
		"first":      wanted,
		"sortField":  defaultSort,
	}
	varsJSON, _ := json.Marshal(vars)

	gqlReq := graphqlRequest{
		Query:     jobSearchQuery,
		Variables: string(varsJSON),
	}
	body, _ := json.Marshal(gqlReq)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.apiURL+graphqlPath, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("upwork: build request: %w", err)
	}
	req.Header.Set("Content-Type", jsonContentType)
	req.Header.Set("Accept", jsonContentType)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "scrappy/0.1.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upwork: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
		return nil, fmt.Errorf("upwork: status %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	bodyBytes, err := util.ReadBodyLimited(resp.Body, util.DefaultMaxBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("upwork: read: %w", err)
	}

	var gqlResp graphqlResponse
	if err := json.Unmarshal(bodyBytes, &gqlResp); err != nil {
		return nil, fmt.Errorf("upwork: decode: %w", err)
	}

	if len(gqlResp.Errors) > 0 {
		return nil, fmt.Errorf("upwork: GraphQL errors: %s", gqlResp.Errors[0].Message)
	}

	if gqlResp.Data.MarketplaceJobPostings == nil {
		return nil, fmt.Errorf("upwork: no data in response")
	}

	edges := gqlResp.Data.MarketplaceJobPostings.Edges
	jobs := make([]model.JobPost, 0, len(edges))
	for _, edge := range edges {
		job, err := processNode(edge.Node)
		if err != nil {
			continue
		}
		jobs = append(jobs, job)
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

// obtainToken gets an OAuth2 access token using client_credentials.
func (s *Scraper) obtainToken(ctx context.Context) (string, error) {
	body := fmt.Sprintf(
		"grant_type=%s&client_id=%s&client_secret=%s&scope=%s",
		grantTypeCC, s.clientID, s.clientSecret, scope,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.apiURL+tokenPath, strings.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", jsonContentType)
	req.Header.Set("User-Agent", "scrappy/0.1.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("token status %d", resp.StatusCode)
	}

	var token tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return "", fmt.Errorf("token decode: %w", err)
	}

	if token.AccessToken == "" {
		return "", fmt.Errorf("empty access token")
	}

	return token.AccessToken, nil
}

// processNode converts a GraphQL job posting node to a JobPost.
func processNode(node jobPostingNode) (model.JobPost, error) {
	if node.ID == "" || node.Title == "" {
		return model.JobPost{}, fmt.Errorf("empty id or title")
	}

	jobURL := fmt.Sprintf("https://www.upwork.com/jobs/%s", node.ID)
	if node.Ciphertext != "" {
		jobURL = fmt.Sprintf("https://www.upwork.com/jobs/%s", node.Ciphertext)
	}

	desc := strings.TrimSpace(node.Description)

	// Parse compensation
	var comp *model.Compensation
	if node.Amount != nil && node.Amount.Amount != "" {
		amt := parseFloatOrZero(node.Amount.Amount)
		comp = &model.Compensation{
			Interval: "fixed",
			MinAmount: &amt,
			MaxAmount: &amt,
			Currency:  node.Amount.CurrencyCode,
		}
		if comp.Currency == "" {
			comp.Currency = "USD"
		}
	} else if node.WeeklyBudget != nil && node.WeeklyBudget.Amount != "" {
		amt := parseFloatOrZero(node.WeeklyBudget.Amount)
		comp = &model.Compensation{
			Interval: "weekly",
			MinAmount: &amt,
			MaxAmount: &amt,
			Currency:  node.WeeklyBudget.CurrencyCode,
		}
		if comp.Currency == "" {
			comp.Currency = "USD"
		}
	}

	// Parse date
	var datePosted *time.Time
	if node.CreatedDateTime != "" {
		datePosted = parseDate(node.CreatedDateTime)
	}

	// Detect remote
	titleDesc := strings.ToLower(node.Title + " " + desc)
	isRemote := strings.Contains(titleDesc, "remote") ||
		strings.Contains(titleDesc, "work from home") ||
		strings.Contains(titleDesc, "wfh")

	// Map engagement to job type
	jobType := ""
	if node.Engagement != "" {
		eng := strings.ToLower(node.Engagement)
		if strings.Contains(eng, "full") {
			jobType = "fulltime"
		} else if strings.Contains(eng, "part") {
			jobType = "parttime"
		} else if strings.Contains(eng, "contract") || strings.Contains(eng, "hourly") {
			jobType = "contract"
		}
	}

	// Build enriched description
	meta := make([]string, 0, 8)
	if node.Category != nil && node.Category.Name != "" {
		meta = append(meta, "Category: "+node.Category.Name)
	}
	if node.Subcategory != nil && node.Subcategory.Name != "" {
		meta = append(meta, "Subcategory: "+node.Subcategory.Name)
	}
	if len(node.Skills) > 0 {
		skillNames := make([]string, 0, len(node.Skills))
		for _, s := range node.Skills {
			if s.Name != "" {
				skillNames = append(skillNames, s.Name)
			}
		}
		if len(skillNames) > 0 {
			meta = append(meta, "Skills: "+strings.Join(skillNames, ", "))
		}
	}
	if node.ContractorTier != "" {
		meta = append(meta, "Experience Level: "+humanizeTier(node.ContractorTier))
	}
	if node.Duration != "" {
		meta = append(meta, "Duration: "+node.Duration)
	}
	if node.Client != nil {
		clientMeta := make([]string, 0, 2)
		if node.Client.TotalPostedJobs != nil {
			clientMeta = append(clientMeta, fmt.Sprintf("%d jobs posted", *node.Client.TotalPostedJobs))
		}
		if node.Client.TotalHires != nil {
			clientMeta = append(clientMeta, fmt.Sprintf("%d hires", *node.Client.TotalHires))
		}
		if len(clientMeta) > 0 {
			meta = append(meta, "Client: "+strings.Join(clientMeta, ", "))
		}
	}

	enrichedDesc := desc
	if len(meta) > 0 {
		metaStr := strings.Join(meta, "\n")
		if enrichedDesc != "" {
			enrichedDesc = enrichedDesc + "\n\n---\n" + metaStr
		} else {
			enrichedDesc = metaStr
		}
	}

	return model.JobPost{
		ID:          "upwork-" + node.ID,
		Title:       node.Title,
		CompanyName: "Upwork Client",
		JobURL:      jobURL,
		Description: enrichedDesc,
		Compensation: comp,
		DatePosted:  datePosted,
		IsRemote:    isRemote,
		JobType:     jobType,
		Site:        string(model.SiteUpwork),
		ApplyMethod: "external_url",
	}, nil
}

// humanizeTier converts Upwork contractor tier codes to readable labels.
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

// parseDate parses a date string.
func parseDate(v string) *time.Time {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05.000Z",
		"2006-01-02",
	}
	for _, f := range formats {
		t, err := time.Parse(f, v)
		if err == nil {
			return &t
		}
	}
	return nil
}

// parseFloatOrZero parses a float string, returns 0 on error.
func parseFloatOrZero(s string) float64 {
	var v float64
	if _, err := fmt.Sscanf(s, "%f", &v); err != nil {
		return 0
	}
	return v
}
