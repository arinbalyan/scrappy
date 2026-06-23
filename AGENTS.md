# Agent context for scrappy

This file gives coding agents enough context to change **scrappy** safely. It complements [`CONTRIBUTING.md`](./CONTRIBUTING.md) (human contributor guide) and the [docs site](https://arinbalyan.github.io/scrappy/).

## What this repo is

- **scrappy** — Go CLI and library that scrapes 100+ job boards / ATS endpoints in parallel.
- **Default branch:** `main` (all PRs target `main`).
- **License:** personal, non-commercial use only.

## Layout (where to edit)

| Path | Purpose |
|------|---------|
| `cmd/scrappy/` | Cobra CLI entry point |
| `pkg/scrappy/` | Public API (`engine.go`, orchestration) |
| `internal/scraper/<site>/` | One package per job board |
| `internal/model/` | Shared types (`JobPost`, `Site`, `ScraperInput`) |
| `tests/scraper/<site>/` | Contract tests (external test package) |
| `config.yaml` | Per-site search terms and locations |
| `docs/sites/` | Per-scraper maintainer notes (MkDocs) |

## Adding or fixing a scraper

1. Implement `Scraper` in `internal/scraper/<sitename>/`:

```go
type Scraper interface {
    SiteName() model.Site
    Scrape(ctx context.Context, input model.ScraperInput) ([]model.JobPost, error)
}
```

2. Register in `pkg/scrappy/engine.go` (`<sitename>scraper.New(nil)`).
3. Add `SiteFoo` constant in `internal/model/types.go`.
4. Add config stanza in `config.yaml`.
5. Add contract test under `tests/scraper/<sitename>/` using `httptest` mocks — **no live HTTP in CI**.
6. Document in `docs/sites/<sitename>.md`.

**Parsing preference:** JSON API → JSON-LD → `__NEXT_DATA__` → HTML regex.

## Commands agents should run before opening a PR

```bash
go test ./...
go vet ./...
go test -race ./...
```

Optional local smoke:

```bash
go build ./cmd/scrappy/
./scrappy --sites hackernews --search "golang" --results-wanted 5
```

## Conventions

- Pass `context.Context` as the first parameter to blocking functions.
- Wrap errors: `fmt.Errorf("context: %w", err)` — avoid panics outside `main`/`init`.
- HTTP clients: `util.NewHTTPClient(util.ClientOptions{Retries: 2, Timeout: 20 * time.Second})`.
- Branch names: `fix/<scope>/<name>`, `feat/<scope>/<name>`, etc.
- PR body should reference issues (`Closes #123`).

## CI expectations

PRs must pass build, unit tests, race detector, `go vet`, and gitleaks. Do not commit secrets or live API keys.

## Out of scope for agents

- Do not retarget frozen branches (`dev`, `beta`) — use `main` only.
- Do not hit production job-board endpoints from automated tests.
- Do not change license terms or remove non-commercial notices.
