# Overview

`scrappy` is a bulk job-board scraper written in Go. It ports and extends an open-source Python job-board scraping library, targeting better memory efficiency, faster concurrency, fewer third-party dependencies, and email enrichment.

## Why Go

| Concern | JobSpy (Python) | scrappy (Go) |
|---|---|---|
| Concurrency | ThreadPoolExecutor (OS-thread bound) | goroutines + errgroup (lightweight) |
| Memory | High (per-thread stack, GIL overhead) | Low (goroutine stack approx 2 KB) |
| Binary | Requires Python runtime | Single static binary (5–10 MB) |
| Proxy rotation | Round-robin, no health checks | SOCKS5 pool with pre-flight probes |
| Email enrichment | Bare regex, no validation | Extract → normalize → MX lookup → company-page follow-up |
| Deduplication | Per-DataFrame only | Cross-site `sync.Map` dedup |
| Exports | pandas DataFrame only | CSV, JSONL, XLSX, Parquet |

## Bulk-first design

`scrappy` is not designed for one-off lookups — it is designed for volume operations: fanning out across many sites concurrently, running each at its own safe RPS limit, and streaming the result to disk without materializing everything in memory.

```bash
scrappy scrape --sites linkedin,indeed,glassdoor,remoteok \
               --search "software engineer" --location "Remote" \
               --results-wanted 5000 \
               --format parquet --out /data/jobs.parquet
```

## Repository layout

```
scrappy/
# [the archive file truncated and lost some content.]
