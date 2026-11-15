const C = { accent: '#C9412E', marker: '#FBE36B', navy: '#1F3A8A', ink: '#1E2230', sub: '#5B6170', hair: '#DDE0E7', foot: '#8A8F9C', peach: '#FBE1DC', white: '#FFFFFF', card: '#F3F4F8', dark: '#12141C' };
const A = JSON.parse(storage.liAssets || 'null');
const __mediaRaw = penpot.currentFile.getPluginData('penpot-cli.media');
const __mediaMan = __mediaRaw ? JSON.parse(__mediaRaw) : {};
const IMG = (url) => { const m = (storage.media || {})[url] ?? __mediaMan[url]?.img; if (!m) throw new Error('media belum diupload: ' + url); return { fillOpacity: 1, fillImage: m }; };
const FC = {};
const FONT = (t, fam, wt) => {
  const exact = n => penpot.fonts.findAllByName(n).find(x => x.name === n) || penpot.fonts.all.find(x => x.name === n);
  const f = FC[fam] ?? (FC[fam] = exact(fam));
  if (!f) throw new Error('font tidak ada: ' + fam);
  const v = f.variants.find(v => v.fontWeight === String(wt) && v.fontStyle !== 'italic') || f.variants[0];
  f.applyToText(t, v);
};
const J = 'JetBrains Mono', P = 'Plus Jakarta Sans';
// board flex. o: dir,gap,pad[t,r,b,l],ai,jc,w,h (angka | 'fill' | 'auto'),fill,r,stroke,sw,name
const BOX = (parent, o) => {
  const b = penpot.createBoard(); b.name = o.name || 'Box'; b.clipContent = !!o.clip;
  parent.appendChild(b); b.resize(typeof o.w === 'number' ? o.w : 1, typeof o.h === 'number' ? o.h : 1);
  const f = b.addFlexLayout(); f.dir = o.dir || 'column';
  f.rowGap = o.gap || 0; f.columnGap = o.gap || 0;
  f.alignItems = o.ai || 'start'; f.justifyContent = o.jc || 'start';
  const p = o.pad || [0, 0, 0, 0];
  f.topPadding = p[0]; f.rightPadding = p[1]; f.bottomPadding = p[2]; f.leftPadding = p[3];
  const sz = v => typeof v === 'number' ? 'fix' : (v === 'fill' ? 'fix' : 'auto');
  f.horizontalSizing = sz(o.w ?? 'auto'); f.verticalSizing = sz(o.h ?? 'auto');
  b.fills = o.fill ? [typeof o.fill === 'string' ? { fillColor: o.fill, fillOpacity: 1 } : o.fill] : [];
  if (o.r !== undefined) b.borderRadius = o.r;
  if (o.stroke) b.strokes = [{ strokeColor: o.stroke, strokeOpacity: 1, strokeWidth: o.sw || 2, strokeStyle: 'solid', strokeAlignment: 'inner' }];
  if (b.layoutChild && parent.flex) { b.layoutChild.horizontalSizing = o.w === 'fill' ? 'fill' : (typeof o.w === 'number' ? 'fix' : 'auto'); b.layoutChild.verticalSizing = o.h === 'fill' ? 'fill' : (typeof o.h === 'number' ? 'fix' : 'auto'); }
  return b;
};
// teks. o: size,wt,fam,color,ls,lh,w ('fill' | angka | undefined=auto),align,name
const TXT = (parent, s, o) => {
  const t = penpot.createText(s); t.name = o.name || s.slice(0, 24);
  FONT(t, o.fam || P, o.wt || 500); t.fontSize = String(o.size || 24);
  if (o.ls !== undefined) t.letterSpacing = String(o.ls);
  t.lineHeight = String(o.lh || 1.2);
  t.fills = [{ fillColor: o.color || C.ink, fillOpacity: 1 }];
  if (o.align) t.align = o.align;
  parent.appendChild(t);
  if (o.w === 'fill' && parent.flex) { t.growType = 'auto-height'; t.layoutChild.horizontalSizing = 'fill'; }
  else if (typeof o.w === 'number') { t.resize(o.w, t.height); t.growType = 'auto-height'; }
  else t.growType = 'auto-width';
  return t;
};
const ICON = (parent, name, size, color) => {
  const svg = A.icons[name].replace(/stroke="currentColor"/g, `stroke="${color}"`).replace(/width="24"/, `width="${size}"`).replace(/height="24"/, `height="${size}"`);
  const s = penpot.createShapeFromSvg(svg); s.name = 'Icon ' + name; parent.appendChild(s); s.resize(size, size);
  if (s.layoutChild) { s.layoutChild.horizontalSizing = 'fix'; s.layoutChild.verticalSizing = 'fix'; }
  return s;
};
const DOT = (parent, size, color) => { const e = penpot.createEllipse(); e.name = 'Dot'; parent.appendChild(e); e.resize(size, size); e.fills = [{ fillColor: color, fillOpacity: 1 }]; if (e.layoutChild) { e.layoutChild.horizontalSizing = 'fix'; e.layoutChild.verticalSizing = 'fix'; } return e; };
const CHIP = (parent, label) => { const c = BOX(parent, { name: 'Eyebrow Chip', dir: 'row', gap: 12, pad: [12, 26, 12, 26], ai: 'center', fill: C.peach, r: 999 }); DOT(c, 12, C.accent); TXT(c, label, { fam: J, wt: 700, size: 24, color: C.accent, ls: 2, name: 'Eyebrow' }); return c; };
// judul + marker (rotasi dikunci lewat ROT di akhir)
const MARK = (parent, s, size, deg) => { const m = BOX(parent, { name: 'Marker', dir: 'row', pad: [4, 18, 4, 18], fill: C.marker, r: 10, ai: 'center' }); TXT(m, s, { wt: 800, size, ls: -size * 0.03, color: C.ink, lh: 1.1, name: 'marked' }); MARKS.push([m, deg]); return m; };
const MARKS = [];
const wait = ms => new Promise(r => setTimeout(r, ms));
const SETTLE = async (root) => {
  const depth = s => { let d = 0, p = s.parent; while (p) { d++; p = p.parent; } return d; };
  const bs = [root, ...penpotUtils.findShapes(s => s.type === 'board' && s.flex && (s.flex.horizontalSizing === 'auto' || s.flex.verticalSizing === 'auto'), root)].sort((a, b) => depth(b) - depth(a));
  for (let pass = 0; pass < 2; pass++) {
    const before = bs.map(b => b.width + 'x' + b.height).join('|');
    for (const b of bs) { const r = b.rotation || 0; b.rotation = r + 0.01; b.rotation = r; }
    await wait(60);
    if (bs.map(b => b.width + 'x' + b.height).join('|') === before) break;
  }
};
// kunci ukuran marker (isi + padding) lalu putar
const ROT = async () => {
  for (const [b, deg] of MARKS) {
    const fl = b.flex; const k = b.children[0];
    for (let i = 0; i < 8 && (k.width <= 1.5 || k.height <= 1.5); i++) await wait(50);
    const w = k.width + fl.leftPadding + fl.rightPadding, h = k.height + fl.topPadding + fl.bottomPadding;
    fl.horizontalSizing = 'fix'; fl.verticalSizing = 'fix'; b.resize(w, h);
    b.rotation = (b.rotation + deg + 360) % 360;
  }
  MARKS.length = 0;
};
// slide 1080x1350
const SLIDE = (name, x, dark, idx, total) => {
  const s = penpot.createBoard(); s.name = name; s.clipContent = true; penpot.root.appendChild(s); s.resize(1080, 1350); s.x = x; s.y = 0;
  const f = s.addFlexLayout(); f.dir = 'column'; f.rowGap = 36; f.alignItems = 'start'; f.justifyContent = 'start';
  f.topPadding = 96; f.bottomPadding = 150; f.leftPadding = 80; f.rightPadding = 80; f.horizontalSizing = 'fix'; f.verticalSizing = 'fix';
  s.fills = [{ fillColor: dark ? C.dark : C.white, fillOpacity: 1 }];
  const ft = penpot.createText('yourname.com'); ft.name = 'Footer URL'; FONT(ft, J, 700); ft.fontSize = '28'; ft.fills = [{ fillColor: C.foot, fillOpacity: 1 }];
  s.appendChild(ft); ft.layoutChild.absolute = true; ft.growType = 'auto-width'; ft.x = s.x + 80; ft.y = 1270;
  const pn = penpot.createText(idx + ' / ' + total); pn.name = 'Page Number'; FONT(pn, J, 700); pn.fontSize = '26'; pn.fills = [{ fillColor: dark ? C.foot : C.sub, fillOpacity: 1 }];
  s.appendChild(pn); pn.layoutChild.absolute = true; pn.resize(200, 30); pn.growType = 'auto-height'; pn.align = 'right'; pn.x = s.x + 800; pn.y = 1272;
  return s;
};
