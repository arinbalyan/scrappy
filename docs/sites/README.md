# Supported Sites

scrappy scrapes over 100 job boards and ATS endpoints. Use the search bar or browse the section below.

## Quick Reference

| Site Type | Count | Examples |
|-----------|-------|---------|
| Major Boards | 11 | LinkedIn, Indeed, Google, Monster, Dice, Reed, ZipRecruiter |
| ATS Providers | 39 | Workday, Taleo, BambooHR, Greenhouse, Lever, Ashby, Personio |
| Remote-Focused | 7 | RemoteOK, Remotive, WeWorkRemotely, WorkingNomads |
| Government | 6 | USAJobs, Canada Job Bank, CareerOneStop, UNDP Jobs |
| Regional | 17 | Bayt, StepStone, France Travail, Wuzzuf, Jobs in Japan |
| Niche & Tech | 84 | HackerNews, Crunchboard, CryptoJobs, Tesla, Upwork |

## Using sites from the CLI

```bash
# Single site
scrappy --sites linkedin --search "engineer"

# Multiple sites
scrappy --sites linkedin,indeed,google --search "software engineer"

# All sites (default)
scrappy --search "developer" --results-wanted 100
```

## Site Documentation

Select a site from the sidebar for its specific scraping notes, supported features, and known limitations.
