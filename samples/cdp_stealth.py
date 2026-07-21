#!/usr/bin/env python3
"""Connect to the always-on Chromium over CDP and drive it with stealth patches.

The container boots a headful Chromium with DevTools listening on
127.0.0.1:9222 (see entrypoint.sh). This attaches to THAT browser — it does not
launch a new one — so whatever it does is visible live in /vnc/.

The stealth block is dependency-free: a set of init scripts that mask the most
common headless/automation tells (navigator.webdriver, missing chrome object,
empty plugins, permissions mismatch, WebGL vendor). Good enough for most bot
checks; for heavier fingerprinting add a maintained library.

Run inside the container:
    python3 /opt/samples/cdp_stealth.py https://bot.sannysoft.com
"""
import sys
from playwright.sync_api import sync_playwright

CDP_URL = "http://127.0.0.1:9222"

# Injected into every page/frame before any site script runs.
STEALTH_JS = r"""
// navigator.webdriver -> undefined
Object.defineProperty(navigator, 'webdriver', {get: () => undefined});
// window.chrome present (headless Chrome omits it)
window.chrome = window.chrome || { runtime: {} };
// non-empty plugins & mimeTypes
Object.defineProperty(navigator, 'plugins', {get: () => [1, 2, 3, 4, 5]});
Object.defineProperty(navigator, 'languages', {get: () => ['en-US', 'en']});
// Notification.permission consistency
const _query = window.navigator.permissions && window.navigator.permissions.query;
if (_query) {
  window.navigator.permissions.query = (p) =>
    p && p.name === 'notifications'
      ? Promise.resolve({ state: Notification.permission })
      : _query(p);
}
// WebGL vendor/renderer spoof
const _getParameter = WebGLRenderingContext.prototype.getParameter;
WebGLRenderingContext.prototype.getParameter = function (p) {
  if (p === 37445) return 'Intel Inc.';           // UNMASKED_VENDOR_WEBGL
  if (p === 37446) return 'Intel Iris OpenGL Engine'; // UNMASKED_RENDERER_WEBGL
  return _getParameter.call(this, p);
};
"""


def main():
    url = sys.argv[1] if len(sys.argv) > 1 else "https://bot.sannysoft.com"
    with sync_playwright() as p:
        # Attach to the running browser over CDP (does not spawn a new one).
        browser = p.chromium.connect_over_cdp(CDP_URL)
        # Reuse the existing default context so the window stays visible in VNC.
        ctx = browser.contexts[0] if browser.contexts else browser.new_context()
        ctx.add_init_script(STEALTH_JS)
        page = ctx.pages[0] if ctx.pages else ctx.new_page()
        page.goto(url, wait_until="domcontentloaded")
        print("title:", page.title())
        print("webdriver:", page.evaluate("() => navigator.webdriver"))
        # Leave the page open so it stays on screen; don't close the browser.


if __name__ == "__main__":
    main()
