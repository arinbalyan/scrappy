#!/usr/bin/env node
/**
 * fetch-page.mjs — Browser-based page fetcher with stealth anti-detection.
 *
 * Uses Playwright with puppeteer-extra-plugin-stealth to render a URL in
 * headless Chromium and returns the full HTML + cookies as JSON on stdout.
 *
 * Usage:
 *   node scripts/fetch-page.mjs <url> [--wait <selector>] [--timeout <ms>]
 */

import { chromium } from "playwright-extra";
import stealth from "puppeteer-extra-plugin-stealth";

chromium.use(stealth());

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

    if (waitSelector) {
      try {
        await page.waitForSelector(waitSelector, { timeout: 5000 });
      } catch {
        // selector may not appear
      }
    }

    await page.waitForTimeout(1000);

    const html = await page.content();
    const cookies = await context.cookies();

    const output = JSON.stringify({
      html,
      cookies: cookies.map((c) => ({
        name: c.name,
        value: c.value,
        domain: c.domain,
        path: c.path,
      })),
      status,
    });
    console.log(output);
  } finally {
    await browser.close();
  }
}

main().catch((err) => {
  console.error(err.message || String(err));
  process.exit(1);
});
