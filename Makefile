.PHONY: all build test vet race lint clean install dev-deps help

BINARY_NAME   := scrappy
CMD_DIR       := ./cmd/scrappy
OUT_DIR       := dist
GO_FLAGS      := -ldflags="-s -w -X main.version=$(VERSION)"
GIT_COMMIT    := $(shell git rev-parse --short HEAD 2>/dev/null || echo "dev")
BUILD_TIME    := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
VERSION       := $(shell git describe --tags --always --dirty 2>/dev/null || echo "v0.1.0-dev")

GO ?= go
GOFUMPT ?= gofumpt
GOLANGCI_LINT ?= golangci-lint

GOOS := $(shell $(GO) env GOOS)
GOARCH := $(shell $(GO) env GOARCH)

## —— Build & Test ——

all: clean build test vet lint ## Build, test, vet, and lint

build: ## Build the scrappy binary for current platform
	$(GO) build $(GO_FLAGS) -o $(OUT_DIR)/$(BINARY_NAME)_$(GOOS)_$(GOARCH) $(CMD_DIR)

build-all: ## Cross-compile for all platforms
	GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 $(GO) build $(GO_FLAGS) -o $(OUT_DIR)/$(BINARY_NAME)_linux_amd64 $(CMD_DIR)
	GOOS=darwin  GOARCH=arm64 CGO_ENABLED=0 $(GO) build $(GO_FLAGS) -o $(OUT_DIR)/$(BINARY_NAME)_darwin_arm64 $(CMD_DIR)
	GOOS=darwin  GOARCH=amd64 CGO_ENABLED=0 $(GO) build $(GO_FLAGS) -o $(OUT_DIR)/$(BINARY_NAME)_darwin_amd64 $(CMD_DIR)
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 $(GO) build $(GO_FLAGS) -o $(OUT_DIR)/$(BINARY_NAME)_windows_amd64.exe $(CMD_DIR)
	@echo "Cross-compiled binaries in $(OUT_DIR)/"
	@(cd $(OUT_DIR) && sha256sum *) || (cd $(OUT_DIR) && shasum -a 256 *)

test: ## Run all unit tests
	$(GO) test ./... -count=1 -timeout=120s

test-race: ## Run tests with race detector
	$(GO) test -race ./... -count=1 -timeout=300s

test-short: ## Run short tests only (skip integration)
	$(GO) test ./... -short -count=1 -timeout=60s

test-cover: ## Run tests with coverage report
	$(GO) test ./... -coverprofile=coverage.out -covermode=atomic -count=1 -timeout=120s
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

vet: ## Run go vet
	$(GO) vet ./...

lint: ## Run golangci-lint (if installed)
	$(GOLANGCI_LINT) run ./... 2>/dev/null || echo "golangci-lint not installed — skipping"

fumpt: ## Run gofumpt (if installed)
	$(GOFUMPT) -l -w . 2>/dev/null || echo "gofumpt not installed — skipping"

tidy: ## Tidy and verify go modules
	$(GO) mod tidy
	$(GO) mod verify

## —— Benchmarks ——

bench: ## Run benchmarks
	$(GO) test -bench=. -benchmem ./...

bench-cpu: ## CPU profile
	$(GO) test -bench=. -benchmem -cpuprofile=cpu.prof ./...

bench-mem: ## Memory profile
	$(GO) test -bench=. -benchmem -memprofile=mem.prof ./...

pprof-cpu: ## Analyze CPU profile
	$(GO) tool pprof -http=:6060 cpu.prof

pprof-mem: ## Analyze memory profile
	$(GO) tool pprof -http=:6060 mem.prof

## —— Fuzzing ——

fuzz: ## Run fuzz tests (5 min each)
	$(GO) test -fuzz=FuzzEmail -fuzztime=5m ./internal/email/...
	$(GO) test -fuzz=FuzzStripHTML -fuzztime=5m ./internal/util/...
	$(GO) test -fuzz=FuzzNormalizeSalary -fuzztime=5m ./internal/normalize/...

## —— Docker ——

docker: ## Build Docker image
	docker build -t scrappy:$(VERSION) -f Dockerfile .
	docker tag scrappy:$(VERSION) scrappy:latest

docker-run: ## Run scrappy in Docker
	docker run --rm -v $$(pwd)/output:/output scrappy:latest

## —— Clean ——

clean: ## Remove build artifacts
	rm -rf $(OUT_DIR)/*
	rm -f coverage.out coverage.html
	rm -f cpu.prof mem.prof
	rm -rf tmp/

## —— Development ——

dev-deps: ## Install development dependencies
	go install mvdan.cc/gofumpt@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

install: build ## Build and install the binary
	cp $(OUT_DIR)/$(BINARY_NAME)_$(GOOS)_$(GOARCH) /usr/local/bin/$(BINARY_NAME)
	@echo "Installed /usr/local/bin/$(BINARY_NAME)"

## —— Release ——

release: clean build-all ## Build release artifacts
	@echo "Release artifacts:"
	@ls -la $(OUT_DIR)/

## —— Help ——

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
