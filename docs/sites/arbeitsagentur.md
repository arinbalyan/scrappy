# Arbeitsagentur

## Current integration
- Parser order:
  1. JSON API at `https://rest.arbeitsagentur.de/jobboerse/jobsuche-service/pc/v4/jobs`
- Extracted fields: refnr, titel, arbeitgeber, beruf, arbeitsort, eintrittsdatum, aktuelleVeroeffentlichungsdatum, homeOffice, externeUrl, uepiAnpiLogo

## Supported knobs
- `results_wanted`
- `search_term` — server-side via `was` param
- `location` — server-side via `wo` param
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)

## Constraints and breakpoints
- **Requires `ARBEITSAGENTUR_API_KEY`** environment variable, sent as `X-API-Key` header.
- API returns German-language fields — no English parameter aliases.
- Pagination is page-based with `size=100` per page, driven by `seite` param.
- `hasMore` is determined by `seite*100 < maxErgebnisse`.
- Date format supports RFC3339, ISO 8601, and German formats (`02.01.2006`, `02/01/2006`).
- Job URL is constructed as a search link using the `refnr`.

## Debug/update playbook
1. Verify `ARBEITSAGENTUR_API_KEY` is set.
2. Confirm API responds at the REST endpoint.
3. Check the `stellenangebote` structure in the response.
4. Validate German date parsing for any new formats.
5. Re-run `go test ./tests/scraper/arbeitsagentur` and full suite.
