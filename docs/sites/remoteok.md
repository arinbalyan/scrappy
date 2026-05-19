# RemoteOK

## Current integration
- Endpoint: `https://remoteok.com/api`
- Response shape: array where index 0 is metadata, remaining entries are jobs.
- Extracted fields: id, position, company, url, epoch.

## Fragile points
- API response envelope (metadata row behavior).
- Field names (`position`, `company`, `url`, `epoch`).

## Debug checklist
1. Confirm first item is metadata; skip only if schema still matches.
2. Validate required fields for each row before mapping.
3. Guard against mixed types/nulls in numeric fields.
