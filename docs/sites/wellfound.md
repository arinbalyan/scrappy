# Wellfound

## Current integration
- Source mode: HTML parsing from listing page.
- Extracted fields: url, title, company.

## Fragile points
- Card markup and class names.
- Relative vs absolute URLs.

## Debug checklist
1. Capture raw HTML and verify anchor/title/company blocks.
2. Rework selectors when markup changes.
3. Normalize URL construction consistently.
