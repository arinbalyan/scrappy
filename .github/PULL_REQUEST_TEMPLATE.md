---
name: Pull Request
about: Submit a change to scrappy
title: "feat: "
labels: needs-review
assignees: ''
---

## Description

Please include a summary of the change and which issue is fixed. List any
dependencies that are required for this change.

Fixes # (issue)

## Type of Change

- [ ] Bug fix (non-breaking change which fixes an issue)
- [ ] New board (new scraper for a job board)
- [ ] New feature (non-breaking change which adds functionality)
- [ ] Performance improvement
- [ ] Documentation update
- [ ] Refactoring (no functional changes)
- [ ] CI / tooling change

## How Has This Been Tested?

- [ ] `go build ./...` succeeds
- [ ] `go test ./...` passes (or `go test -short` for large runs)
- [ ] `go vet ./...` passes
- [ ] `go test -race ./...` passes (if concurrency changes made)
- [ ] Manual test with a real board (if scraper change)

## Checklist

- [ ] My code follows the project's coding conventions (see CONTRIBUTING.md)
- [ ] I have added tests that prove my fix is effective or that my feature works
- [ ] New scraper: I added a contract test in `tests/scraper/<name>/contract_test.go`
- [ ] New scraper: I added documentation in `docs/sites/<name>.md`
- [ ] New scraper: I registered the site in `internal/model/types.go` and `pkg/scrappy/engine.go`
- [ ] I have updated the documentation (if applicable)
- [ ] My changes generate no new warnings or errors

## Additional Context

Add any other context about the pull request here (screenshots, benchmark
results, architecture decisions, etc.).
