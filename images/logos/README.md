# Replacearr logo assets

Source artwork for the Replacearr brand mark and PWA icons.

| File | Purpose |
| --- | --- |
| `replacearr-mark.svg` | Master vector logo (512×512 viewBox). Edit this first. |
| `replacearr-icon-192.png` | Exported 192×192 PNG for reference and regeneration. |
| `replacearr-icon-512.png` | Exported 512×512 PNG for reference and regeneration. |
| `favicon.ico` | Multi-size ICO (16 / 32 / 48) for browsers that still want `.ico`. |

Runtime copies used by the web app live in `web/public/` (`favicon.ico`, `icon-192.png`, `icon-512.png`). After changing the SVG, re-export with a real SVG engine and copy into `web/public/`:

```bash
# preferred (librsvg)
rsvg-convert -w 512 -h 512 images/logos/replacearr-mark.svg -o images/logos/replacearr-icon-512.png
rsvg-convert -w 192 -h 192 images/logos/replacearr-mark.svg -o images/logos/replacearr-icon-192.png

# or Node sharp (also uses librsvg)
node -e "const s=require('sharp'); const src='images/logos/replacearr-mark.svg'; Promise.all([s(src).resize(512,512).png().toFile('images/logos/replacearr-icon-512.png'), s(src).resize(192,192).png().toFile('images/logos/replacearr-icon-192.png')])"

# favicon.ico from PNG sizes (ImageMagick packing PNGs is fine; do not feed it the SVG)
# with sharp: write 16/32/48 PNGs, then:
convert /tmp/icon-16.png /tmp/icon-32.png /tmp/icon-48.png images/logos/favicon.ico

cp images/logos/favicon.ico web/public/favicon.ico
cp images/logos/replacearr-icon-192.png web/public/icon-192.png
cp images/logos/replacearr-icon-512.png web/public/icon-512.png
```

Do not use ImageMagick’s built-in `MSVG` renderer (`convert file.svg …`): it drops gradients, filters, and most of the mark, leaving only the tiny play circle on black. Packing already-rasterized PNGs into `.ico` is fine.
