# Google Jobs

**Status: DEPRECATED — HTML structure changed.**

Reason: Google SERP HTML structure changed. The embedded `application/ld+json` JobPosting blocks and the fallback SERP HTML selectors (`data-job-id` + class selectors) no longer match current page output. Returns 200 with no parseable jobs.

The scraper remains in the codebase but returns empty results until the HTML structure is re-parsed.
