# Glassdoor

## Current integration
- Source mode: HTML parsing of listing page.
- Extracted fields: job id, title, company.

## Fragile points
- CSS class names and `data-jobid` attribute.
- Country/domain differences (`.com` vs `.co.uk`, etc.).

## Debug checklist
1. Capture page HTML and validate `data-jobid` still exists.
2. Update title/company selectors if class names shifted.
3. Confirm anti-bot/consent interstitials are not being parsed as jobs.
