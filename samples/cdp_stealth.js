#!/usr/bin/env node
/*
 * Connect to the always-on Chromium over CDP and drive it with stealth patches.
 *
 * The container boots a headful Chromium with DevTools on 127.0.0.1:9222 (see
 * entrypoint.sh). This ATTACHES to that browser (does not launch one), so the
 * automation is visible live in /vnc/.
 *
 * Uses puppeteer-core (no bundled Chromium download). Install once inside the
 * container, then run:
 *     npm i -g puppeteer-core        # or: cd /opt/samples && npm i puppeteer-core
 *     node /opt/samples/cdp_stealth.js https://bot.sannysoft.com
 *
 * The stealth block is dependency-free — the same evasions as the Python
 * sample. For heavy fingerprinting, swap in puppeteer-extra + the stealth
 * plugin.
 */
const puppeteer = require('puppeteer-core');

const CDP_URL = 'http://127.0.0.1:9222';

const STEALTH_JS = () => {
  Object.defineProperty(navigator, 'webdriver', { get: () => undefined });
  window.chrome = window.chrome || { runtime: {} };
  Object.defineProperty(navigator, 'plugins', { get: () => [1, 2, 3, 4, 5] });
  Object.defineProperty(navigator, 'languages', { get: () => ['en-US', 'en'] });
  const _query = window.navigator.permissions && window.navigator.permissions.query;
  if (_query) {
    window.navigator.permissions.query = (p) =>
      p && p.name === 'notifications'
        ? Promise.resolve({ state: Notification.permission })
        : _query(p);
  }
  const _getParameter = WebGLRenderingContext.prototype.getParameter;
  WebGLRenderingContext.prototype.getParameter = function (p) {
    if (p === 37445) return 'Intel Inc.';
    if (p === 37446) return 'Intel Iris OpenGL Engine';
    return _getParameter.call(this, p);
  };
};

(async () => {
  const url = process.argv[2] || 'https://bot.sannysoft.com';
  // Attach to the running browser (does not spawn a new one).
  const browser = await puppeteer.connect({ browserURL: CDP_URL, defaultViewport: null });
  const pages = await browser.pages();
  const page = pages[0] || (await browser.newPage());
  await page.evaluateOnNewDocument(STEALTH_JS);
  await page.goto(url, { waitUntil: 'domcontentloaded' });
  console.log('title:', await page.title());
  console.log('webdriver:', await page.evaluate(() => navigator.webdriver));
  // Leave the page open; detach without closing the browser.
  browser.disconnect();
})();
