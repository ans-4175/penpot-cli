#!/usr/bin/env python3
"""pen2penpot — porting node .pen (pen.dev) ke Penpot lewat penpot-cli.

Membaca node dari file .pen (JSON) lalu menghasilkan JavaScript Penpot Plugin
API yang membangun ulang subtree-nya. Dijalankan dengan `penpot-cli exec -f`.

    pen2penpot.py FILE.pen NODE_ID [--page "DS — Design System"] [--x X --y Y]
                  [--lib DS.lib.pen] [--components] [--emit out.js] [--run]

Pemetaan:
  frame + layout vertical/horizontal → board + flex (column/row)
  frame layout none                  → board tanpa layout, anak absolut (x/y relatif)
  width/height fill_container        → layoutChild.*Sizing = 'fill'
  width/height tidak diset (frame)   → 'auto' (fit content)
  text textGrowth fixed-width        → growType auto-height + lebar
  text auto                          → growType auto-width
  fill color/#RRGGBBAA/gradient/image → fills (image: storage.media[url], lihat upload --base)
  stroke + strokeWidth               → strokes (inner, kecuali strokeAlignment)
  effect shadow                      → shadows (drop/inner)
  rotation                           → dibalik tandanya (pen CCW, Penpot CW)
  icon lucide                        → createShapeFromSvg (SVG lucide-static, di-cache)
  reusable: true                     → komponen local library (--components)
  ref                                → instance komponen (penpot-cli.map di plugin data
                                       file: penId → componentId) + override descendants
  $variable                          → nilai variabel .pen (diresolve saat porting)

Peta komponen disimpan di plugin data file ('penpot-cli.pen-map'), sehingga
port berikutnya (story/carousel) bisa meng-instance komponen DS yang sudah
diport sebelumnya.
"""
from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import sys
import urllib.request
from pathlib import Path

CACHE = Path.home() / ".cache" / "penpot-cli" / "lucide"


# ---------------------------------------------------------------- .pen model

class PenDoc:
    def __init__(self, path: Path, libs: list[Path]):
        self.path = path
        self.base = path.parent
        self.doc = json.loads(path.read_text())
        self.synthetic: dict[str, dict] = {}
        self.nodes: dict[str, dict] = {}
        self.parent: dict[str, dict] = {}
        self.vars: dict[str, object] = {}
        self._index(self.doc, "")
        self.imports = self.doc.get("imports") or {}
        for alias, rel in self.imports.items():
            p = self.base / rel
            if p.exists():
                lib = json.loads(p.read_text())
                self._index(lib, alias + ":")
        for p in libs:
            lib = json.loads(p.read_text())
            self._index(lib, "")

    def _index(self, doc: dict, prefix: str):
        for k, v in (doc.get("variables") or {}).items():
            val = v.get("value")
            if isinstance(val, list):  # themed → ambil entri pertama
                val = val[0].get("value") if val else None
            self.vars.setdefault(k, val)
        def walk(n):
            if n.get("id"):
                self.nodes[prefix + n["id"]] = n
                if prefix and n["id"] not in self.nodes:
                    self.nodes[n["id"]] = n
            for c in n.get("children") or []:
                if c.get("id"):
                    self.parent.setdefault(c["id"], n)
                walk(c)
        for c in doc.get("children") or []:
            walk(c)

    def resolve(self, v):
        seen = 0
        while isinstance(v, str) and v.startswith("$") and seen < 8:
            v = self.vars.get(v[1:], v)
            seen += 1
        return v


# ---------------------------------------------------------------- helpers

def hexa(c: str):
    """#RGB/#RRGGBB/#RRGGBBAA → (#RRGGBB uppercase, opacity)."""
    c = c.strip()
    if not c.startswith("#"):
        return c.upper(), 1.0
    h = c[1:]
    if len(h) == 3:
        h = "".join(x * 2 for x in h)
    a = 1.0
    if len(h) == 8:
        a = round(int(h[6:8], 16) / 255, 3)
        h = h[:6]
    return "#" + h.upper(), a


def lucide_svg(name: str) -> str | None:
    CACHE.mkdir(parents=True, exist_ok=True)
    f = CACHE / f"{name}.svg"
    if not f.exists():
        try:
            url = f"https://unpkg.com/lucide-static@latest/icons/{name}.svg"
            req = urllib.request.Request(url, headers={"User-Agent": "pen2penpot"})
            f.write_bytes(urllib.request.urlopen(req, timeout=20).read())
        except Exception as e:  # noqa: BLE001
            print(f"[warn] ikon lucide {name!r} gagal diunduh: {e}", file=sys.stderr)
            return None
    return f.read_text()


class Emitter:
    """Membangun JS. Setiap node menjadi variabel n<i>."""

    def __init__(self, pen: PenDoc, components: bool, shallow: bool = False):
        self.pen = pen
        self.components = components
        self.shallow = shallow
        self.lines: list[str] = []
        self.i = 0
        self.warn: list[str] = []
        self.media: set[str] = set()
        self.comp_roots: list[tuple[str, str]] = []  # (var, penId)
        self.deferred: list[str] = []  # rotasi: diterapkan setelah layout selesai

    def v(self):
        self.i += 1
        return f"n{self.i}"

    def js(self, s: str):
        self.lines.append(s)

    # -- properti umum

    def fills(self, var, fill, is_text=False):
        if fill is None:
            if not is_text:
                self.js(f"{var}.fills = [];")
            return
        items = fill if isinstance(fill, list) else [fill]
        out = []
        for f in items:
            f = self.pen.resolve(f)
            if isinstance(f, str):
                col, a = hexa(f)
                out.append(f"{{fillColor:{json.dumps(col)},fillOpacity:{a}}}")
            elif isinstance(f, dict):
                if f.get("enabled") is False:
                    continue
                t = f.get("type", "color")
                if t == "color":
                    col, a = hexa(self.pen.resolve(f.get("color", "#000000")))
                    out.append(f"{{fillColor:{json.dumps(col)},fillOpacity:{a}}}")
                elif t == "image":
                    url = f.get("url")
                    self.media.add(url)
                    op = f.get("opacity", 1)
                    out.append(f"__img({json.dumps(url)},{op})")
                elif t == "gradient":
                    out.append(self.gradient(f))
                else:
                    self.warn.append(f"fill {t} tidak didukung")
        self.js(f"{var}.fills = [{','.join(x for x in out if x)}].filter(Boolean);")

    def gradient(self, f):
        # pen: rotation derajat CCW, 0 = ke atas. Penpot: start/end ternormalisasi.
        import math
        rot = float(f.get("rotation", 0) or 0)
        rad = math.radians(rot)
        dx, dy = -math.sin(rad) * 0.5, -math.cos(rad) * 0.5
        sx, sy, ex, ey = 0.5 - dx, 0.5 - dy, 0.5 + dx, 0.5 + dy
        stops = []
        for c in f.get("colors") or []:
            col, a = hexa(self.pen.resolve(c["color"]))
            stops.append(f"{{color:{json.dumps(col)},opacity:{a},offset:{c.get('position', 0)}}}")
        kind = "radial" if f.get("gradientType") == "radial" else "linear"
        return (f"{{fillOpacity:{f.get('opacity', 1)},fillColorGradient:{{type:'{kind}',startX:{sx:.3f},startY:{sy:.3f},"
                f"endX:{ex:.3f},endY:{ey:.3f},width:1,stops:[{','.join(stops)}]}}}}")

    def strokes(self, var, n):
        st = n.get("stroke")
        if not st:
            return
        sw = n.get("strokeWidth", 1)
        if isinstance(sw, dict):
            self.warn.append(f"{n.get('id')}: strokeWidth per sisi → pakai nilai maks")
            sw = max(v for v in sw.values() if isinstance(v, (int, float)))
        col, a = hexa(self.pen.resolve(st if isinstance(st, str) else st.get("color", "#000000")))
        align = {"outer": "outer", "center": "center"}.get(n.get("strokeAlignment"), "inner")
        self.js(f"{var}.strokes = [{{strokeColor:{json.dumps(col)},strokeOpacity:{a},strokeWidth:{sw},strokeStyle:'solid',strokeAlignment:'{align}'}}];")

    def shadows(self, var, n):
        eff = n.get("effect")
        if not eff:
            return
        out = []
        for e in eff if isinstance(eff, list) else [eff]:
            if e.get("type") != "shadow" or e.get("enabled") is False:
                if e.get("type") in ("blur", "background_blur"):
                    self.js(f"{var}.blur = {{type:'layer-blur',value:{e.get('radius', 4)},hidden:false}};")
                continue
            col, a = hexa(self.pen.resolve(e.get("color", "#00000040")))
            off = e.get("offset") or {}
            style = "inner-shadow" if e.get("shadowType") == "inner" else "drop-shadow"
            out.append(f"{{style:'{style}',offsetX:{off.get('x', 0)},offsetY:{off.get('y', 0)},blur:{e.get('blur', 0)},"
                       f"spread:{e.get('spread', 0)},hidden:false,color:{{color:{json.dumps(col)},opacity:{a}}}}}")
        if out:
            self.js(f"{var}.shadows = [{','.join(out)}];")

    def radius(self, var, n):
        r = self.pen.resolve(n.get("cornerRadius"))
        if r is None:
            return
        if isinstance(r, list):
            r = [self.pen.resolve(x) for x in r]
            self.js(f"{var}.borderRadiusTopLeft={r[0]};{var}.borderRadiusTopRight={r[1]};{var}.borderRadiusBottomRight={r[2]};{var}.borderRadiusBottomLeft={r[3]};")
        else:
            self.js(f"{var}.borderRadius = {r};")

    def common(self, var, n):
        if n.get("opacity") is not None and n.get("opacity") != 1:
            self.js(f"{var}.opacity = {self.pen.resolve(n['opacity'])};")
        if n.get("enabled") is False:
            self.js(f"{var}.hidden = true;")

    # -- penempatan dalam parent

    def place(self, var, n, parent_var, parent_layout, w_spec, h_spec):
        """Dipanggil SETELAH appendChild. Mengatur sizing di layout / posisi absolut."""
        in_layout = parent_layout in ("vertical", "horizontal")
        absolute = n.get("layoutPosition") == "absolute"
        if in_layout and not absolute:
            hs = "fill" if w_spec == "fill" else ("auto" if w_spec == "auto" else "fix")
            vs = "fill" if h_spec == "fill" else ("auto" if h_spec == "auto" else "fix")
            self.js(f"if ({var}.layoutChild) {{ {var}.layoutChild.horizontalSizing='{hs}'; {var}.layoutChild.verticalSizing='{vs}'; }}")
        else:
            if in_layout and absolute:
                self.js(f"if ({var}.layoutChild) {var}.layoutChild.absolute = true;")
            x, y = n.get("x", 0) or 0, n.get("y", 0) or 0
            self.js(f"penpotUtils.setParentXY({var},{x},{y});")
        rot = n.get("rotation")
        if rot:
            # Penpot: rotation = sudut ABSOLUT dan anak ikut berputar bersama
            # induk. Diterapkan top-down (urutan deferred = urutan traversal,
            # induk dulu): induk diputar → anak sudah ikut miring → tambahkan
            # rotasi relatifnya sendiri.
            # Board flex 'auto' yang diputar: Penpot menghitung ulang ukurannya
            # dari bounding box anak yang ikut miring → membesar terus tiap
            # reflow (586×533 dari 430×177, exec jadi lambat/timeout). Kunci
            # ukuran (auto → fix) dulu, baru putar.
            self.deferred.append(f"await __rotate({var}, {-float(rot)});")

    @staticmethod
    def size_spec(val):
        if isinstance(val, (int, float)):
            return "fix"
        if isinstance(val, str):
            if val.startswith("fill_container"):
                return "fill"
            if val.startswith("fit_content"):
                return "auto"
        return None

    @staticmethod
    def fallback(val, default):
        if isinstance(val, (int, float)):
            return val
        if isinstance(val, str):
            m = re.search(r"\(([\d.]+)\)", val)
            if m:
                return float(m.group(1))
        return default

    # -- node

    def node_raw(self, n, parent_var, parent_layout, root=False):
        t = n.get("type")
        if n.get("enabled") is False and not root and t != "ref":
            return None  # disabled → lewati (guide overlay dll)
        handler = getattr(self, "n_" + t, None)
        if not handler:
            self.warn.append(f"type {t} ({n.get('id')}) dilewati")
            return None
        return handler(n, parent_var, parent_layout, root)

    def name(self, var, n):
        self.js(f"{var}.name = {json.dumps(n.get('name') or n.get('type'))};")
        if n.get("id"):
            self.js(f"{var}.setPluginData('penId', {json.dumps(n['id'])});")

    def n_frame(self, n, parent_var, parent_layout, root):
        var = self.v()
        layout = n.get("layout", "horizontal" if n.get("children") is not None or True else "none")
        if "layout" not in n:
            layout = "horizontal"
        w, h = n.get("width"), n.get("height")
        ws, hs = self.size_spec(w), self.size_spec(h)
        if layout == "none":
            ws, hs = ws or "fix", hs or "fix"
        else:
            ws, hs = ws or "auto", hs or "auto"
        self.js(f"const {var} = penpot.createBoard();")
        self.name(var, n)
        self.js(f"{var}.clipContent = {'true' if n.get('clip') else 'false'};")
        self.js(f"{parent_var}.appendChild({var});")
        # ukuran awal sebelum layout: fill → ikut ukuran parent supaya flex
        # tidak terkunci di nilai sementara; auto → kecil (tumbuh ikut isi)
        # PENTING: board flex 'auto' tidak menyusut di bawah ukuran resize terakhir
        # → sumbu auto diawali 1px, sumbu fill diawali ukuran parent.
        # fill di sumbu UTAMA parent → mulai 1px (beberapa anak fill berbagi ruang);
        # fill di sumbu SILANG → ukuran parent.
        main_w = parent_layout == "horizontal"
        main_h = parent_layout == "vertical"
        def init(spec, val, pdim, is_main):
            if spec == "fill" and not root:
                return 1 if is_main else f"{parent_var}.{pdim}"
            if spec == "auto":
                return 1
            return self.fallback(val, 10)
        iw, ih = init(ws, w, "width", main_w), init(hs, h, "height", main_h)
        self.js(f"{var}.resize({iw},{ih});")
        if layout in ("vertical", "horizontal"):
            fl = var + "f"
            self.js(f"const {fl} = {var}.addFlexLayout();")
            self.js(f"{fl}.dir = '{'column' if layout == 'vertical' else 'row'}';")
            gap = self.pen.resolve(n.get("gap", 0)) or 0
            self.js(f"{fl}.rowGap = {gap if layout == 'vertical' else 0}; {fl}.columnGap = {gap if layout == 'horizontal' else 0};")
            ai = {"center": "center", "end": "end", "start": "start"}.get(n.get("alignItems"), "start")
            jc = {"center": "center", "end": "end", "space_between": "space-between", "space_around": "space-around"}.get(n.get("justifyContent"), "start")
            self.js(f"{fl}.alignItems = '{ai}'; {fl}.justifyContent = '{jc}';")
            p = n.get("padding", 0)
            p = [self.pen.resolve(x) for x in p] if isinstance(p, list) else [self.pen.resolve(p)]
            if len(p) == 1:
                p = p * 4
            elif len(p) == 2:
                p = [p[0], p[1], p[0], p[1]]
            self.js(f"{fl}.topPadding={p[0]};{fl}.rightPadding={p[1]};{fl}.bottomPadding={p[2]};{fl}.leftPadding={p[3]};")
            self.js(f"{fl}.horizontalSizing = '{'auto' if ws == 'auto' else 'fix'}'; {fl}.verticalSizing = '{'auto' if hs == 'auto' else 'fix'}';")
        self.fills(var, n.get("fill"))
        self.strokes(var, n)
        self.shadows(var, n)
        self.radius(var, n)
        self.common(var, n)
        if not root:
            self.place(var, n, parent_var, parent_layout, ws, hs)
        if not (root and self.shallow):
            for c in n.get("children") or []:
                self.node(c, var, layout)
        self.make_component(var, n)
        return var

    def make_component(self, var, n):
        if n.get("reusable") and self.components and n.get("type") == "frame":
            self.comp_roots.append((var, n["id"]))
            self.js(f"__mkComp({var}, {json.dumps(n['id'])});")

    def node(self, n, parent_var, parent_layout, root=False):
        """Node reusable non-frame (text/rect/ellipse) dibungkus board auto-size
        SAAT dibuat, supaya urutan flex terjaga dan komponennya pas dengan isi."""
        if (n.get("reusable") and self.components and n.get("type") not in ("frame", "ref", "group")
                and not (n.get("enabled") is False and not root)):
            inner = {k: v for k, v in n.items() if k not in ("reusable", "x", "y", "rotation", "layoutPosition")}
            inner["name"] = "Text" if n.get("type") == "text" else (n.get("type") or "shape").title()
            inner["id"] = n["id"] + "~inner"
            grow = n.get("textGrowth", "auto")
            w_fixed = n.get("type") == "text" and grow != "auto" and isinstance(n.get("width"), (int, float))
            wrapper = {"type": "frame", "id": n["id"], "name": n.get("name"), "reusable": True,
                       "layout": "vertical", "children": [inner]}
            for k in ("x", "y", "rotation", "layoutPosition"):
                if k in n:
                    wrapper[k] = n[k]
            if n.get("type") != "text":
                # shape (rect/ellipse): wrapper berukuran tetap, shape mengisi penuh →
                # instance yang di-resize ikut menskalakan shape-nya.
                wrapper["width"], wrapper["height"] = n.get("width", 100), n.get("height", 100)
                inner["width"] = inner["height"] = "fill_container"
            elif w_fixed:
                wrapper["width"] = n["width"]
                inner["width"] = "fill_container"
            elif isinstance(n.get("width"), str) and n["width"].startswith("fill"):
                wrapper["width"] = n["width"]
                inner["width"] = "fill_container"
            return self.node_raw(wrapper, parent_var, parent_layout, root)
        return self.node_raw(n, parent_var, parent_layout, root)

    def n_group(self, n, parent_var, parent_layout, root):
        n2 = dict(n, type="frame", layout="none")
        return self.n_frame(n2, parent_var, parent_layout, root)

    def n_text(self, n, parent_var, parent_layout, root):
        var = self.v()
        content = self.pen.resolve(n.get("content", "")) or " "
        self.js(f"const {var} = penpot.createText({json.dumps(content)});")
        self.name(var, n)
        fam = self.pen.resolve(n.get("fontFamily", "Inter")) or "Inter"
        wt = str(self.pen.resolve(n.get("fontWeight", "400")) or "400")
        wt = {"normal": "400", "bold": "700"}.get(wt, wt)
        italic = "true" if n.get("fontStyle") == "italic" else "false"
        self.js(f"__font({var},{json.dumps(fam)},{json.dumps(wt)},{italic});")
        self.js(f"{var}.fontSize = '{self.pen.resolve(n.get('fontSize', 14))}';")
        if n.get("lineHeight") is not None:
            self.js(f"{var}.lineHeight = '{self.pen.resolve(n['lineHeight'])}';")
        if n.get("letterSpacing") is not None:
            self.js(f"{var}.letterSpacing = '{self.pen.resolve(n['letterSpacing'])}';")
        if n.get("textAlign"):
            self.js(f"{var}.align = '{n['textAlign']}';")
        if n.get("textAlignVertical"):
            self.js(f"{var}.verticalAlign = '{ {'middle': 'center'}.get(n['textAlignVertical'], n['textAlignVertical']) }';")
        if n.get("underline"):
            self.js(f"{var}.textDecoration = 'underline';")
        if n.get("strikethrough"):
            self.js(f"{var}.textDecoration = 'line-through';")
        self.fills(var, n.get("fill"), is_text=True)
        self.shadows(var, n)
        self.common(var, n)
        self.js(f"{parent_var}.appendChild({var});")
        grow = n.get("textGrowth", "auto")
        w, h = n.get("width"), n.get("height")
        ws = self.size_spec(w)
        if grow == "auto":
            self.js(f"{var}.growType = 'auto-width';")
            ws, hs = "auto", "auto"
        elif grow == "fixed-width":
            # fill → lebar awal = lebar parent dikurangi padding (bukan 200: teks
            # sempat membungkus sempit dan tingginya tidak menyusut lagi)
            iw = self.fallback(w, 200)
            if ws == "fill" and parent_layout in ("vertical", "horizontal"):
                iw = f"Math.max(1, {parent_var}.width - (({parent_var}.flex && ({parent_var}.flex.leftPadding + {parent_var}.flex.rightPadding)) || 0))"
            self.js(f"{var}.resize({iw},Math.max({var}.height,1)); {var}.growType = 'auto-height';")
            hs = "auto"
            ws = ws or "fix"
        else:
            self.js(f"{var}.resize({self.fallback(w, 200)},{self.fallback(h, 50)}); {var}.growType = 'fixed';")
            hs = "fix"
            ws = ws or "fix"
        self.place(var, n, parent_var, parent_layout, ws, hs)
        self.make_component(var, n)
        return var

    def _shape(self, n, parent_var, parent_layout, ctor):
        var = self.v()
        self.js(f"const {var} = penpot.{ctor}();")
        self.name(var, n)
        self.js(f"{parent_var}.appendChild({var});")
        w, h = n.get("width"), n.get("height")
        self.js(f"{var}.resize({self.fallback(w, 100)},{self.fallback(h, 100)});")
        self.fills(var, n.get("fill"))
        self.strokes(var, n)
        self.shadows(var, n)
        self.radius(var, n)
        self.common(var, n)
        self.place(var, n, parent_var, parent_layout, self.size_spec(w) or "fix", self.size_spec(h) or "fix")
        self.make_component(var, n)
        return var

    def n_rectangle(self, n, p, pl, root):
        return self._shape(n, p, pl, "createRectangle")

    def n_ellipse(self, n, p, pl, root):
        if n.get("innerRadius") or n.get("sweepAngle") not in (None, 360):
            self.warn.append(f"{n.get('id')}: ellipse innerRadius/sweep → elips penuh")
        return self._shape(n, p, pl, "createEllipse")

    def n_path(self, n, p, pl, root):
        geo = n.get("geometry")
        vb = n.get("viewBox")
        if not geo:
            return None
        w, h = self.fallback(n.get("width"), 24), self.fallback(n.get("height"), 24)
        vb = vb or [0, 0, w, h]
        # fill dan/atau stroke (garis panah/konektor di .pen = stroke tanpa fill)
        attrs = []
        f = self.pen.resolve(n.get("fill"))
        if isinstance(f, str):
            col, a = hexa(f)
            attrs.append(f'fill="{col}"' + (f' fill-opacity="{a}"' if a < 1 else ""))
        else:
            attrs.append('fill="none"' if n.get("stroke") else 'fill="#000000"')
        s = self.pen.resolve(n.get("stroke"))
        if isinstance(s, str):
            col, a = hexa(s)
            sw = self.pen.resolve(n.get("strokeWidth", 1))
            attrs.append(f'stroke="{col}" stroke-width="{sw}"' + (f' stroke-opacity="{a}"' if a < 1 else ""))
            for k, svgk in (("strokeLinecap", "stroke-linecap"), ("strokeLinejoin", "stroke-linejoin")):
                if n.get(k):
                    attrs.append(f'{svgk}="{n[k]}"')
        svg = (f'<svg xmlns="http://www.w3.org/2000/svg" width="{w}" height="{h}" viewBox="{" ".join(map(str, vb))}" '
               f'preserveAspectRatio="none"><path d="{geo}" {" ".join(attrs)}/></svg>')
        return self._svg(n, p, pl, svg, w, h)

    def _svg(self, n, parent_var, parent_layout, svg, w, h):
        var = self.v()
        self.js(f"const {var} = penpot.createShapeFromSvg({json.dumps(svg)});")
        self.name(var, n)
        self.js(f"{parent_var}.appendChild({var}); {var}.resize({w},{h});")
        self.place(var, n, parent_var, parent_layout, "fix", "fix")
        return var

    def n_icon(self, n, parent_var, parent_layout, root):
        lib = n.get("library", "lucide")
        name = self.pen.resolve(n.get("icon"))
        w, h = self.fallback(n.get("width"), 24), self.fallback(n.get("height"), 24)
        col = hexa(self.pen.resolve(n.get("fill", "#000000")) if isinstance(self.pen.resolve(n.get("fill", "#000000")), str) else "#000000")[0]
        svg = lucide_svg(name) if lib in ("lucide", "feather") else None
        if not svg:
            self.warn.append(f"ikon {lib}/{name} → placeholder")
            return self._shape(dict(n, fill="#CBD5E1"), parent_var, parent_layout, "createRectangle")
        svg = re.sub(r'stroke="currentColor"', f'stroke="{col}"', svg)
        sw = round(2 * max(w, h) / 24, 2)
        svg = re.sub(r'stroke-width="[\d.]+"', f'stroke-width="{sw}"', svg)
        svg = re.sub(r"<!--.*?-->", "", svg, flags=re.S).strip()
        return self._svg(n, parent_var, parent_layout, svg, w, h)

    def n_ref(self, n, parent_var, parent_layout, root):
        ref = n["ref"]
        src = self.pen.nodes.get(ref) or self.pen.nodes.get(ref.split(":")[-1])
        var = self.v()
        key = ref.split(":")[-1]
        self.js(f"const {var} = __inst({json.dumps(key)});")
        self.js(f"if (!{var}) throw new Error('komponen untuk ref {key} belum diport (jalankan port DS dulu)');")
        self.js(f"{var}.name = {json.dumps(n.get('name') or (src or {}).get('name') or 'Instance')};")
        self.js(f"{var}.setPluginData('penId', {json.dumps(n['id'])});")
        self.js(f"{parent_var}.appendChild({var});")
        if n.get("fill") is not None:
            self.fills(var, n.get("fill"))
        w, h = n.get("width"), n.get("height")
        if isinstance(w, (int, float)) or isinstance(h, (int, float)):
            self.js(f"{var}.resize({w if isinstance(w, (int, float)) else var + '.width'},{h if isinstance(h, (int, float)) else var + '.height'});")
        sw = self.size_spec(w) or ("fix" if src is None else (self.size_spec(src.get("width")) or "auto"))
        sh = self.size_spec(h) or ("fix" if src is None else (self.size_spec(src.get("height")) or "auto"))
        # descendants: override per penId → cari shape anak dengan nama/urutan yang sama
        for dk, ov in (n.get("descendants") or {}).items():
            did = dk.split("/")[-1].split(":")[-1]
            dsrc = self.pen.nodes.get(did)
            dname = (dsrc or {}).get("name")
            dv = self.v()
            self.js(f"const {dv} = __desc({var}, {json.dumps(did)}, {json.dumps(dname)});")
            self.js(f"if ({dv}) {{")
            if "type" in ov:
                self.warn.append(f"{n.get('id')}: replace descendant {did} tidak didukung → dilewati")
            if "content" in ov:
                self.js(f"  {dv}.characters = {json.dumps(self.pen.resolve(ov['content']))};")
            if "fontSize" in ov:
                self.js(f"  {dv}.fontSize = '{ov['fontSize']}';")
            if "fontWeight" in ov:
                fam = self.pen.resolve((dsrc or {}).get("fontFamily", "Inter"))
                self.js(f"  __font({dv}, {json.dumps(fam)}, {json.dumps(str(ov['fontWeight']))}, false, true);")
            if "fill" in ov:
                self.fills(dv, ov["fill"], is_text=(dsrc or {}).get("type") == "text")
            if "enabled" in ov:
                self.js(f"  {dv}.hidden = {'false' if ov['enabled'] else 'true'};")
            if "width" in ov and isinstance(ov["width"], (int, float)):
                self.js(f"  {dv}.resize({ov['width']}, {dv}.height);")
            self.js("}")
        if n.get("enabled") is False:
            self.js(f"{var}.hidden = true;")
        self.place(var, n, parent_var, parent_layout, sw, sh)
        return var

    def n_note(self, n, p, pl, root):
        return None

    n_prompt = n_context = n_note


PRELUDE = r"""
const __mediaRaw = penpot.currentFile.getPluginData('penpot-cli.media');
const __mediaMan = __mediaRaw ? JSON.parse(__mediaRaw) : {};
storage.media = storage.media ?? {};
const __missing = [];
const __img = (url, op) => {
  const m = storage.media[url] ?? __mediaMan[url]?.img;
  if (!m) { __missing.push(url); return null; }
  return { fillOpacity: op ?? 1, fillImage: m };
};
const __fontCache = {};
const __font = (t, fam, wt, italic, keepFamily) => {
  // findByName = pencocokan sebagian ("Anton" → "Anton SC")! Cari nama persis dulu.
  const exact = n => penpot.fonts.findAllByName(n).find(x => x.name === n) || penpot.fonts.all.find(x => x.name === n);
  const f = __fontCache[fam] ?? (__fontCache[fam] = exact(fam) || penpot.fonts.findByName(fam) || exact('Inter'));
  if (!f) return;
  const vs = f.variants;
  const pick = vs.find(v => v.fontWeight === wt && (v.fontStyle === 'italic') === italic)
    || vs.find(v => v.fontWeight === wt) || vs.find(v => v.fontWeight === '400' && v.fontStyle !== 'italic') || vs[0];
  f.applyToText(t, pick);
};
const __made = [];
// Rotasi board flex 'auto': Penpot menghitung ulang ukuran dari bounding box
// anak yang ikut miring → membengkak tiap reflow (dan exec melambat sampai
// timeout). Tunggu teks anak selesai diukur, kunci ukuran (auto → fix) dari
// isi + padding, baru putar. Rotasi relatif (anak ikut induk).
const __rotate = async (b, deg) => {
  const fl = b.flex;
  if (fl && (fl.horizontalSizing === 'auto' || fl.verticalSizing === 'auto')) {
    const ks = () => b.children.filter(k => !k.hidden);
    for (let i = 0; i < 30 && ks().some(k => k.width <= 1.5 || k.height <= 1.5); i++) await new Promise(r => setTimeout(r, 50));
    const k = ks(), row = fl.dir === 'row' || fl.dir === 'row-reverse', g = row ? (fl.columnGap || 0) : (fl.rowGap || 0);
    let w = b.width, h = b.height;
    if (k.length) {
      const sw = row ? k.reduce((a, c) => a + c.width, 0) + g * (k.length - 1) : Math.max(...k.map(c => c.width));
      const sh = row ? Math.max(...k.map(c => c.height)) : k.reduce((a, c) => a + c.height, 0) + g * (k.length - 1);
      if (fl.horizontalSizing === 'auto') w = sw + (fl.leftPadding || 0) + (fl.rightPadding || 0);
      if (fl.verticalSizing === 'auto') h = sh + (fl.topPadding || 0) + (fl.bottomPadding || 0);
    }
    if (fl.horizontalSizing === 'auto') fl.horizontalSizing = 'fix';
    if (fl.verticalSizing === 'auto') fl.verticalSizing = 'fix';
    b.resize(w, h);
  }
  b.rotation = ((b.rotation || 0) + deg + 360) % 360;
};
// Penpot tidak selalu menghitung ulang board flex 'auto' setelah anak
// ditambahkan lewat API. Menyenggol rotasi (r+0.01 → r) memaksa reflow.
// Jalankan dari board terdalam ke luar, SEBELUM rotasi akhir diterapkan.
const __settle = async (root) => {
  const depth = s => { let d = 0, p = s.parent; while (p) { d++; p = p.parent; } return d; };
  const isAuto = b => b.flex && (b.flex.horizontalSizing === 'auto' || b.flex.verticalSizing === 'auto');
  const boards = [root, ...penpotUtils.findShapes(s => s.type === 'board' && isAuto(s), root)].filter(b => b && b.flex)
    .sort((a, b) => depth(b) - depth(a));
  for (let pass = 0; pass < 3; pass++) {
    const before = boards.map(b => b.width + 'x' + b.height).join('|');
    for (const b of boards) { const r = b.rotation || 0; b.rotation = r + 0.01; b.rotation = r; }
    await new Promise(r => setTimeout(r, 60));
    if (boards.map(b => b.width + 'x' + b.height).join('|') === before) break;
  }
};
const __mkComp = (target, penId) => {
  const nm = target.name;
  const c = penpot.library.local.createComponent([target]);
  c.name = nm.split(' / ').pop();
  if (nm.includes(' / ')) c.path = nm.split(' / ').slice(0, -1).join(' / ');
  target.name = nm;
  __map[penId] = c.id; __made.push(nm);
  return target;
};
const __mapRaw = penpot.currentFile.getPluginData('penpot-cli.pen-map');
const __map = __mapRaw ? JSON.parse(__mapRaw) : {};
const __inst = (penId) => {
  const cid = __map[penId];
  if (!cid) return null;
  const c = penpot.library.local.components.find(c => c.id === cid);
  return c ? c.instance() : null;
};
const __desc = (inst, penId, name) => {
  const all = penpotUtils.findShapes(s => true, inst);
  return all.find(s => s.getPluginData && s.getPluginData('penId') === penId)
    || (name ? all.find(s => s.name === name) : null);
};
"""


def build(pen: PenDoc, node_id: str, page: str | None, x, y, components: bool, into: str | None = None, shallow: bool = False, section_name: str | None = None):
    if "," in node_id:  # beberapa node → bungkus jadi satu section sintetis
        ids = node_id.split(",")
        kids = []
        for i in ids:
            k = pen.nodes.get(i) or pen.nodes.get(i.split(":")[-1])
            if not k:
                sys.exit(f"node {i} tidak ada")
            kids.append(k)
        n = {"type": "frame", "id": "sec-" + "-".join(x.split(":")[-1] for x in ids), "name": section_name or "Section",
             "width": "fill_container", "gap": 48, "alignItems": "start", "children": kids}
        pen.parent[n["id"]] = {"type": "frame", "layout": "vertical"}
    else:
        n = pen.nodes.get(node_id) or pen.nodes.get(node_id.split(":")[-1])
    if not n:
        sys.exit(f"node {node_id} tidak ada")
    em = Emitter(pen, components, shallow)
    em.js(PRELUDE)
    if page:
        em.js(f"""let __pg = penpotUtils.getPageByName({json.dumps(page)});
if (!__pg) {{ __pg = penpot.createPage(); __pg.name = {json.dumps(page)}; }}
if (penpot.currentPage.id !== __pg.id) await penpot.openPage(__pg);""")
    if into:
        em.js(f"const __root = penpotUtils.findShapeById({json.dumps(into)}); if (!__root) throw new Error('board --into tidak ditemukan');")
        pp = pen.parent.get(n["id"]) or {}
        playout = pp.get("layout", "horizontal") if pp.get("type") == "frame" else "none"
        root_var = em.node(n, "__root", playout, root=False)
    else:
        em.js("const __root = penpot.root;")
        root_var = em.node(n, "__root", "none", root=True)
    if root_var is None:
        sys.exit("root tidak terbangun")
    if into:
        pass
    elif x is None:
        em.js(f"""{{ const k = __root.children.filter(s => s.id !== {root_var}.id);
  {root_var}.x = k.length ? Math.round(Math.max(...k.map(s => s.x + s.width)) + 200) : 0;
  {root_var}.y = k.length ? Math.round(Math.min(...k.map(s => s.y))) : 0; }}""")
    else:
        em.js(f"{root_var}.x = {x}; {root_var}.y = {y};")
    em.js(f"await __settle({root_var});")
    for line in em.deferred:
        em.js(line)
    em.js("penpot.currentFile.setPluginData('penpot-cli.pen-map', JSON.stringify(__map));")
    em.js("await new Promise(r => setTimeout(r, 250));")
    em.js(f"return {{ root: {root_var}.id, name: {root_var}.name, x: {root_var}.x, y: {root_var}.y, w: {root_var}.width, h: {root_var}.height, "
          f"components: __made, missingMedia: [...new Set(__missing)] }};")
    return "\n".join(em.lines), em


def build_row(pen: PenDoc, ids: list[str], page: str | None, x: float, y: float, gap: float):
    """Beberapa board root (mis. 5 slide carousel) dalam SATU exec: satu prelude,
    satu settle per board, rotasi di akhir. Hemat round-trip & waktu tunggu tab."""
    em = Emitter(pen, False)
    em.js(PRELUDE)
    if page:
        em.js(f"""let __pg = penpotUtils.getPageByName({json.dumps(page)});
if (!__pg) {{ __pg = penpot.createPage(); __pg.name = {json.dumps(page)}; }}
if (penpot.currentPage.id !== __pg.id) await penpot.openPage(__pg);
if (penpot.currentPage.id !== __pg.id) throw new Error('page aktif salah');""")
    em.js("const __root = penpot.root; const __out = [];")
    cx = x
    roots = []
    for nid in ids:
        n = pen.nodes.get(nid) or pen.nodes.get(nid.split(":")[-1])
        if not n:
            sys.exit(f"node {nid} tidak ada")
        rv = em.node(n, "__root", "none", root=True)
        em.js(f"{rv}.x = {cx}; {rv}.y = {y};")
        roots.append(rv)
        cx += em.fallback(n.get("width"), 1080) + gap
    for rv in roots:
        em.js(f"await __settle({rv});")
    for line in em.deferred:
        em.js(line)
    em.js("await new Promise(r => setTimeout(r, 150));")
    em.js("return { roots: [" + ",".join(f"{{id:{rv}.id,name:{rv}.name,w:{rv}.width,h:{rv}.height}}" for rv in roots) + "], missingMedia: [...new Set(__missing)] };")
    return "\n".join(em.lines), em


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("pen")
    ap.add_argument("node")
    ap.add_argument("--page")
    ap.add_argument("--x", type=float)
    ap.add_argument("--y", type=float)
    ap.add_argument("--lib", action="append", default=[], help="file .pen tambahan untuk resolve ref")
    ap.add_argument("--components", action="store_true", help="node reusable → komponen Penpot")
    ap.add_argument("--into", help="id board Penpot tujuan (porting per section ke dalam board yang sudah ada)")
    ap.add_argument("--shallow", action="store_true", help="hanya node root tanpa children (kerangka)")
    ap.add_argument("--section", help="nama section sintetis saat NODE berisi beberapa id dipisah koma")
    ap.add_argument("--row", action="store_true", help="NODE berisi beberapa id → masing-masing jadi board root berjejer (x, x+w+gap, ...) dalam SATU exec")
    ap.add_argument("--gap", type=float, default=60, help="jarak antar board untuk --row")
    ap.add_argument("--emit", help="tulis JS ke file")
    ap.add_argument("--run", action="store_true", help="langsung jalankan via penpot-cli exec")
    ap.add_argument("--media", action="store_true", help="cetak daftar url image yang dibutuhkan lalu keluar")
    a = ap.parse_args()
    pen = PenDoc(Path(a.pen).expanduser(), [Path(p).expanduser() for p in a.lib])
    if a.row:
        js, em = build_row(pen, a.node.split(","), a.page, a.x or 0, a.y or 0, a.gap)
    else:
        js, em = build(pen, a.node, a.page, a.x, a.y, a.components, a.into, a.shallow, a.section)
    if a.media:
        print("\n".join(sorted(em.media)))
        return
    for w in sorted(set(em.warn)):
        print("[warn]", w, file=sys.stderr)
    out = a.emit or f"/tmp/pen2penpot-{a.node}.js"
    Path(out).write_text(js)
    print(f"[pen2penpot] {a.node}: {em.i} shape, {len(em.comp_roots)} komponen, {len(em.media)} gambar → {out} ({len(js)//1024} KB)", file=sys.stderr)
    if a.run:
        r = subprocess.run(["penpot-cli", "exec", "-f", out, "--timeout", "120s"])
        sys.exit(r.returncode)


if __name__ == "__main__":
    main()
