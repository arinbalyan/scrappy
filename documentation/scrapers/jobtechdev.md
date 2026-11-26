# jobtechdev

Source id: jobtechdev
Family: keyed
Health: keyed
Endpoint: GET https://jobsearch.api.jobtechdev.se/search?offset=0&limit=2
Parser: json with `hits` envelope
Fetcher: plain
Credentials: JOBTECHDEV_API_KEY
Rate: 20 requests per minute
Concurrency: 2
Robots: allow credentialed public API use; skip without credentials
Attribution: JobTechDev / Arbetsförmedlingen

## Behavior

- Search is server-side through the `q` query field; the manifest fixture carries a frozen query URL.
- Headline supplies the title, employer name the company, and webpage URL the canonical job URL.
- The application URL is the URL fallback when a posting has no public webpage.
- Municipality supplies location, with the city as the second alias.
- Description text is read from the nested `description.text` field.
- The historical scraper's offset pagination is outside the single-request adapter; the fixture covers one page.

## Quirks

- The Swedish API requires an `api-key` header; replay uses a synthetic credential value.
- Employment type, salary, deadlines, and remote metadata from the historical scraper stay outside the normalized record contract.
- The live API returned HTTP 200 with a valid `hits` envelope when probed with a test key.

## Break and fix log

- 2026-11-26: Ported the JSON search API with a keyed manifest, synthetic credential fixture, and offline replay.

## Live validation

- 2026-11-26: GET with query `rust` returned HTTP 200 and a populated `hits` array using a test key; health stays `keyed` because a real key is required.
