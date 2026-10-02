#!/usr/bin/env python3
"""Median colour of each sticker family, found by hue rather than by guessed
screen coordinates, so the numbers survive any change of framing."""
import sys, numpy as np
from PIL import Image
a = np.asarray(Image.open(sys.argv[1]).convert("RGB"), dtype=int)
R, G, B = a[:,:,0], a[:,:,1], a[:,:,2]
fams = {
    "white  245,245,245": (R > 150) & (G > 150) & (B > 150),
    "yellow 255,204,  0": (R > 120) & (G > 90) & (B < 90) & (np.abs(R - G) < 90),
    "red    204, 24, 24": (R > 70) & (R > 2.2 * G) & (R > 2.2 * B),
    "orange 255,138,  0": (R > 120) & (G > 60) & (G < 0.75 * R) & (B < 80),
    "green    0,150, 70": (G > 70) & (G > 1.6 * R) & (G > 1.2 * B),
    "blue     0, 74,200": (B > 70) & (B > 1.6 * G) & (B > 1.6 * R),
}
for name, m in fams.items():
    n = int(m.sum())
    if n < 40:
        print(f"  {name}   absent ({n} px)")
        continue
    med = np.median(a[m], axis=0).astype(int)
    lo, hi = np.percentile(a[m][:, 0], [5, 95])
    print(f"  {name}  median {tuple(med)}  range {int(lo)}-{int(hi)}  {100.0*n/m.size:5.2f}% of frame")
