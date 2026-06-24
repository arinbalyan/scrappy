# scrappy

**Bulk job-board scraper for 100+ sites — Go-native, high concurrency, bulk-first.**

## Features

- **100+ job boards / ATS endpoints** — LinkedIn, Indeed, Google Jobs, ATS suites, and niche boards
- **Bulk-first** — fan out across all sites concurrently, process thousands of postings
- **Email enrichment** — MX-validated contact addresses from descriptions and company pages
- **Quality scoring** — deterministic 0-100 score per posting without an LLM
- **Multiple exports** — JSONL, CSV, XLSX, Parquet
- **Proxy support** — SOCKS5/HTTP with TCP-dial health checks and round-robin
- **Memory-aware** — configurable memory cap with automatic concurrency scaling
- **Browser fallback** — optional Playwright-based rendering for anti-bot sites

## Quick links

- [Quickstart](001-Quickstart.md)
- [Installation](002-Installation.md)
- [CLI Reference](003-CLI-Reference.md)
- [Configuration](004-Configuration.md)
- [Scraper Status](005-Status.md)
