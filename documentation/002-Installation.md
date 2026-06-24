# Installation

## Pre-built binaries

Download the latest release for your platform:

```bash
# Linux (x86_64)
curl -fsSL https://github.com/arinbalyan/scrappy/releases/latest/download/scrappy_linux_amd64.tar.gz | tar xz
sudo mv scrappy_linux_amd64 /usr/local/bin/scrappy

# macOS (Apple Silicon)
curl -fsSL https://github.com/arinbalyan/scrappy/releases/latest/download/scrappy_darwin_arm64.tar.gz | tar xz
sudo mv scrappy_darwin_arm64 /usr/local/bin/scrappy

# macOS (Intel)
curl -fsSL https://github.com/arinbalyan/scrappy/releases/latest/download/scrappy_darwin_amd64.tar.gz | tar xz
sudo mv scrappy_darwin_amd64 /usr/local/bin/scrappy

# Windows (PowerShell)
curl.exe -LO https://github.com/arinbalyan/scrappy/releases/latest/download/scrappy_windows_amd64.zip
Expand-Archive scrappy_windows_amd64.zip -DestinationPath .
.\scrappy_windows_amd64.exe --help
```

## Go install

```bash
go install github.com/arinbalyan/scrappy/cmd/scrappy@latest
```

## Build from source

```bash
git clone https://github.com/arinbalyan/scrappy.git
cd scrappy
go build -o scrappy ./cmd/scrappy
```

## Post-install

1. Copy `.env.example` to `.env` and configure API keys
2. Run `scrappy doctor` to verify your setup
3. Run `scrappy` without arguments for the interactive wizard

### Playwright (optional, for browser-based scrapers)

Some sites (LinkedIn, Monster, Google) use a headless browser fallback:

```bash
cd scripts
npm install
```

This requires Node.js and Playwright-compatible Chromium installed.
