# Memory Management

`scrappy` schedules 65+ scrapers concurrently, each loading pages, parsing HTML/JSON, and accumulating results. Without a memory budget, the runtime can consume hundreds of MB — especially when many sites return large descriptions or company pages.

## `--memory-cap` flag

Limits total Go heap by scaling global concurrency. Accepts:

```
--memory-cap 512MB
--memory-cap 1GB
--memory-cap 256       # plain number = MB
--memory-cap 0         # unlimited (default)
```

## Concurrency tiers

When `--memory-cap` is set, `globalConcurrency` is pinned to a safe level:

| Memory cap | Max concurrent scrapers |
|---|---|
| ≤256 MB | 3 |
| ≤512 MB | 5 |
| ≤1 GB | 8 |
| >1 GB | 12 |

Without a cap (default), the engine uses **8 concurrent scrapers**, or `--max-rps` if set (clamped between 2 and 16).

## Heap monitor

When a memory cap is configured, a background goroutine runs every 10 seconds and reads `runtime.MemStats`. If `Alloc` exceeds **80% of the cap**, a WARN-level message is logged:

```
memory_pressure: alloc_mb=450 cap_mb=512 pct=88 gc_cycles=42
```

This is advisory only — the engine does not kill scrapers. Use it to tune your cap or reduce the number of sites.

## Recommended caps

| Environment | Recommended cap | Reasoning |
|---|---|---|
| Laptop (8–16 GB RAM) | 512 MB | Leaves room for browser, editor, other apps |
| VPS / dedicated (2–4 GB) | 1 GB | Can use higher concurrency for throughput |
| Docker container | 256 MB | Tight budget, best-effort scraping |
| CI runner | 256 MB | Shared resources, avoid OOM kills |

## Example

```bash
# Laptop: 5 concurrent scrapers, heap warning at ~410 MB
scrappy --memory-cap 512MB --sites linkedin,indeed,remoteok \
  --search "golang" --results-wanted 500 --format jsonl

# Container: 3 concurrent scrapers
scrappy --memory-cap 256MB --sites indeed --search "devops" \
  --results-wanted 200 --format csv --out /data/jobs.csv
```

## Monitoring

```bash
# Verbose logging shows memory telemetry
scrappy --memory-cap 512MB --log-level DEBUG --sites indeed \
  --search "engineer" --results-wanted 100
```

Look for `memory_pressure` warnings in the output. If they appear frequently, lower your cap or reduce `--results-wanted`.
