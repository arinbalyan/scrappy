<div align="center">
  <h1>scrappy</h1>
  <p><strong>Bulk job-board scraper</strong></p>

  <p>
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
| — | `scrappy --help` | Complete CLI reference with flag descriptions |
| — | `.env.example` | All supported environment variables |

Use `scrappy doctor` to diagnose your setup.

## Sites supported

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
| `ats-manatal` | `jobicy` | `jobsdb` | `weworkremotely` |
| `ats-mercor` | `drupaljobs` | `jobsinjapan` | `wordpressjobs` |
| `ats-oracle` | `duunitori` | `jobspresso` | `workingnomads` |
| `ats-personio` | `ecojobs` | `jobstreet` | `wuzzuf` |
| `ats-phenom` | `echojobs` | `jobtechdev` | `ycjobs` |
| `ats-pinpoint` | `elixirjobs` | `joinrise` | `ziprecruiter` |
| `ats-recruitee` | `eurojobs` | `landingjobs` |  |
| `ats-recruiterflow` | `exa` | `linkedin` |  |
| `ats-recruitify` | `findwork` | `monster` |  |

## Installation

Install via one-liner above, `go install`, or build from source with `go build ./cmd/scrappy`.

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

## Go library usage
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
