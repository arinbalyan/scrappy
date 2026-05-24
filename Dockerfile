# syntax=docker/dockerfile:1
FROM golang:1.26-alpine AS builder
ARG VERSION=0.1.0
WORKDIR /app
COPY go.* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build \
    -ldflags="-s -w" \
    -trimpath \
    -o /scrappy \
    ./cmd/scrappy

FROM gcr.io/distroless/static-debian12:nonroot
ARG VERSION=0.1.0
LABEL org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.description="Bulk job-board scraper for lead generation" \
      org.opencontainers.image.source="https://github.com/arinbalyan/scrappy"
COPY --from=builder /scrappy /scrappy
ENTRYPOINT ["/scrappy"]
