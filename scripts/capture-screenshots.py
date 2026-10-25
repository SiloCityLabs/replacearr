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

PAGES = [
    ("00-login.png", None, "Sign In"),
    ("01-dashboard.png", "Dashboard", "Welcome to Replacearr"),
    ("02-series.png", "TV Series (Sonarr)", "TV Series Library"),
    ("03-movies.png", "Movies (Radarr)", "Movies Library"),
    ("04-indexers-search.png", "Indexers & Search", "Indexers & Unified Search"),
    ("05-queue.png", "Queue & Clients", "Live Download Queue"),
    ("06-plugins.png", "Go Plugins", "Go Plugin System"),
    ("07-users.png", "Users & RBAC", "Users & Access Control"),
    ("08-settings.png", "Settings", "Episode and movie naming"),
    ("09-settings-general.png", "General", "Bind Address"),
    ("10-settings-ui.png", "UI", "First Day of Week"),
    ("11-settings-media.png", "Media Management", "Episode Naming"),
    ("12-settings-profiles.png", "Profiles", "Quality Profiles"),
    ("13-settings-quality.png", "Quality", "MB per minute"),
    ("14-settings-indexers.png", "Indexers", "Add Indexer"),
    ("15-settings-download-clients.png", "Download Clients", "Client records are live"),
    ("16-settings-import-lists.png", "Import Lists", "List sync options"),
    ("17-settings-connect.png", "Connect", "Saved connections are not sent"),
    ("18-settings-metadata.png", "Metadata", "Preferred language"),
    ("19-settings-tags.png", "Tags", "Add Tag"),
    ("20-compatibility.png", "Compatibility Hub", "Overseerr"),
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


def wait_text(cdp: CDP, needle: str, timeout=12):
    deadline = time.time() + timeout
    while time.time() < deadline:
        text = cdp.eval("document.body ? document.body.innerText.slice(0, 5000) : ''") or ""
        if needle in text:
            return
        time.sleep(0.15)
    raise SystemExit(f"timed out waiting for {needle!r}")


def click_label(cdp: CDP, label: str):
    expr = """
    (() => {
      const label = %s;
      const buttons = [...document.querySelectorAll('button')];
      let btn = buttons.find((b) => b.textContent.trim() === label);
      if (!btn) btn = buttons.find((b) => b.textContent.trim().startsWith(label));
      if (!btn) return 'missing';
      btn.scrollIntoView({block: 'center', inline: 'center'});
      btn.click();
      return 'ok';
    })()
    """ % json.dumps(label)
    if cdp.eval(expr) != "ok":
        raise SystemExit(f"could not click {label!r}")


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
            f"--user-data-dir=/tmp/chrome-shot-{args.profile}",
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

        for filename, nav_label, needle in PAGES:
            if filename == "00-login.png":
                continue
            if nav_label:
                click_label(cdp, nav_label)
            wait_text(cdp, needle)
            time.sleep(0.4)
            shoot(cdp, out_dir, filename)
    finally:
        chrome.terminate()
        try:
            chrome.wait(timeout=5)
        except subprocess.TimeoutExpired:
            chrome.kill()


if __name__ == "__main__":
    main()
