<div align="center">
  <h1>🕷️ scrappy</h1>
  <p><strong>Bulk job-board scraper for 100+ sites</strong></p>

  <p>
    <a href="https://github.com/arinbalyan/scrappy/actions/workflows/ci.yml">
      <img src="https://github.com/arinbalyan/scrappy/actions/workflows/ci.yml/badge.svg" alt="CI">
    </a>
    <a href="https://github.com/arinbalyan/scrappy/actions/workflows/docs.yml">
      <img src="https://github.com/arinbalyan/scrappy/actions/workflows/docs.yml/badge.svg" alt="Docs">
    </a>
    <a href="https://go.dev/">
      <img src="https://img.shields.io/github/go-mod/go-version/arinbalyan/scrappy" alt="Go Version">
    </a>
    <a href="LICENSE">
      <img src="https://img.shields.io/badge/license-non--commercial-blue" alt="License">
    </a>
    <a href="https://github.com/arinbalyan/scrappy/stargazers">
      <img src="https://img.shields.io/github/stars/arinbalyan/scrappy?style=flat" alt="Stars">
    </a>
    <a href="https://github.com/arinbalyan/scrappy/forks">
      <img src="https://img.shields.io/github/forks/arinbalyan/scrappy" alt="Forks">
    </a>
    <img src="https://api.visitorbadge.io/api/visitors?path=https%3A%2F%2Fgithub.com%2Farinbalyan%2Fscrappy&countColor=%23263759" alt="Visitors">
  </p>
</div>

<br>

## Features

- **100+ job boards / ATS endpoints** -- LinkedIn, Indeed, Google Jobs, ATS suites, and niche boards
- **Bulk-first** -- fan out across all sites concurrently, process thousands of postings
- **Go-native** -- static binary (~10 MB), zero Python dependency
- **Email enrichment** -- MX-validated contact addresses from descriptions and company pages
- **Quality scoring** -- deterministic 0-100 score per posting without an LLM
- **Multiple exports** -- JSONL, CSV, XLSX, Parquet
- **Proxy support** -- SOCKS5/HTTP with TCP-dial health checks and round-robin
- **Memory-aware** -- configurable memory cap with automatic concurrency scaling
- **Browser fallback** -- optional Playwright-based rendering for anti-bot sites (monster)

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

Per-site notes live in [docs/012-Scraping.md](docs/012-Scraping.md) and detailed pages in [docs/sites/](docs/sites/).

| Site | Site | Site | Site |
|------|------|------|------|
| `4dayweek` | `ats-rippling` | `freelancercom` | `nofluffjobs` |
| `academiccareers` | `ats-smartrecruiters` | `functionalworks` | `opensourcedesignjobs` |
| `adzuna` | `ats-successfactors` | `germantechjobs` | `powertofly` |
| `aijobs` | `ats-talentlyft` | `getonboard` | `pyjobs` |
| `androidjobs` | `ats-taleo` | `golangjobs` | `pythonjobs` |
| `arbeitnow` | `ats-teamtailor` | `google` | `railsjobs` |
| `arbeitsagentur` | `ats-trakstar` | `greenhouse` | `realworkfromanywhere` |
| `ats-adp` | `ats-ukg` | `greenjobsboard` | `reed` |
| `ats-ashby` | `ats-workable` | `guardianjobs` | `remotefirstjobs` |
| `ats-avature` | `ats-workday` | `gunio` | `remoteok` |
| `ats-bamboohr` | `authenticjobs` | `hackernews` | `remotive` |
| `ats-breezyhr` | `bayt` | `hasjob` | `simplyhired` |
| `ats-bullhorn` | `berlinstartupjobs` | `headhunter` | `snagajob` |
| `ats-comeet` | `builtin` | `higheredjobs` | `startupjobs` |
| `ats-crelate` | `canadajobbank` | `himalayas` | `stepstone` |
| `ats-deel` | `careerbuilder` | `hiringcafe` | `swissdevjobs` |
| `ats-fountain` | `careerjet` | `huggingfacejobs` | `talroo` |
| `ats-freshteam` | `careeronestop` | `icrunchdata` | `techcareers` |
| `ats-gem` | `clojurejobs` | `indeed` | `tesla` |
| `ats-hiringthing` | `conservationjobs` | `infojobs` | `themuse` |
| `ats-icims` | `coroflot` | `internshala` | `ukvisajobs` |
| `ats-ismartrecruit` | `crunchboard` | `ismartrecruit` | `undpjobs` |
| `ats-jazzhr` | `cryptocurrencyjobs` | `jazzhr` | `upwork` |
| `ats-jobscore` | `cryptojobslist` | `jobdataapi` | `usajobs` |
| `ats-jobvite` | `devitjobs` | `jobicy` | `virtualvocations` |
| `ats-jobylon` | `devopsjobs` | `jobindex` | `vuejobs` |
| `ats-joincom` | `dice` | `jobsacuk` | `web3career` |
| `ats-loxo` | `djinni` | `jobsch` | `wellfound` |
| `ats-manatal` | `dribbble` | `jobsdb` | `weworkremotely` |
| `ats-mercor` | `drupaljobs` | `jobsinjapan` | `wordpressjobs` |
| `ats-oracle` | `duunitori` | `jobspresso` | `workingnomads` |
| `ats-personio` | `ecojobs` | `jobstreet` | `wuzzuf` |
| `ats-phenom` | `echojobs` | `jobtechdev` | `ycjobs` |
| `ats-pinpoint` | `elixirjobs` | `joinrise` | `ziprecruiter` |
| `ats-recruitee` | `eurojobs` | `landingjobs` |  |
| `ats-recruiterflow` | `exa` | `linkedin` |  |
| `ats-recruitify` | `findwork` | `monster` |  |

## Installation

See [docs/003-Installation.md](docs/003-Installation.md) for Go, Docker, and CI installation.

## Build with Makefile

The project includes a `Makefile` for common development tasks:

```bash
make build      # Build the scrappy binary to bin/scrappy
make test       # Run all unit tests
make test-race  # Run tests with race detector
make vet        # Run go vet
make lint       # Run golangci-lint
make clean      # Remove build artifacts
make docker     # Build Docker image
make all        # build + test + vet
```

A `.dockerignore` excludes unnecessary files from Docker builds for smaller images.

## Go library usage

```go
import (
    "github.com/arinbalyan/scrappy/pkg/scrappy"
)

engine := scrappy.NewEngine()
jobs, err := engine.Scrape(ctx, scrappy.ScraperInput{
    Sites:        []string{"linkedin", "indeed"},
    SearchTerm:   "software engineer",
    Location:     "San Francisco, CA",
    ResultsWanted: 500,
})
```

## Contributing

See [docs/019-Architecture-Reference.md](docs/019-Architecture-Reference.md) for the package layout and [docs/012-Scraping.md](docs/012-Scraping.md) for scraping details.

<br>

<!-- Star History -->
<div align="center">
  <a href="https://star-history.com/#arinbalyan/scrappy&Timeline">
    <picture>
      <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/svg?repos=arinbalyan/scrappy&type=Timeline&theme=dark" />
      <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/svg?repos=arinbalyan/scrappy&type=Timeline" />
      <img alt="Star History Chart" src="https://api.star-history.com/svg?repos=arinbalyan/scrappy&type=Timeline" width="600" />
    </picture>
  </a>
</div>

<br>

## License

This project is licensed under the terms specified in the [LICENSE](LICENSE) file. Personal, non-commercial use only.
