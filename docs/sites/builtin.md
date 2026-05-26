# BuiltIn

**Status: DEPRECATED — HTML structure changed.**

Reason: `__NEXT_DATA__` regex no longer matches current HTML. BuiltIn changed their page rendering, breaking both the structured data extraction and the fallback HTML card parsing.

The scraper remains in the codebase but returns empty results until the HTML structure is re-parsed.
