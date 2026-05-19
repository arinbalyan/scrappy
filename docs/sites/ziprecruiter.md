# ZipRecruiter

## Current integration
- Source mode: HTML parsing of listing page.
- Extracted fields: job id, url, title, company.

## Fragile points
- HTML classes (`job_content`, `t_org_link`) and card structure.
- Anti-bot pages replacing normal content.

## Debug checklist
1. Verify card regex still matches real listing HTML.
2. Confirm job URL extraction still returns absolute URLs.
3. Add/revise parser fallback if markup changes.
