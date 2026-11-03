# Replacearr logo assets

Source artwork for the Replacearr brand mark and PWA icons.

| File | Purpose |
| --- | --- |
| `replacearr-mark.svg` | Master vector logo (512×512 viewBox). Edit this first. |
| `replacearr-icon-192.png` | Exported 192×192 PNG for reference and regeneration. |
| `replacearr-icon-512.png` | Exported 512×512 PNG for reference and regeneration. |

Runtime copies used by the web app live in `web/public/` (`icon-192.png`, `icon-512.png`). After changing the SVG, re-export PNGs (ImageMagick, `rsvg-convert`, or similar) and copy them into `web/public/`.
