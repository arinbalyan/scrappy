# Multi-Value Search (Cartesian Product)

## The problem

A single scrape run is limited to one search term and one location:

```bash
scrappy --sites indeed --search "AI Engineer" --location "Remote"
```

If you want AI Engineer jobs in both Remote AND New York, you would need to run the tool twice. If you also want to search for "Software Engineer" in both locations, that is four separate runs.

## The solution

scrappy accepts **comma-separated values** for `--search` and `--location`. It generates the cartesian product of all terms x all locations and iterates over every combination for each site.

```bash
scrappy --sites indeed --search "AI Engineer,Software Engineer" \
        --location "Remote,New York" --results-wanted 500
```

This internally generates 4 passes per site:

| Pass | Search Term | Location |
|------|-------------|----------|
| 1 | AI Engineer | Remote |
| 2 | AI Engineer | New York |
| 3 | Software Engineer | Remote |
| 4 | Software Engineer | New York |

Results from all passes are pooled, deduplicated (by job URL), and returned as a single set.

## How it works internally

1. CLI parses `--search "AI Engineer,Software Dev"` into `["AI Engineer", "Software Dev"]`.
2. CLI parses `--location "Remote,New York"` into `["Remote", "New York"]`.
3. The engine iterates over `SearchTerms x Locations` for each site.
4. For each `(term, location)` pair, the site scraper runs a full pagination loop.
5. Results are appended to a shared pool, deduplicated by `job_url`.
6. The pool is trimmed to `results-wanted` total.

The relevant struct fields in `ScraperInput`:

```go
type ScraperInput struct {
    SearchTerm  string   // single fallback (first of SearchTerms)
    Location    string   // single fallback (first of Locations)
    SearchTerms []string // multi-value -- cartesian product source
    Locations   []string // multi-value -- cartesian product source
    // ...
}
```

## Config YAML equivalent

In `config.yaml`, use list form for `search` and `location`:

```yaml
defaults:
  search:
    - AI Engineer
    - Software Engineer
  location:
    - Remote
    - New York
```

This produces the same 4-pass cartesian product as the CLI example above.

You can mix CLI and config: CLI flags override config entirely. If you set `--search` on the CLI, the config's `defaults.search` is ignored.

## Per-site override behaviour

When a site has its own `search` or `location` in the `sites:` block, the **site-specific values replace the global ones** for that site only -- the cartesian product is still computed.

```yaml
defaults:
  search:
    - AI Engineer
    - Software Engineer
  location:
    - Remote
    - New York

sites:
  indeed:
    search:
      - '"AI Engineer" OR "ML Engineer"'
      - '"GTM Engineer"'
    location:
      - Remote
      - San Francisco
```

For `indeed`, the cartesian product is:

| Pass | Search Term | Location |
|------|-------------|----------|
| 1 | "AI Engineer" OR "ML Engineer" | Remote |
| 2 | "AI Engineer" OR "ML Engineer" | San Francisco |
| 3 | "GTM Engineer" | Remote |
| 4 | "GTM Engineer" | San Francisco |

Other sites (e.g. `linkedin`, `glassdoor`) still use `[AI Engineer, Software Engineer] x [Remote, New York]`.

## Examples

### 2 terms x 2 locations (4 passes)

```bash
scrappy --sites indeed --search "AI Engineer,Rust Developer" \
        --location "Remote,San Francisco" --results-wanted 400
```

### 3 terms x 1 location (3 passes)

```bash
scrappy --sites linkedin --search "AI Engineer,ML Engineer,LLM Engineer" \
        --location "Remote" --results-wanted 300
```

### 1 term x 3 locations (3 passes)

```bash
scrappy --sites glassdoor --search "software engineer" \
        --location "Remote,New York,Hyderabad" --results-wanted 300
```

### All 65+ sites with multiple terms

```bash
scrappy --search "AI Engineer,Rust Developer" \
        --location "Remote" --results-wanted 5000
```

### With config file multi-value defaults

```yaml
# config.yaml
defaults:
  search:
    - AI Engineer
    - GTM Engineer
    - LLM Engineer
  location:
    - Remote
    - San Francisco
  results_wanted: 1000
```

```bash
scrappy --sites linkedin,indeed,glassdoor
```

Generates 3 x 2 = 6 passes per site (18 total).

## Best practices

### Do not overdo it

Each combination is a full pagination loop against that site. A 5-term x 5-location config generates **25 passes per site**. With 10 sites, that is 250 pagination cycles.

```yaml
# Reasonable for a daily bulk run: 3 x 2 = 6 passes per site
defaults:
  search:
    - AI Engineer
    - ML Engineer
    - LLM Engineer
  location:
    - Remote
    - San Francisco
```

```yaml
# Ambitious -- expect long runtimes: 5 x 4 = 20 passes per site
defaults:
  search:
    - AI Engineer
    - ML Engineer
    - LLM Engineer
    - Data Engineer
    - Rust Developer
  location:
    - Remote
    - New York
    - San Francisco
    - London
```

### Watch rate limits

More passes = more requests per site. If a site rate-limits at N requests/second, each pass consumes N requests x pages per pass. For 20 passes on a site with 10 pages each and 3 req/s:

```
20 passes x 10 pages = 200 requests
200 requests / 3 req/s = ~67 seconds for that site
```

Use `--site-rps` to control per-site rates. See [012-Scraping.md](012-Scraping.md).

### Use site-specific overrides for targeted combos

Give each site only the terms that make sense for its audience:

```yaml
sites:
  remoteok:
    search:
      - ai engineer
      - llm engineer
      - go developer
    location:
      - Remote
  stepstone:
    search:
      - AI Engineer
    location:
      - Germany
      - Austria
```

### Combine with `--timeout`

Multi-value runs take longer. Set a generous timeout:

```bash
scrappy --sites linkedin,indeed --search "AI Engineer,Rust Developer" \
        --location "Remote,New York" --results-wanted 1000 \
        --timeout 1800
```

### Combine with `--email` for targeted sourcing

```bash
scrappy --sites remoteok,remotive,arbeitnow \
        --search "golang developer,rust developer" \
        --location "Remote" --results-wanted 500 --email
```

Only jobs with at least one email address are returned -- useful when the output feeds a lead-generation pipeline.

### Combine with quality filtering

```bash
scrappy --sites remoteok,remotive \
        --search "golang developer,rust developer" \
        --location "Remote" --results-wanted 500 \
        --min-score 60
```
