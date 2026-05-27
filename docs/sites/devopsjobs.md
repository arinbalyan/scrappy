# DevOpsJobs

## Current integration
- Parser order:
  1. RSS feed at `https://devopsjobs.io/jobs.rss`
- Extracted fields: title, link, guid, description, pubDate

## Supported knobs
- `results_wanted`
- `search_term` — client-side filter on title + description
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)

## Constraints and breakpoints
- Single RSS feed fetch — no pagination.
- Uses streaming XML decoder (`xml.NewDecoder`).
- Title format is parsed as `Job Title - Company - City` using ` - ` separator.
- Description is HTML — stripped to plain text.
- Search term is client-side (case-insensitive substring match).

## Debug/update playbook
1. Verify the RSS feed URL at `devopsjobs.io/jobs.rss`.
2. Validate `<item>` structure (title, link, guid, description, pubDate).
3. Check title format separator pattern (` - `).
4. Re-run `go test ./tests/scraper/devopsjobs` and full suite.
