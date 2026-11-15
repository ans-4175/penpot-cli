#!/usr/bin/env python3
"""Bangun assets.json (ikon lucide) untuk lib.js.

  python3 make_assets.py [--icons a,b,c] > /tmp/li/assets.json

Pakai di exec (lihat Workflow J):
  const ASSETS = <isi assets.json>; storage.liAssets = JSON.stringify(ASSETS);
SVG lucide dibersihkan dari komentar <!-- --> (sandbox Penpot menolaknya:
SES_HTML_COMMENT_REJECTED).
"""
import argparse, json, os, re, sys
sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), ".."))
import pen2penpot as P

DEFAULT_ICONS = "palette,component,layout-template,book-open,monitor,smartphone,mail,users,user,building-2,git-branch,shield,hand,map-pin,arrow-right,arrow-down"

def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--icons", default=DEFAULT_ICONS)
    a = ap.parse_args()
    icons = {}
    for n in a.icons.split(","):
        s = P.lucide_svg(n.strip())
        if s:
            icons[n.strip()] = re.sub(r"<!--.*?-->", "", re.sub(r"\s+", " ", s), flags=re.S).strip()
    json.dump({"icons": icons}, sys.stdout)

if __name__ == "__main__":
    main()
