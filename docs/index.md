# scrappy documentation

## Bulk job-board scraper for 100+ sites

`scrappy` is a Go-based job scraping toolkit that supports **100+ job boards and ATS endpoints** — LinkedIn, Indeed, Google Jobs, ATS suites (Workday, Taleo, BambooHR, etc.), and niche boards.

### Quick start

```bash
scrappy --sites remoteok --search "golang" --results-wanted 50
```

For the interactive wizard, run without arguments:
```bash
scrappy
```

### One-line install

```bash
# Linux (x86_64)
curl -fsSL https://github.com/arinbalyan/scrappy/releases/latest/download/scrappy_linux_amd64.tar.gz | tar xz && sudo mv scrappy_linux_amd64 /usr/local/bin/scrappy

# macOS (Apple Silicon)
curl -fsSL https://github.com/arinbalyan/scrappy/releases/latest/download/scrappy_darwin_arm64.tar.gz | tar xz && sudo mv scrappy_darwin_arm64 /usr/local/bin/scrappy
```

### Features

- **100+ job boards / ATS endpoints** — LinkedIn, Indeed, Google Jobs, ATS suites, and niche boards
- **Bulk-first** — fan out across all sites concurrently, process thousands of postings
- **Go-native** — static binary (~10 MB), zero Python dependency
- **Email enrichment** — MX-validated contact addresses from descriptions and company pages
- **Quality scoring** — deterministic 0-100 score per posting without an LLM
- **Multiple exports** — JSONL, CSV, XLSX, Parquet
- **Proxy support** — SOCKS5/HTTP with TCP-dial health checks and round-robin
- **Memory-aware** — configurable memory cap with automatic concurrency scaling
- **Browser fallback** — optional Playwright-based rendering for anti-bot sites

### License

Personal, non-commercial use only. See [LICENSE](https://github.com/arinbalyan/scrappy/blob/main/LICENSE).
