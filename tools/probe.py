#!/usr/bin/env python3
"""Probe captured frames: report measured pixel values instead of impressions.

Usage: probe.py image.png [x:y label ...]     coordinates are percentages 0-100
"""
import sys
from collections import Counter

from PIL import Image


def main():
    path = sys.argv[1]
    im = Image.open(path).convert("RGB")
    w, h = im.size
    px = im.load()
    print(f"{path}  {w}x{h}")

    # Coarse census: the 8x8 grid of most-saturated and brightest cells tells us
    # where colour actually landed.
    print("\nbrightness / dominant colour by region (rows top->bottom):")
    for gy in range(6):
        row = []
        for gx in range(8):
            x = int((gx + 0.5) * w / 8)
            y = int((gy + 0.5) * h / 6)
            r, g, b = px[x, y]
            lum = 0.2126 * r + 0.7152 * g + 0.0722 * b
            mx = max(r, g, b)
            name = "-"
            if mx > 24:
                if mx - min(r, g, b) > 30:
                    name = "RGB"[(r + g + b) // 3 % 3] if False else ("R" if r == mx else "G" if g == mx else "B")
                else:
                    name = "n"
            row.append(f"{lum:4.0f}{name}")
        print("  " + " ".join(row))

    hist = Counter(px[x, y] for x in range(0, w, 7) for y in range(0, h, 7))
    print("\ntop colours (rgb: count, share):")
    total = sum(hist.values())
    for col, n in hist.most_common(12):
        print(f"  {col}: {n:6d} {100.0*n/total:5.1f}%")

    if len(sys.argv) > 2:
        print("\nnamed samples:")
        for spec in sys.argv[2:]:
            parts = spec.split(":")
            x = int(float(parts[0]) / 100 * w)
            y = int(float(parts[1]) / 100 * h)
            label = parts[2] if len(parts) > 2 else ""
            print(f"  {label:22s} ({x:4d},{y:4d}) = {px[x, y]}")

    # Where is the brightest pixel, and how saturated is the most saturated pixel?
    best = max(((0.2126 * r + 0.7152 * g + 0.0722 * b), c, (x, y))
               for x in range(0, w, 3) for y in range(0, h, 3) for c in [px[x, y]])
    print(f"\nbrightest sampled pixel: {best[1]} at {best[2]} (luma {best[0]:.0f})")


main()
