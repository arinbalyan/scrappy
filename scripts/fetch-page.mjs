#!/usr/bin/env node
/**
 * fetch-page.mjs — Browser-based page fetcher for scrappy.
 *
 * Uses Playwright to render a URL in headless Chromium and returns the
 * full HTML + cookies as JSON on stdout. Designed to be called via
 * os/exec from the Go scraper when a plain HTTP request is blocked by
 * anti-bot challenges (DataDome, Cloudflare, reCAPTCHA).
 *
 * Usage:
 *   node scripts/fetch-page.mjs <url> [--wait <selector>] [--timeout <ms>]
 *
 * Output (stdout JSON):
 *   { "html": "...", "cookies": [{"name":"...","value":"..."}], "status": 200 }
 *
 * Exit code: 0 on success, non-zero on error (error text on stderr).
 */

import { chromium } from "playwright";

const args = process.argv.slice(2);
if (args.length === 0) {
  console.error("Usage: fetch-page.mjs <url> [--wait <selector>] [--timeout <ms>]");
  process.exit(1);
}

const url = args[0];
const waitSelector = args.indexOf("--wait") !== -1 ? args[args.indexOf("--wait") + 1] : null;
const timeoutMs = args.indexOf("--timeout") !== -1
  ? parseInt(args[args.indexOf("--timeout") + 1], 10) || 30000
  : 30000;

async function main() {
  const browser = await chromium.launch({
    headless: true,
    args: [
      "--no-sandbox",
      "--disable-setuid-sandbox",
      "--disable-dev-shm-usage",
      "--disable-gpu",
    ],
  });

  const context = await browser.newContext({
    userAgent:
      "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36",
    viewport: { width: 1920, height: 1080 },
    locale: "en-US",
  });

  const page = await context.newPage();

  try {
    const response = await page.goto(url, {
      waitUntil: "networkidle",
      timeout: timeoutMs,
    });

    const status = response ? response.status() : 0;

    // If a wait selector is provided, wait for it to appear.
    if (waitSelector) {
      try {
        await page.waitForSelector(waitSelector, { timeout: 5000 });
      } catch {
        // Selector may not appear; continue with whatever rendered.
      }
    }

    // Give any async JS a moment to finish.
    await page.waitForTimeout(1000);

    const html = await page.content();
    const cookies = await context.cookies();

    // Filter cookies to only essential fields.
    const cookieList = cookies.map((c) => ({
      name: c.name,
      value: c.value,
      domain: c.domain,
      path: c.path,
    }));

    // Output JSON — this is what Go reads from stdout.
    const output = JSON.stringify({ html, cookies: cookieList, status });
    console.log(output);
  } finally {
    await browser.close();
  }
}

main().catch((err) => {
  console.error(err.message || String(err));
  process.exit(1);
});
