# Installation

## Requirements

- Go 1.26+ (see `go.mod` for exact toolchain version)
- No Python or external runtime required -- scrappy is a single static binary

## Go install (CLI binary)

```bash
go install github.com/arinbalyan/scrappy/cmd/scrappy@latest
```

The binary is placed in `$GOPATH/bin` (or `$GOBIN`). Verify:

```bash
scrappy --version
# scrappy v0.1.0
scrappy --help
```

### As a Go library

```bash
go get github.com/arinbalyan/scrappy
```

```go
import "github.com/arinbalyan/scrappy/pkg/scrappy"

engine := scrappy.NewEngine()
jobs, err := engine.Scrape(ctx, input)
```

## Build from source

```bash
git clone git@github.com:arinbalyan/scrappy.git
cd scrappy
go mod tidy
go build -o /usr/local/bin/scrappy ./cmd/scrappy
```

### All-in-one build + test

```bash
go mod tidy
go build ./...
go test ./...
go vet ./...
```

## Development setup

```bash
git clone git@github.com:arinbalyan/scrappy.git
cd scrappy
go mod tidy

# Copy the environment template
cp .env.example .env
# Edit .env with your API keys

# Run all unit tests
go test ./...

# Run with race detector
go test -race ./...

# Run contract tests (hits mock servers)
go test -count=1 -timeout 120s ./tests/scraper/...

# Static analysis
go vet ./...

# Build the binary
go build -o scrappy ./cmd/scrappy
```

No code generation or build tools are needed beyond the Go toolchain.

## Docker

A `Dockerfile` is provided for containerized builds. The image uses a multi-stage build for a small static binary (~10 MB).

```bash
# Build the image
docker build -t scrappy .

# Run with default entrypoint
docker run scrappy --sites remoteok --search "rust" \
  --location "Remote" --results-wanted 200

# Write output to a mounted volume
docker run -v $PWD/data:/out scrappy \
  --sites indeed,glassdoor --search "golang" \
  --results-wanted 100 --format csv --out /out/jobs.csv

# Use proxy
docker run -e SCRAPPY_PROXIES=socks5://host:7890 scrappy \
  --sites linkedin --search "engineer" --results-wanted 50
```

### Docker Compose for scheduled runs

A `docker-compose.yml` is provided for scheduled bulk scraping:

```yaml
services:
  scrappy:
    build: .
    volumes: ["./data:/out"]
    command: >
      --sites linkedin,indeed,glassdoor,remoteok
      --search "software engineer" --location "Remote"
      --results-wanted 1000 --format jsonl --out /out/jobs.jsonl
    environment:
      - SCRAPPY_PROXIES=socks5://proxy:7890
```

## CI / GitHub Actions

```yaml
name: CI
on: [push, pull_request]
jobs:
  build-test-vet:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v5
      - uses: actions/setup-go@v6
        with:
          go-version: '1.26.x'
          cache: true
      - run: go mod tidy && git diff --exit-code go.mod go.sum
      - run: go build ./...
      - run: go test ./...
      - run: go test -race ./...
      - run: go vet ./...
```

The CI workflow also runs a gitleaks secrets scan on every push.

## Dependencies

scrappy keeps its dependency tree minimal:

| Dependency | Purpose |
|------------|---------|
| `github.com/spf13/cobra` | CLI framework (flags, commands) |
| `gopkg.in/yaml.v3` | Parse config.yaml |
| `golang.org/x/time/rate` | Per-site token-bucket rate limiter |
| `github.com/xuri/excelize/v2` | XLSX export |
| `github.com/xitongsys/parquet-go` | Parquet export |
| `github.com/stretchr/testify` | Test assertions (dev only) |

All HTTP, JSON, CSV, and concurrency primitives use Go standard library -- nothing beyond these direct dependencies.

## Environment variables

See [015-Environment-Variables.md](015-Environment-Variables.md) for the complete reference. Key variables:

| Variable | Purpose |
|----------|---------|
| `ADZUNA_APP_ID`, `ADZUNA_APP_KEY` | Adzuna API credentials |
| `CAREERJET_AFFID` | CareerJet partner ID |
| `INFOJOBS_CLIENT_ID`, `INFOJOBS_CLIENT_SECRET` | InfoJobs API credentials |
| `FINDWORK_API_KEY` | Findwork API key |
| `ARBEITSAGENTUR_API_KEY` | Arbeitsagentur API key |
| `SCRAPPY_PROXIES` | Comma-separated SOCKS5 proxy URLs |
| `SCRAPPY_LOG_LEVEL` | Default log level (DEBUG|INFO|WARN|ERROR) |

Variables can be set in the environment, in a `.env` file beside `config.yaml`, or in `~/.scrappy/.env`.
