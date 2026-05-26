.PHONY: build test test-race vet lint clean docker

build:
	go build -ldflags="-s -w" -o bin/scrappy ./cmd/scrappy

test:
	go test -count=1 ./...

test-race:
	go test -race -count=1 ./...

vet:
	go vet ./...

lint:
	golangci-lint run ./...

clean:
	rm -rf bin/

docker:
	docker build -t scrappy .

all: build test vet
