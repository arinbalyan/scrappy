---
name: Performance or memory issue
about: Report high memory usage, slow scraping, or performance regressions
title: "[Perf] "
labels: performance
assignees: ''
---

## Performance Issue

Describe the performance or memory problem you're experiencing.

## Metrics

- **Number of sites scraped**: 
- **Results wanted**: 
- **Peak memory usage**: 
- **Total runtime**: 
- **Number of jobs collected**: 

## Command Used

```bash
scrappy --email --results-wanted 100 --memory-cap 512MB --log-level DEBUG
```

## Environment

- **scrappy version**:
- **OS**:
- **RAM available**:
- **CPU count**:
- **Go version** (if from source):

## Memory Profile (if available)

If you can run with `--log-level DEBUG`, include the memory section from the output:

```
[INFO] resource_usage ...
```

## Additional Context

- Does memory grow steadily or spike?
- Are there specific sites that cause high memory usage?
- Are you using proxy rotation?
- How many concurrent scrapes were running?

## Checklist

- [ ] I searched existing issues before filing this report
- [ ] I included resource usage logs
- [ ] I can reproduce the issue
