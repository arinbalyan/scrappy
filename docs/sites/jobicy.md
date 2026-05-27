# Jobicy

**Status: DEPRECATED — HTML structure changed.**

Reason: Jobicy returns HTTP 400 because the scraper passes an invalid `industry` slug. The site expects valid industry slug values; the current integration does not map search terms to valid slugs, so the response contains no job data.

The scraper remains in the codebase but returns empty results until the slug mapping is fixed or the HTML structure is re-parsed.
