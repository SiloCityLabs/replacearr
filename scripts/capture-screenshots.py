#!/usr/bin/env python3
"""Capture Replacearr UI screenshots for desktop or mobile. Redacts hex API keys."""

import argparse
import base64
import json
import subprocess
import time
import urllib.request
from pathlib import Path

import websocket

REPO = Path(__file__).resolve().parents[1]
WEB = "http://127.0.0.1:5173/"
API = "http://127.0.0.1:8989/api/v1/auth/login"

REDACT = r"""
(() => {
  const hex = /[a-f0-9]{24,}/gi;
  const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
  let node;
  while ((node = walker.nextNode())) {
    if (hex.test(node.textContent || '')) {
      hex.lastIndex = 0;
      node.textContent = node.textContent.replace(hex, 'your-api-key');
    } else {
      hex.lastIndex = 0;
    }
  }
  document.querySelectorAll('input').forEach((input) => {
    if (input.type === 'password') input.value = '';
  });
  return 'redacted';
})()
"""

# (filename, nav_label|None, wait_needle, kind)
# kind: "main" opens mobile hamburger first; "settings" prefers settings sidebar buttons
PAGES = [
    ("00-login.png", None, "Sign In", "main"),
    ("01-dashboard.png", "Dashboard", "Welcome to Replacearr", "main"),
    ("02-series.png", "TV Series", "TV Library", "main"),
    ("03-movies.png", "Movies", "Movies Library", "main"),
    ("04-indexers-search.png", "Indexers", "YAML Definitions", "main"),
    ("05-queue.png", "Queue & Clients", "Live Download Queue", "main"),
    ("06-plugins.png", "Go Plugins", "Go Plugin System", "main"),
    ("07-users.png", "Users & RBAC", "Users & Access Control", "main"),
    ("08-settings.png", "Settings", "Episode and movie naming", "main"),
    ("09-settings-general.png", "General", "Bind Address", "settings"),
    ("10-settings-ui.png", "UI", "First Day of Week", "settings"),
    ("11-settings-media.png", "Media Management", "Episode Naming", "settings"),
    ("12-settings-profiles.png", "Profiles", "Quality Profiles", "settings"),
    ("13-settings-quality.png", "Quality", "MB per minute", "settings"),
    ("14-settings-indexers.png", "Indexers", "Indexer rows save", "settings"),
    ("15-settings-download-clients.png", "Download Clients", "Client records are live", "settings"),
    ("16-settings-import-lists.png", "Import Lists", "List sync options", "settings"),
    ("17-settings-system.png", "System", "Compat endpoints", "settings"),
    ("18-settings-metadata.png", "Metadata", "Preferred language", "settings"),
    ("19-settings-tags.png", "Tags", "Add Tag", "settings"),
    ("20-compatibility.png", "Go Plugins", "Overseerr", "main"),
]


class CDP:
    def __init__(self, url: str):
        self.ws = websocket.create_connection(url, max_size=64_000_000, timeout=30)
        self.n = 0

    def call(self, method: str, params=None):
        self.n += 1
        self.ws.send(json.dumps({"id": self.n, "method": method, "params": params or {}}))
        while True:
            data = json.loads(self.ws.recv())
            if data.get("id") != self.n:
                continue
            if "error" in data:
                raise RuntimeError(f"{method}: {data['error']}")
            return data.get("result") or {}

    def eval(self, expression: str):
        result = self.call(
            "Runtime.evaluate",
            {"expression": expression, "returnByValue": True, "awaitPromise": True},
        )
        if result.get("exceptionDetails"):
            raise RuntimeError(result["exceptionDetails"])
        return (result.get("result") or {}).get("value")


def login_token() -> str:
    req = urllib.request.Request(
        API,
        data=json.dumps({"username": "admin", "password": "replacearr"}).encode(),
        headers={"Content-Type": "application/json"},
    )
    with urllib.request.urlopen(req, timeout=10) as resp:
        body = json.load(resp)
    token = body.get("token") or ""
    if not token:
        raise SystemExit("login did not return a token")
    return token


def wait_port(port: int):
    for _ in range(80):
        try:
            urllib.request.urlopen(f"http://127.0.0.1:{port}/json/version", timeout=1)
            return
        except Exception:
            time.sleep(0.1)
    raise SystemExit(f"chrome debug port {port} did not open")


def open_tab(port: int) -> str:
    req = urllib.request.Request(f"http://127.0.0.1:{port}/json/new?about:blank", method="PUT")
    try:
        with urllib.request.urlopen(req, timeout=5) as resp:
            return json.load(resp)["webSocketDebuggerUrl"]
    except Exception:
        pass
    with urllib.request.urlopen(f"http://127.0.0.1:{port}/json", timeout=5) as resp:
        for tab in json.load(resp):
            if tab.get("webSocketDebuggerUrl"):
                return tab["webSocketDebuggerUrl"]
    raise SystemExit("no debuggable tab")


def wait_text(cdp: CDP, needle: str, timeout=15):
    deadline = time.time() + timeout
    while time.time() < deadline:
        text = cdp.eval("document.body ? document.body.innerText.slice(0, 8000) : ''") or ""
        if needle in text:
            return
        time.sleep(0.15)
    raise SystemExit(f"timed out waiting for {needle!r}")


def open_mobile_nav(cdp: CDP):
    cdp.eval(
        """
        (() => {
          const open = document.querySelector('button[aria-label="Open navigation"]');
          if (open) { open.click(); return 'opened'; }
          return 'visible';
        })()
        """
    )
    time.sleep(0.25)


def click_label(cdp: CDP, label: str, kind: str):
    prefer_last = kind == "settings"
    expr = """
    (() => {
      const label = %s;
      const preferLast = %s;
      const buttons = [...document.querySelectorAll('button')];
      const exact = buttons.filter((b) => b.textContent.replace(/\\s+/g, ' ').trim() === label);
      const starts = buttons.filter((b) => b.textContent.replace(/\\s+/g, ' ').trim().startsWith(label));
      const pool = exact.length ? exact : starts;
      if (!pool.length) return 'missing';
      const btn = preferLast ? pool[pool.length - 1] : pool[0];
      btn.scrollIntoView({block: 'center', inline: 'center'});
      btn.click();
      return 'ok';
    })()
    """ % (json.dumps(label), "true" if prefer_last else "false")
    if cdp.eval(expr) != "ok":
        raise SystemExit(f"could not click {label!r} (kind={kind})")


def shoot(cdp: CDP, out_dir: Path, name: str):
    cdp.eval(REDACT)
    time.sleep(0.2)
    result = cdp.call("Page.captureScreenshot", {"format": "png", "fromSurface": True})
    path = out_dir / name
    path.write_bytes(base64.b64decode(result["data"]))
    print(f"wrote {path.relative_to(REPO)} ({path.stat().st_size} bytes)")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--profile", choices=("desktop", "mobile"), required=True)
    parser.add_argument("--port", type=int, default=9334)
    args = parser.parse_args()

    if args.profile == "desktop":
        width, height = 1440, 900
        out_dir = REPO / "images" / "desktop"
    else:
        width, height = 390, 844
        out_dir = REPO / "images" / "mobile"

    out_dir.mkdir(parents=True, exist_ok=True)
    token = login_token()

    # Drop stale Connect captures replaced by System.
    for stale in ("17-settings-connect.png",):
        old = out_dir / stale
        if old.exists():
            old.unlink()
            print(f"removed {old.relative_to(REPO)}")

    chrome = subprocess.Popen(
        [
            "google-chrome",
            "--headless=new",
            "--disable-gpu",
            "--no-sandbox",
            "--hide-scrollbars",
            f"--remote-debugging-port={args.port}",
            "--remote-allow-origins=*",
            f"--window-size={width},{height}",
            "--force-device-scale-factor=1",
            f"--user-data-dir=/tmp/chrome-shot-{args.profile}-{args.port}",
            "about:blank",
        ],
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    )
    try:
        wait_port(args.port)
        cdp = CDP(open_tab(args.port))
        cdp.call("Page.enable")
        cdp.call("Runtime.enable")
        cdp.call(
            "Emulation.setDeviceMetricsOverride",
            {
                "width": width,
                "height": height,
                "deviceScaleFactor": 1,
                "mobile": args.profile == "mobile",
            },
        )
        cdp.call("Page.navigate", {"url": WEB})
        wait_text(cdp, "Sign In")
        shoot(cdp, out_dir, "00-login.png")

        cdp.eval("localStorage.setItem('replacearr_token', %s); 'set'" % json.dumps(token))
        cdp.call("Page.reload", {"ignoreCache": True})
        wait_text(cdp, "Welcome to Replacearr")
        time.sleep(0.5)

        for filename, nav_label, needle, kind in PAGES:
            if filename == "00-login.png":
                continue
            if nav_label:
                if kind == "main" and args.profile == "mobile":
                    open_mobile_nav(cdp)
                click_label(cdp, nav_label, kind)
            wait_text(cdp, needle)
            # Compatibility gallery shot: scroll to *arr shim cards with client base URLs
            # so it differs from the top-of-page plugins overview.
            if filename == "20-compatibility.png":
                cdp.eval(
                    """
                    (() => {
                      const hit = [...document.querySelectorAll('h3')].find((el) =>
                        /Sonarr|Radarr|Prowlarr/i.test(el.textContent || '')
                      );
                      (hit || [...document.querySelectorAll('div')].find((el) =>
                        (el.textContent || '').includes('Client base URL')
                      ))?.scrollIntoView({ block: 'start' });
                      return 'scrolled';
                    })()
                    """
                )
                time.sleep(0.35)
            time.sleep(0.45)
            shoot(cdp, out_dir, filename)
    finally:
        chrome.terminate()
        try:
            chrome.wait(timeout=5)
        except subprocess.TimeoutExpired:
            chrome.kill()


if __name__ == "__main__":
    main()
