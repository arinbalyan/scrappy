# scrappy

Bulk job-board scraper for 55+ sites, written in Go.

## Features

- **55+ job boards** -- LinkedIn, Indeed, Glassdoor, Google Jobs, and more
- **Bulk-first** -- fan out across all sites concurrently, process thousands of postings
- **Go-native** -- static binary (~10 MB), zero Python dependency
- **Email enrichment** -- MX-validated contact addresses from descriptions and company pages
- **Quality scoring** -- deterministic 0-100 score per posting without an LLM
- **Multiple exports** -- JSONL, CSV, XLSX, Parquet
- **Proxy support** -- SOCKS5/HTTP with TCP-dial health checks and round-robin
- **Memory-aware** -- configurable memory cap with automatic concurrency scaling

## Quick start

### One-line install

```bash
# Linux (x86_64)
curl -fsSL https://github.com/arinbalyan/scrappy/releases/latest/download/scrappy_linux_amd64.tar.gz | tar xz && sudo mv scrappy_linux_amd64 /usr/local/bin/scrappy

# macOS (Apple Silicon)
curl -fsSL https://github.com/arinbalyan/scrappy/releases/latest/download/scrappy_darwin_arm64.tar.gz | tar xz && sudo mv scrappy_darwin_arm64 /usr/local/bin/scrappy

# macOS (Intel)
curl -fsSL https://github.com/arinbalyan/scrappy/releases/latest/download/scrappy_darwin_amd64.tar.gz | tar xz && sudo mv scrappy_darwin_amd64 /usr/local/bin/scrappy
```

```powershell
# Windows (PowerShell)
curl.exe -LO https://github.com/arinbalyan/scrappy/releases/latest/download/scrappy_windows_amd64.zip
Expand-Archive scrappy_windows_amd64.zip -DestinationPath .
.\scrappy_windows_amd64.exe --help
```

### Or install with Go

```
go install github.com/arinbalyan/scrappy/cmd/scrappy@latest
```

### First scrape

```bash
scrappy --sites remoteok --search "golang" --results-wanted 50
```

For the interactive wizard, run without arguments:
```bash
scrappy
```

## Documentation

| # | Document | Description |
|---|----------|-------------|
| 001 | [Quickstart](docs/001-Quickstart.md) | Get running in 5 minutes |
| 002 | [Architecture Overview](docs/002-Architecture-Overview.md) | High-level design and data flow |
| 003 | [Installation](docs/003-Installation.md) | Install from source, Docker, or CI |
| 004 | [CLI Reference](docs/004-CLI-Reference.md) | Complete flag reference with examples |
| 005 | [Interactive Mode](docs/005-Interactive-Mode.md) | Wizard-based configuration |
| 006 | [Non-Interactive Mode](docs/006-NonInteractive-Mode.md) | Script and cron usage |
| 007 | [Multi-Value Search](docs/007-Multi-Value.md) | Cartesian product of terms and locations |
| 008 | [Configuration](docs/008-Configuration.md) | YAML config file reference |
| 009 | [Export Formats](docs/009-Export-Formats.md) | Output format details and column schema |
| 010 | [Email](docs/010-Email.md) | Extraction, MX validation, enrichment |
| 011 | [Quality](docs/011-Quality.md) | Deterministic scoring formula |
| 012 | [Scraping](docs/012-Scraping.md) | Per-site notes and rate limits |
| 013 | [Proxy](docs/013-Proxy.md) | Proxy setup for local and CI |
| 014 | [Dedup](docs/014-Dedup.md) | Cross-site URL and company deduplication |
| 015 | [Environment Variables](docs/015-Environment-Variables.md) | All supported environment variables |
| 016 | [Memory Management](docs/016-Memory-Management.md) | Memory cap and concurrency scaling |
| 017 | [Troubleshooting](docs/017-Troubleshooting.md) | Common issues and solutions |
| 018 | [FAQ](docs/018-FAQ.md) | Frequently asked questions |
| 019 | [Architecture Reference](docs/019-Architecture-Reference.md) | Detailed package architecture |

## Sites supported

All 55+ sites with per-board notes in [docs/012-Scraping.md](docs/012-Scraping.md) and per-site documentation in [docs/sites/](docs/sites/).

## Installation

See [docs/003-Installation.md](docs/003-Installation.md) for Go, Docker, and CI installation.

## Go library usage

```go
import (
    "github.com/arinbalyan/scrappy/internal/model"
    "github.com/arinbalyan/scrappy/pkg/scrappy"
)

engine := scrappy.NewEngine()
jobs, err := engine.Scrape(ctx, model.ScraperInput{
    Sites:        []model.Site{model.SiteLinkedIn, model.SiteIndeed},
    SearchTerm:   "software engineer",
    Location:     "San Francisco, CA",
    ResultsWanted: 500,
})
```

## Contributing

See [docs/019-Architecture-Reference.md](docs/019-Architecture-Reference.md) for the package layout and [docs/012-Scraping.md](docs/012-Scraping.md) for scraping details.

## License

MIT