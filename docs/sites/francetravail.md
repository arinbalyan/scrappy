# France Travail (Pôle emploi)

## Current integration
- Parser order:
  1. JSON API at `https://api.francetravail.io/partenaire/offresdemploi/v2/offres/search`
  2. OAuth2 client credentials flow for authentication
- Extracted fields: id, intitule, description, dateCreation, lieuTravail, entreprise, origineOffre

## Supported knobs
- `results_wanted` — capped at 50
- `search_term` — server-side via `motsCles` param
- shared resilience knobs (`retries`, `max_rps`, `site_rps`, proxy config)

## Constraints and breakpoints
- **Requires `FRANCETRAVAIL_CLIENT_ID` and `FRANCETRAVAIL_CLIENT_SECRET`** environment variables for OAuth2.
- Uses OAuth2 client credentials flow: POST to token endpoint with `client_credentials` grant type.
- Access token is cached and auto-refreshed 60 seconds before expiry.
- Pagination via `range` header: `0-{wanted-1}` — single request batch.
- Location is hardcoded to `Country: "France"`.
- Job URL from `origineOffre.urlOrigine` or constructed fallback link.
- API scope: `api_offresdemploiv2 o2dsoffre`.

## Debug/update playbook
1. Verify `FRANCETRAVAIL_CLIENT_ID` and `FRANCETRAVAIL_CLIENT_SECRET` are set.
2. Confirm the OAuth2 token endpoint responds.
3. Check the search API for response structure changes.
4. Re-run `go test ./tests/scraper/francetravail` and full suite.
