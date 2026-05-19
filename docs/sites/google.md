# Google Jobs

## Current integration
- Source mode: SERP HTML parsing.
- Query input:
  - prefers `google_search_term`
  - falls back to `search_term`
- Extracted fields: id/title/company from known blocks.

## Fragile points
- SERP structure/classes are volatile.
- Geo/locale/consent pages can alter output shape.

## Debug checklist
1. Validate response is a real jobs SERP, not consent/captcha page.
2. Re-check `data-job-id` and title/company class blocks.
3. Keep parser tolerant to class drift (multiple selector candidates).
