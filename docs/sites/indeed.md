# Indeed

## Current integration
- Endpoint: `https://apis.indeed.com/graphql`
- Query shape: `jobSearch(what, location, limit=100, cursor, filters)`
- Pagination: `pageInfo.nextCursor`
- Key params from input:
  - `search_term` -> `what`
  - `location` + `distance` -> `location.where/radius`
  - `hours_old` -> `filters.date.start = "{hours}h"`
  - `easy_apply` -> `filters.keyword(field=indeedApplyScope, keys=[DESKTOP])`
  - `job_type`/`is_remote` -> `filters.composite.keyword(field=attributes, keys=[...])`

## Fragile points / likely breakpoints
- GraphQL field names under `data.jobSearch.results[].job.*`
- Compensation subtree (`estimated/baseSalary/range`)
- Employer detail nesting (`employer.dossier.employerDetails`)
- Attribute keys for job type/remote filters

## Debug checklist when it breaks
1. Log raw status/body for non-2xx.
2. Validate GraphQL response path: `data.jobSearch.results`.
3. Validate cursor progression (`nextCursor` not empty/stuck).
4. Re-check filter key constants against live payload.
5. Re-check compensation unit mapping (`YEAR/MONTH/WEEK/DAY/HOUR`).
