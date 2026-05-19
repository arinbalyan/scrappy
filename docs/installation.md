# Installation

## Go install (library)

```bash
go get github.com/arinbalyan/scrappy
```

```go
import "github.com/arinbalyan/scrappy/internal/scraper"
```

## Go install (CLI)

```bash
go install github.com/arinbalyan/scrappy/cmd/scrappy@latest
scrappy --help
```

Build from source:

```bash
git clone git@github.com:arinbalyan/scrappy.git
cd scrappy
go mod tidy
go build -o /usr/local/bin/scrappy ./cmd/scrappy
```

## Docker

```bash
docker build -t scrappy .
docker run scrappy scrape --sites indeed,remoteok --search "software engineer" --results-wanted 200
```

## Development setup

```bash
git clone git@github.com:arinbalyan/scrappy.git
cd scrappy
go mod tidy
go test ./...
```

## CI / GitHub Actions

```yaml
- uses: actions/setup-go@v5
  with: { go-version: 1.24 }
- run: go mod tidy
- run: go build ./...
- run: go test -race -cover ./...
- run: go vet ./...
```
