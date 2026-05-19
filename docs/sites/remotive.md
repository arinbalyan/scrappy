# Remotive

## Current integration
- Endpoint: `https://remotive.com/api/remote-jobs`
- Optional query: `search`
- Extracted fields: id, title, company_name, candidate_required_location, publication_date, url, description, category.

## Fragile points
- Job list key (`jobs`) and field names.
- Date format in `publication_date`.

## Debug checklist
1. Validate `jobs` array exists and is non-empty.
2. Validate RFC3339 date parsing; add fallback parser if needed.
3. Keep mapping tolerant to missing optional fields.
