# hackernews

## Current integration
- Parser order:
  1. HTML scraping: https://hacker-news.firebaseio.com/v0/ (Who is Hiring)
- Extracted fields: title (post body), company, remote

## Supported knobs
- `results_wanted`
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)
- `search_term`, `location`
- 

## Constraints and breakpoints
- Fetches 'Who is Hiring?' thread from Hacker News Firebase API. Extracts company names via regex. Processes items from the monthly thread.

## Debug/update playbook
1. Fetch https://hacker-news.firebaseio.com/v0/ (Who is Hiring) with browser-like headers and examine HTML structure.
2. Update regex or selector patterns if site markup changes.
3. Verify anti-bot challenge detection is working if blocked.
4. Re-run `go test ./tests/scraper/hackernews` and full suite.
