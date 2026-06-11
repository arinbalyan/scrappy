---
name: Bug report
about: Report a scraper failure, crash, or unexpected behavior
title: "[Bug] "
labels: bug
assignees: ''
---

## Bug Description

A clear and concise description of the issue.

## Command Used

```bash
scrappy --email --results-wanted 10 --site linkedin --log-level DEBUG
```

## Expected Behavior

What should have happened.

## Actual Behavior

What actually happened. Paste full output including error messages.

## Environment

- **scrappy version**: (`scrappy --version` or commit SHA)
- **OS**: (Linux/macOS/Windows)
- **Source**: binary release / built from source
- **Go version** (if building from source): `go version`

## Debug Logs

Run with `--log-level DEBUG` and paste relevant output:

```
[DEBUG] ...
[INFO] ...
[ERROR] ...
```

## Additional Context

- Does this happen consistently or intermittently?
- Does it affect all sites or just specific ones?
- Any proxy or network configuration?

## Checklist

- [ ] I searched existing issues before filing this report
- [ ] I included `--log-level DEBUG` output
- [ ] I can reproduce the issue consistently
