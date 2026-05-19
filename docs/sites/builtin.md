# BuiltIn

## Current integration
- Source mode: HTML parsing from listing page.
- Extracted fields: url, title, company.

## Fragile points
- Card block structure and classes.
- City-specific pages can differ in markup.

## Debug checklist
1. Verify parser against each target city variant.
2. Keep selector strategy tolerant to optional wrappers.
3. Add fixture tests when parsing logic changes.
