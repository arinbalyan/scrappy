# Quickstart

## One-line install

```bash
# Linux (x86_64)
curl -fsSL https://github.com/arinbalyan/scrappy/releases/latest/download/scrappy_linux_amd64.tar.gz | tar xz && sudo mv scrappy_linux_amd64 /usr/local/bin/scrappy

# macOS (Apple Silicon)
curl -fsSL https://github.com/arinbalyan/scrappy/releases/latest/download/scrappy_darwin_arm64.tar.gz | tar xz && sudo mv scrappy_darwin_arm64 /usr/local/bin/scrappy
```

## Or install with Go

```bash
go install github.com/arinbalyan/scrappy/cmd/scrappy@latest
```

## First scrape

```bash
scrappy --sites remoteok --search "golang" --results-wanted 50
```

## Interactive wizard

Run without arguments for the interactive setup:

```bash
scrappy
```

## Configure API keys

Copy `.env.example` to `.env` and fill in your keys:

```bash
cp .env.example .env
```

Required for: Adzuna, Careerjet, Indeed, InfoJobs, Findwork, Arbeitsagentur, Web3Career, and others.

## Building from source

```bash
git clone https://github.com/arinbalyan/scrappy.git
cd scrappy
go build -o scrappy ./cmd/scrappy
```
