# SimplyHired

**Status: DEPRECATED — HTML structure changed.**

Reason: SimplyHired now returns a Cloudflare challenge page (HTTP 403). The CSS selectors and regex patterns (`data-testid="searchSerpJob"`, `SerpJob` class selectors) cannot match the Cloudflare interstitial HTML, so no jobs are parsed.

The scraper remains in the codebase but returns empty results until the Cloudflare challenge is resolved or the HTML structure is re-parsed.
