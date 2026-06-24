package email

import (
	"container/list"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/arinbalyan/scrappy/internal/util"
)

// GitHubDiscoverer queries the public GitHub REST API for historical
// commit author emails of a given login. Confirmed via agent-browser:
// GET /repos/{owner}/{repo}/commits?author={login} returns commit.author.email
// with personal emails intact despite the "Keep my email private" setting
// (which only applies to new commits).
//
// Ponytail: stdlib net/http, JSON parsing, simple LRU cache. No
// graphql dependency. Falls back gracefully when the token is absent
// (60 req/h unauthenticated).
type GitHubDiscoverer struct {
	Token      string
	HTTPClient *http.Client
	UserAgent  string
	Cache      *ghLRU
	mu         sync.Mutex
}

// NewGitHubDiscoverer returns a discoverer with sensible defaults.
func NewGitHubDiscoverer(token string) *GitHubDiscoverer {
	return &GitHubDiscoverer{
		Token:      token,
		HTTPClient: &http.Client{Timeout: 15 * time.Second},
		UserAgent:  "scrappy/1.0 (+https://github.com/arinbalyan/scrappy)",
		Cache:      newGHLRU(256),
	}
}

// CommitsForAuthor returns the unique personal emails found in
// commit.author.email across the public commits of the given GitHub
// login. Filtered to exclude noreply hosts and bot suffixes.
//
// Returns nil on error; caller logs and moves on.
func (g *GitHubDiscoverer) CommitsForAuthor(ctx context.Context, login string) ([]string, error) {
	login = strings.TrimSpace(login)
	if login == "" {
		return nil, nil
	}

	// Check cache.
	if cached, ok := g.Cache.Get(login); ok {
		return cached, nil
	}

	// 1. Find the most-recently-pushed public repo for this user.
	reposURL := fmt.Sprintf("https://api.github.com/users/%s/repos?per_page=5&sort=pushed&type=public", url.PathEscape(login))
	repos, err := g.fetchJSON(ctx, reposURL, nil)
	if err != nil {
		return nil, fmt.Errorf("github_repos: %w", err)
	}

	type repoItem struct {
		Name          string `json:"name"`
		FullName      string `json:"full_name"`
		Private       bool   `json:"private"`
		Fork          bool   `json:"fork"`
		HasCommitsURL string `json:"commits_url"` // ends with {:sha}; trim
	}
	var reposList []repoItem
	if err := json.Unmarshal(repos, &reposList); err != nil {
		return nil, fmt.Errorf("github_repos_decode: %w", err)
	}

	// Find the first non-fork public repo.
	var target *repoItem
	for i := range reposList {
		if !reposList[i].Private && !reposList[i].Fork && reposList[i].HasCommitsURL != "" {
			target = &reposList[i]
			break
		}
	}
	if target == nil {
		// No public repos; cache the empty result to avoid retrying.
		g.Cache.Put(login, nil)
		return nil, nil
	}

	// 2. Query commits by author.
	commitsURL := fmt.Sprintf("https://api.github.com/repos/%s/commits?author=%s&per_page=10",
		url.PathEscape(target.FullName), url.QueryEscape(login))
	commits, err := g.fetchJSON(ctx, commitsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("github_commits: %w", err)
	}

	type commitItem struct {
		Author *struct {
			Name  string `json:"name"`
			Email string `json:"email"`
			Login string `json:"login"`
		} `json:"author"`
		Commit *struct {
			Author *struct {
				Name  string `json:"name"`
				Email string `json:"email"`
			} `json:"author"`
		} `json:"commit"`
	}
	var commitsList []commitItem
	if err := json.Unmarshal(commits, &commitsList); err != nil {
		return nil, fmt.Errorf("github_commits_decode: %w", err)
	}

	// 3. Collect unique non-noreply, non-bot emails.
	seen := make(map[string]bool, len(commitsList))
	out := make([]string, 0, len(commitsList))
	for _, c := range commitsList {
		var email string
		if c.Commit != nil && c.Commit.Author != nil {
			email = c.Commit.Author.Email
		}
		if email == "" && c.Author != nil {
			email = c.Author.Email
		}
		email = strings.TrimSpace(email)
		if email == "" {
			continue
		}
		if isGitHubNoreply(email) {
			continue
		}
		if seen[email] {
			continue
		}
		seen[email] = true
		out = append(out, email)
	}
	sort.Strings(out)

	g.Cache.Put(login, out)
	return out, nil
}

// CommitsForOrgMembers iterates the first N members of a GitHub org and
// runs CommitsForAuthor on each. Returns a map[login]emails. Stops
// after maxMembers or 30 API calls (whichever is smaller).
//
// Note: this requires the org to be public. Private org membership
// returns 404 from the public API.
func (g *GitHubDiscoverer) CommitsForOrgMembers(ctx context.Context, org string, maxMembers int) (map[string][]string, error) {
	org = strings.TrimSpace(org)
	if org == "" {
		return nil, nil
	}
	if maxMembers <= 0 {
		maxMembers = 10
	}
	if maxMembers > 30 {
		maxMembers = 30
	}

	membersURL := fmt.Sprintf("https://api.github.com/orgs/%s/public_members?per_page=%d",
		url.PathEscape(org), maxMembers)
	body, err := g.fetchJSON(ctx, membersURL, nil)
	if err != nil {
		return nil, fmt.Errorf("github_org_members: %w", err)
	}
	var members []struct {
		Login string `json:"login"`
	}
	if err := json.Unmarshal(body, &members); err != nil {
		return nil, fmt.Errorf("github_org_members_decode: %w", err)
	}

	out := make(map[string][]string, len(members))
	sem := make(chan struct{}, 3) // bound to 3 concurrent GitHub calls
	var mu sync.Mutex
	var wg sync.WaitGroup
	calls := 0
	maxCalls := 30
	for _, m := range members {
		if calls >= maxCalls {
			break
		}
		if err := ctx.Err(); err != nil {
			break
		}
		login := m.Login
		if login == "" {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(login string) {
			defer wg.Done()
			defer func() { <-sem }()
			emails, err := g.CommitsForAuthor(ctx, login)
			if err != nil {
				util.Debug("github_member_err", map[string]any{"login": login, "err": err.Error()})
				return
			}
			if len(emails) > 0 {
				mu.Lock()
				out[login] = emails
				mu.Unlock()
			}
		}(login)
		calls++
	}
	wg.Wait()
	return out, nil
}

// fetchJSON performs a GET against the GitHub API and returns the raw
// response body. Auth header is added only if a token is configured.
// Used internally by CommitsForAuthor and CommitsForOrgMembers.
func (g *GitHubDiscoverer) fetchJSON(ctx context.Context, rawURL string, _ []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", g.UserAgent)
	if g.Token != "" {
		req.Header.Set("Authorization", "Bearer "+g.Token)
	}

	resp, err := g.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("github_rate_limited (status %d)", resp.StatusCode)
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("github_not_found (status %d)", resp.StatusCode)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("github_status_%d", resp.StatusCode)
	}

	// Read at most 1 MiB.
	limited := http.MaxBytesReader(nil, resp.Body, 1<<20)
	buf := make([]byte, 0, 16<<10)
	tmp := make([]byte, 4096)
	for {
		n, err := limited.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			break
		}
	}
	return buf, nil
}

// isGitHubNoreply returns true for GitHub's privacy-protection addresses
// (noreply.github.com and the older users.noreply.github.com) and
// common bot patterns.
// OrgsForUser returns the organization logins the given user belongs to.
func (g *GitHubDiscoverer) OrgsForUser(ctx context.Context, login string) ([]string, error) {
	login = strings.TrimSpace(login)
	if login == "" {
		return nil, nil
	}
	url := fmt.Sprintf("https://api.github.com/users/%s/orgs", url.PathEscape(login))
	body, err := g.fetchJSON(ctx, url, nil)
	if err != nil {
		return nil, fmt.Errorf("github_orgs: %w", err)
	}
	var orgs []struct {
		Login string `json:"login"`
	}
	if err := json.Unmarshal(body, &orgs); err != nil {
		return nil, fmt.Errorf("github_orgs_decode: %w", err)
	}
	out := make([]string, 0, len(orgs))
	for _, o := range orgs {
		if o.Login != "" {
			out = append(out, o.Login)
		}
	}
	return out, nil
}

// ReposForOrg returns the full repo names (owner/name) for a GitHub org.
func (g *GitHubDiscoverer) ReposForOrg(ctx context.Context, org string) ([]string, error) {
	org = strings.TrimSpace(org)
	if org == "" {
		return nil, nil
	}
	url := fmt.Sprintf("https://api.github.com/orgs/%s/repos?per_page=50&sort=pushed&type=public", url.PathEscape(org))
	body, err := g.fetchJSON(ctx, url, nil)
	if err != nil {
		return nil, fmt.Errorf("github_org_repos: %w", err)
	}
	var repos []struct {
		FullName string `json:"full_name"`
		Fork     bool   `json:"fork"`
	}
	if err := json.Unmarshal(body, &repos); err != nil {
		return nil, fmt.Errorf("github_org_repos_decode: %w", err)
	}
	out := make([]string, 0, len(repos))
	for _, r := range repos {
		if !r.Fork && r.FullName != "" {
			out = append(out, r.FullName)
		}
	}
	return out, nil
}

// EmailsFromRepo returns unique personal emails from recent commits in a repo.
// Uses the same approach as CommitsForAuthor but for a specific repo.
func (g *GitHubDiscoverer) EmailsFromRepo(ctx context.Context, owner, repo string, maxCommits int) ([]string, error) {
	owner = strings.TrimSpace(owner)
	repo = strings.TrimSpace(repo)
	if owner == "" || repo == "" {
		return nil, nil
	}

	cacheKey := owner + "/" + repo
	if cached, ok := g.Cache.Get(cacheKey); ok {
		return cached, nil
	}

	if maxCommits <= 0 || maxCommits > 100 {
		maxCommits = 30
	}

	commitsURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/commits?per_page=%d",
		url.PathEscape(owner), url.PathEscape(repo), maxCommits)
	commits, err := g.fetchJSON(ctx, commitsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("github_repo_commits: %w", err)
	}

	type commitItem struct {
		Author *struct {
			Email string `json:"email"`
			Login string `json:"login"`
		} `json:"author"`
		Commit *struct {
			Author *struct {
				Email string `json:"email"`
			} `json:"author"`
		} `json:"commit"`
	}
	var commitsList []commitItem
	if err := json.Unmarshal(commits, &commitsList); err != nil {
		return nil, fmt.Errorf("github_repo_commits_decode: %w", err)
	}

	seen := make(map[string]bool)
	out := make([]string, 0)
	for _, c := range commitsList {
		var email string
		if c.Commit != nil && c.Commit.Author != nil {
			email = c.Commit.Author.Email
		}
		if email == "" && c.Author != nil {
			email = c.Author.Email
		}
		email = strings.TrimSpace(email)
		if email == "" || isGitHubNoreply(email) || seen[email] {
			continue
		}
		seen[email] = true
		out = append(out, email)
	}

	g.Cache.Put(cacheKey, out)
	return out, nil
}

// DiscoverFromUser discovers organizations and repos from a GitHub login,
// then extracts contributor emails from recent commits. minCommits controls
// how many recent commits to scan per repo (default 30, max 100).
func (g *GitHubDiscoverer) DiscoverFromUser(ctx context.Context, login string, minCommits int) (map[string][]string, error) {
	login = strings.TrimSpace(login)
	if login == "" {
		return nil, nil
	}
	util.Info("github_discover", map[string]any{"user": login})

	// 1. Get orgs the user belongs to.
	orgs, err := g.OrgsForUser(ctx, login)
	if err != nil {
		return nil, fmt.Errorf("discover_orgs: %w", err)
	}
	util.Info("github_orgs", map[string]any{"user": login, "orgs": len(orgs)})

	result := make(map[string][]string)

	// 2. For each org, get repos and extract emails from each repo.
	for _, org := range orgs {
		repos, err := g.ReposForOrg(ctx, org)
		if err != nil {
			util.Debug("github_org_repos_err", map[string]any{"org": org, "err": err.Error()})
			continue
		}
		for _, fullName := range repos {
			parts := strings.SplitN(fullName, "/", 2)
			if len(parts) != 2 {
				continue
			}
			emails, err := g.EmailsFromRepo(ctx, parts[0], parts[1], minCommits)
			if err != nil {
				util.Debug("github_repo_emails_err", map[string]any{"repo": fullName, "err": err.Error()})
				continue
			}
			if len(emails) > 0 {
				result[fullName] = emails
			}
		}
	}

	util.Info("github_discover_done", map[string]any{"user": login, "repos_with_emails": len(result)})
	return result, nil
}

func isGitHubNoreply(email string) bool {
	e := strings.ToLower(email)
	if strings.HasSuffix(e, "@users.noreply.github.com") {
		return true
	}
	if strings.HasSuffix(e, "@noreply.github.com") {
		return true
	}
	// Bot suffixes: -bot, [bot], -ci, -automate
	local := strings.SplitN(e, "@", 2)[0]
	if strings.HasSuffix(local, "-bot") || strings.HasSuffix(local, "[bot]") ||
		strings.HasSuffix(local, "-ci") || strings.HasSuffix(local, "-automate") {
		return true
	}
	return false
}

// ghLRU is a tiny in-memory LRU cache for (login -> []string) results.
// Uses container/list for O(1) move-to-front and eviction.
type ghLRU struct {
	mu    sync.Mutex
	data  map[string]*list.Element
	order *list.List
	cap   int
}

type ghEntry struct {
	key   string
	value []string
}

func newGHLRU(capacity int) *ghLRU {
	if capacity < 16 {
		capacity = 16
	}
	return &ghLRU{
		data:  make(map[string]*list.Element, capacity),
		order: list.New(),
		cap:   capacity,
	}
}

func (c *ghLRU) Get(key string) ([]string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	ele, ok := c.data[key]
	if !ok {
		return nil, false
	}
	c.order.MoveToFront(ele)
	return ele.Value.(*ghEntry).value, true
}

func (c *ghLRU) Put(key string, value []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ele, ok := c.data[key]; ok {
		ele.Value.(*ghEntry).value = value
		c.order.MoveToFront(ele)
		return
	}
	if c.order.Len() >= c.cap {
		old := c.order.Back()
		if old != nil {
			c.order.Remove(old)
			delete(c.data, old.Value.(*ghEntry).key)
		}
	}
	ele := c.order.PushFront(&ghEntry{key: key, value: value})
	c.data[key] = ele
}
