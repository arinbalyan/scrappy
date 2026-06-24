# Browser Automation

scrappy uses a Playwright-based browser fallback for sites that block plain HTTP requests with anti-bot challenges (DataDome, Cloudflare, reCAPTCHA, etc.). The browser automation is **optional** — if the Playwright script is not installed, scrapers fall through gracefully.

---

## Architecture

The browser subsystem has two parts:

```
internal/browser/client.go      ← Go code that calls the script
scripts/fetch-page.mjs           ← Playwright script with stealth
```

The Go code shells out to Node.js via `os/exec`:

```
Go: FetchPage(ctx, url, waitSelector)
    → exec("node", ["scripts/fetch-page.mjs", url])
    ← JSON { html, cookies, status }
```

---

## Playwright Script — `scripts/fetch-page.mjs`

### Dependencies

```bash
cd scripts
npm install playwright puppeteer-extra-plugin-stealth
```

Requires Node.js and Chromium (installed via `npx playwright install chromium`).

### Script logic

1. Launches headless Chromium with stealth plugin:

```javascript
import { chromium } from "playwright-extra";
import stealth from "puppeteer-extra-plugin-stealth";
chromium.use(stealth());
```

2. Browser args for container/server environments:

```javascript
const browser = await chromium.launch({
  headless: true,
  args: [
    "--no-sandbox",
    "--disable-setuid-sandbox",
    "--disable-dev-shm-usage",
    "--disable-gpu",
  ],
});
```

3. Creates a context with a modern Chrome user-agent and 1920×1080 viewport.

4. Navigates to the URL with `waitUntil: "networkidle"` and a 30-second timeout.

5. If `--wait <selector>` is provided, waits for the CSS selector (5s timeout, non-fatal).

6. Waits an additional 1 second for JS rendering.

7. Returns `{ html, cookies, status }` as JSON on stdout.

### Usage

```bash
# Basic fetch
node scripts/fetch-page.mjs https://example.com

# With wait-for-selector
node scripts/fetch-page.mjs https://example.com --wait .job-listing
```

### Stealth features

The `puppeteer-extra-plugin-stealth` plugin patches dozens of browser fingerprint vectors:
- WebGL vendor/renderer spoofing
- Chrome runtime detection evasion
- `navigator.webdriver` set to `false`
- Headless-specific feature removal
- Plugin array normalization

---

## Go Client — `internal/browser/client.go`

### `FetchPage(ctx, targetURL, waitSelector) (*PageResult, error)`

```go
func FetchPage(ctx context.Context, targetURL string, waitSelector string) (*PageResult, error)
```

**Returns:**

```go
type PageResult struct {
    HTML    string   `json:"html"`
    Cookies []Cookie `json:"cookies"`
    Status  int      `json:"status"`
}
```

**Behavior:**
- Detects `scripts/fetch-page.mjs` next to the binary, in CWD, or in parent directories.
- Validates the URL (must be HTTP/HTTPS with a host).
- Default timeout: 45s (or remaining deadline from ctx).
- **One retry** on transient errors (browser boot failure, timeout).
- Stderr from the script is captured and truncated to 1200 chars for error messages.
- Failure to find the script returns a clear error: `"browser: fetch-page.mjs not found — install Playwright and run npm install in scripts/"`.

### `IsAvailable() bool`

Returns `true` if both `node` and the script are found:

```go
func IsAvailable() bool {
    _, err := exec.LookPath("node")
    if err != nil {
        return false
    }
    return detectScriptPath() != ""
}
```

### `CheckDependencies() error`

Returns a human-readable error if Node.js, the script, or Playwright modules are missing.

---

## When is browser fallback used?

Scrapers choose their fetch method based on the site's anti-bot posture:

| Method | Description |
|--------|-------------|
| `GET` | Standard HTTP GET for open RSS feeds and APIs |
| `POST` | Form-encoded search for sites requiring searches |
| `GET-with-browser` | Headless browser for Cloudflare/DataDome sites |

The method for each site is defined in `internal/scraper/*/scrape.go`:

```go
func Method(site model.Site) string {
    switch site {
    case model.SiteGoogle, model.SiteMonster:
        return "GET-with-browser"
    default:
        return "GET"
    }
}
```

When a scraper detects a challenge (based on response content, status codes, or CAPTCHA keywords), it can opt into browser rendering for that request. The browser result's cookies can also be injected into subsequent HTTP requests.

### Sites that use browser fallback

Currently:
- **Google Jobs** — returns JavaScript-rendered content
- **Monster** — anti-bot protection
- **LinkedIn** (some endpoints) — rate-limit mitigation
- **Indeed** (some regions) — anti-bot challenges

---

## Performance considerations

- Each browser launch takes 2-5 seconds. The Go client automatically retries once.
- Browser-based scraping is **significantly slower** than HTTP scraping — use only for sites that require it.
- The default timeout is 45s per page load.
- Concurrency is not capped at the browser level; the engine's global semaphore limits how many concurrent browses run.
- Memory per browser instance: ~100-200 MB.
