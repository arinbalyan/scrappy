# GitHub Email Discovery

The `GitHubDiscoverer` extracts email addresses from commit author data on public GitHub repositories. This is separate from job scraping — it's invoked via the `--github-scrape` CLI flag.

---

## Usage

```bash
# Basic discovery from a user's repos
scrappy --github-scrape --search "torvalds"

# With GitHub token (higher rate limit)
GITHUB_TOKEN=ghp_xxx scrappy --github-scrape --search "torvalds"

# Output to file (default: github_emails.csv)
scrappy --github-scrape --search "linux-foundation" --out emails.csv
```

The `--search` value is treated as a GitHub username or organization name.

---

## Discovery flow

The `DiscoverFromUser` method runs a three-phase pipeline:

```
OrgsForUser(login)
  → ReposForOrg(org) per org
    → EmailsFromRepo(owner, repo) per repo
```

### Phase 1: `OrgsForUser(ctx, login)`

```go
url := fmt.Sprintf("https://api.github.com/users/%s/orgs", login)
```

Returns the list of organization logins the user belongs to. Uses the public GitHub API — requires the org to be public.

### Phase 2: `ReposForOrg(ctx, org)`

```go
url := fmt.Sprintf("https://api.github.com/orgs/%s/repos?per_page=50&sort=pushed&type=public", org)
```

Returns non-fork, public repository full names (`owner/repo`), sorted by most recently pushed.

### Phase 3: `EmailsFromRepo(ctx, owner, repo, maxCommits)`

```go
url := fmt.Sprintf("https://api.github.com/repos/%s/%s/commits?per_page=%d", owner, repo, maxCommits)
```

Extracts unique personal emails from recent commits. Key behavior:

- Reads both `commit.author.email` (the commit metadata) and `author.email` (the GitHub user object).
- Filters out GitHub's noreply addresses (`@users.noreply.github.com`, `@noreply.github.com`).
- Filters out bot addresses (`-bot`, `[bot]`, `-ci`, `-automate` suffixes).
- Default: 30 commits per repo, max 100.
- Falls back to `EmailsFromRepo` for direct org/repo lookups if `DiscoverFromUser` returns nothing.

---

## API endpoints used

| Endpoint | Purpose | Auth |
|----------|---------|------|
| `GET /users/{login}/orgs` | List user's organizations | Optional |
| `GET /orgs/{org}/public_members` | List org members | Optional |
| `GET /orgs/{org}/repos` | List org repos | Optional |
| `GET /users/{login}/repos` | List user's repos | Optional |
| `GET /repos/{owner}/{repo}/commits` | Commit history | Optional |
| `GET /repos/{owner}/{repo}/commits?author={login}` | Author-filtered commits | Optional |

Headers set on all requests:
```
Accept: application/vnd.github+json
X-GitHub-Api-Version: 2022-11-28
Authorization: Bearer <token>   // only when GITHUB_TOKEN is set
```

---

## Rate limiting

| Auth | Rate limit | Per hour |
|------|-----------|----------|
| Unauthenticated | 60 requests/hour | 60 |
| With token | 5,000 requests/hour | 5,000 |

Unauthenticated requests are identified by the lack of `Authorization` header and are subject to IP-based rate limiting (60 req/h). With a `GITHUB_TOKEN`, the limit is 5,000 req/h per token.

The discoverer detects rate limiting by checking for HTTP 403:

```go
if resp.StatusCode == http.StatusForbidden {
    return nil, fmt.Errorf("github_rate_limited (status %d)", resp.StatusCode)
}
```

Error messages include the status code for diagnostics.

---

## Caching

Results are cached in an in-memory LRU cache (capacity: 256 entries):

```go
type ghLRU struct {
    mu    sync.Mutex
    data  map[string]*list.Element
    order *list.List
    cap   int
}
```

- Cache key: GitHub login or `owner/repo`.
- Eviction: least-recently-used.
- Thread-safe via `sync.Mutex`.
- Empty results (no emails found) are also cached to avoid re-scanning.

---

## CLI integration

The `--github-scrape` flag diverts execution to `runGitHubScrape()` in `cmd/scrappy/main.go`:

```go
func runGitHubScrape(cfg *cliConfig) error {
    g := internalemail.NewGitHubDiscoverer(token)
    result, err := g.DiscoverFromUser(ctx, login, 30)
    // ...
}
```

Output is always CSV with two columns: `repo,email`. If the search term is an organization (not a user), the discoverer falls back to treating it as an org and scanning its repos directly:

```go
if len(result) == 0 {
    repos, repoErr := g.ReposForOrg(ctx, login)
    // ...
}
```

---

## Privacy considerations

GitHub's "Keep my email address private" setting only applies to the **author email** shown on the UI and in the commit web view. The **commit author metadata** (`commit.author.email` in the API) still contains the real email for historical commits. scrappy reads the API-level commit metadata, not the web UI, so it can extract real addresses even when privacy mode is enabled.

This is standard behavior: CI tools, package registries, and code review platforms all use the same API data.
