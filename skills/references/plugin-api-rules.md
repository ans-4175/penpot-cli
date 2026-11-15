# Plugin API rules — pola `exec` untuk Penpot

Padanan `execute.md` milik pen.dev, untuk Penpot Plugin API. Ringkas; sumber
kebenaran tetap `penpot-cli overview` + `penpot-cli api <Type> [member]`.
**Wajib juga baca [limits-and-gotchas.md](limits-and-gotchas.md)** — limit
terukur + jebakan yang sudah memakan korban.

## Model eksekusi

- `penpot-cli exec` = body **async function**: `await` boleh, hasil lewat
  `return` (objek JS langsung, tanpa `JSON.stringify`). Jangan `console.log`
  data yang juga di-return (dobel).
- Global: `penpot` (API), `penpotUtils` (helper resmi — pakai, jangan tulis
  ulang), `storage` (objek persisten lintas `exec` selama sesi daemon/plugin).
- **Tidak ada rollback**: exception di tengah kode meninggalkan shape yang
  sudah tercipta. Setelah error, cek dulu (`snapshot`/`state`) apa yang sudah
  ada, lanjutkan — jangan kirim ulang buta.
- Kode panjang → tulis ke file, `penpot-cli exec -f /tmp/section.js`.
- Satu section per `exec`; simpan id hasil ke `storage` untuk call berikutnya.

```js
storage.ids ??= {};
const b = penpot.createBoard();
b.name = 'WIP — Landing / Desktop';
storage.ids.page = b.id;
return { id: b.id };
```

Panggilan berikutnya: `penpotUtils.findShapeById(storage.ids.page)`.

## Koordinat & ukuran

- `x`/`y` = **absolut terhadap page**, writable. `parentX`/`parentY`/`boardX`
  read-only → posisi relatif: `penpotUtils.setParentXY(shape, px, py)`.
- `width`/`height` read-only → `shape.resize(w, h)`.
- Di dalam board flex/grid, x/y diatur layout (kecuali
  `layoutChild.absolute = true`, di mana x/y jadi relatif).

## Membuat shape

```js
const b = penpot.createBoard();          // board = container (bisa layout)
const r = penpot.createRectangle();
const e = penpot.createEllipse();
const t = penpot.createText('Halo');      // string non-kosong
const g = penpot.createShapeFromSvg('<svg …>…</svg>'); // ikon/vektor → Group
parent.appendChild(r);                    // urutan append = z-order & urutan flex
```

Shape baru masuk ke page aktif. **Mutasi hanya boleh di page aktif**:

```js
const pg = penpotUtils.getPageByName('Story Ads') ?? Object.assign(penpot.createPage(), { name: 'Story Ads' });
await penpot.openPage(pg);
if (penpot.currentPage.id !== pg.id) throw new Error('page aktif salah');
```

`page.root.id` sama untuk semua page — identifikasi page lewat `page.id`.

## Fills, strokes, radius

- Ganti **seluruh array**: `s.fills = [{ fillColor: '#0F172A', fillOpacity: 1 }]`;
  transparan: `s.fills = []`. Elemen array read-only.
- Hex huruf besar. Gradient: `{ fillColorGradient: { type:'linear', startX, startY, endX, endY, width:1, stops:[{color, opacity, offset}] } }`.
- Gambar dari URL publik: `const img = await penpot.uploadMediaUrl('nama', url)`.
- **Gambar lokal: `penpot-cli upload <file> [--key k]`** → `storage.media[k]`, lalu
  `s.fills = [{ fillOpacity: 1, fillImage: storage.media['k'] }]`. Jangan tempel
  base64 ke dalam `exec`: request MCP dibatasi ±100 KB (HTTP 413) — `upload`
  mengirim bertahap. `storage` hilang kalau tab plugin di-reload → upload ulang.
- `strokes = [{ strokeColor, strokeWidth, strokeStyle:'solid', strokeAlignment:'inner' }]`.
- `borderRadius` (uniform) atau `borderRadiusTopLeft` … per sudut.

## Flex layout

```js
const sec = penpot.createBoard();
sec.name = 'Hero';
const fl = sec.addFlexLayout();          // board BARU tanpa children
fl.dir = 'column';                        // row | column | *-reverse
fl.rowGap = 24; fl.columnGap = 0;
fl.alignItems = 'center'; fl.justifyContent = 'start';
fl.topPadding = fl.bottomPadding = 96; fl.leftPadding = fl.rightPadding = 120;
fl.horizontalSizing = 'fix'; fl.verticalSizing = 'auto'; // board tumbuh ikut isi
page.appendChild(sec);
sec.layoutChild.horizontalSizing = 'fill'; // isi lebar parent flex
```

- Board yang **sudah punya children**: `penpotUtils.addFlexLayout(board, 'column')`
  (menjaga urutan visual). `board.addFlexLayout()` bisa mengacak urutan.
- Urutan: `children` = urutan visual. Pindah posisi → `parent.insertChild(i, shape)`.
  **`setParentIndex()` tidak berefek pada urutan flex.**
- Ukuran awal: sumbu `auto` → `resize(…, 1)` dulu (auto tidak menyusut di bawah
  resize terakhir); sumbu `fill` → ukuran parent.
- Setelah membangun satu section: **settle** board flex dari terdalam ke luar
  (Penpot tidak selalu reflow `auto` dari API):
  ```js
  const depth = s => { let d = 0, p = s.parent; while (p) { d++; p = p.parent; } return d; };
  const boards = penpotUtils.findShapes(s => s.type === 'board' && !!s.flex, section).sort((a, b) => depth(b) - depth(a));
  for (let i = 0; i < 2; i++) { for (const b of boards) { const r = b.rotation || 0; b.rotation = r + 0.01; b.rotation = r; } await new Promise(r => setTimeout(r, 150)); }
  ```
- **Rotasi diterapkan paling akhir** (setelah settle). Rotasi parent diwariskan
  ke anak yang ditambahkan sesudahnya dan membengkakkan bbox → layout kacau.
- Sizing container: `fix | auto | fill`. Mau container ikut isi → `auto`.
- Sizing child: `child.layoutChild.horizontalSizing/verticalSizing = 'fix'|'auto'|'fill'`.
- Spasi lewat `rowGap`/`columnGap`/padding, bukan x/y child. Margin per
  child ada (`layoutChild.topMargin` …) tapi jarang perlu.
- Grid: `board.addGridLayout()`, `grid.appendChild(shape, row, col)` (1-based).
- `fill` di parent `auto` pada sumbu yang sama = dependensi melingkar → kolaps.

## Rotasi

- `shape.rotation` Penpot berputar **searah jarum jam** untuk nilai positif —
  kebalikan pen.dev (CCW). Porting dari .pen: **balik tandanya** (pen `-1.2` →
  Penpot `1.2`). Set rotation **setelah** shape masuk parent/layout.

## Teks

```js
const t = penpot.createText('Otomatis. Murah.');
t.name = 'Hero / Headline';
const exact = n => penpot.fonts.findAllByName(n).find(f => f.name === n); // findByName = cocok sebagian!
const font = exact('Plus Jakarta Sans');
font.applyToText(t, font.variants.find(v => v.fontWeight === '700' && v.fontStyle !== 'italic'));
t.fontSize = '64';                        // cek tipe via `api Text fontSize`
t.fills = [{ fillColor: '#0F172A', fillOpacity: 1 }];
t.growType = 'auto-height';              // wrap mengikuti lebar
hero.appendChild(t);
t.layoutChild.horizontalSizing = 'fill';
```

- `growType`: `auto-width` (satu baris, tumbuh ke kanan) · `auto-height`
  (wrap, tinggi ikut isi) · `fixed`. **`resize()` memaksa `fixed`** → set
  ulang growType setelah resize.
- Ukuran font ≠ resize. Auto-size tidak instan: `await new Promise(r => setTimeout(r, 100))`
  sebelum membaca `textBounds`/height.
- Styling sebagian teks: `t.getRange(start, end)` → set properti di range.
- Cek font tersedia dengan **nama persis** (`findAllByName(n).find(f => f.name === n)`).
  `findByName('Anton')` mengembalikan "Anton SC", `'Caveat'` → "Caveat Brush".
  `font.variants` berisi `fontWeight` string (`'800'`) + `fontStyle`; pilih
  variant non-italic secara eksplisit.
- Teks multi-warna dalam satu baris: row board flex (`dir='row'`, sizing
  `auto`) berisi beberapa Text `auto-width` — lebih mudah dikontrol daripada range.

### Helper teruji (simpan di storage, pakai lintas exec)

```js
storage.h = {
  txt(content, family, size, weight, color, name) {
    const t = penpot.createText(content); t.name = name || content.slice(0, 24);
    const f = penpot.fonts.findAllByName(family).find(x => x.name === family);
    f.applyToText(t, f.variants.find(v => v.fontWeight === String(weight) && v.fontStyle !== 'italic') || f.variants[0]);
    t.fontSize = String(size); t.fills = [{ fillColor: color, fillOpacity: 1 }]; t.growType = 'auto-width';
    return t;
  },
  row(name, gap = 0) {
    const r = penpot.createBoard(); r.name = name; r.fills = [];
    const fl = r.addFlexLayout(); fl.dir = 'row'; fl.alignItems = 'center'; fl.columnGap = gap;
    fl.horizontalSizing = 'auto'; fl.verticalSizing = 'auto'; return r;
  },
};
```

Board baru default punya fill putih — set `fills = []` untuk wrapper layout.
Teks lebar penuh + rata tengah: append ke parent, `resize(W, h)`, lalu
`growType = 'auto-height'`, `align = 'center'`.

## Komponen & library

- Buat: `penpot.library.local.createComponent([shape])` (atau
  `penpot-cli component create <id> --name 'Button / Primary'`).
- Pakai: `const inst = comp.instance(); parent.appendChild(inst);` lalu ubah
  teks/fill pada descendant instance.
- Hapus/ubah struktur instance butuh `inst.detach()` — hanya bila memang
  sengaja memutus link.
- Variants: `penpotUtils.createVariantContainer([{ shape, properties: { Size: 'S' } }, …])`.
- Warna/tipografi library: `penpot.library.local.createColor()` /
  `createTypography()`. Tokens: `penpot.library.local.tokens` — cek skema
  dulu (`penpot-cli api TokenCatalog`, `api TokenSet`).

## Membaca & verifikasi

```js
return penpotUtils.shapeStructure(penpot.root, 2);              // overview page
return penpotUtils.findShapes(s => s.name.startsWith('WIP — ')).map(s => s.name);
const bad = penpotUtils.analyzeDescendants(board, (root, s) =>
  !penpotUtils.isContainedIn(s, root) ? 'keluar-bounds' : null);
return bad.map(x => ({ id: x.shape.id, name: x.shape.name, issue: x.result }));
```

- Struktur dalam: `penpot-cli snapshot <id>`.
- Visual: `penpot-cli export <id> --out /tmp/sec.png` lalu lihat gambarnya.
  Export per section (bukan seluruh page) — lebih tajam, lebih murah.
- Seleksi user: `penpot.selection` — **langsung salin ke storage**, seleksi
  bisa berubah.

## Hapus dengan aman

```js
// sebelum hapus: di shell → penpot-cli backup save "sebelum hapus X"  (bukan saveVersion)
await penpot.openPage(pg); if (penpot.currentPage.id !== pg.id) throw new Error('page salah');
const s = penpotUtils.findShapeById(id);
if (!s || s.name !== expectedName) throw new Error('target tidak cocok');
s.remove();
```

Jangan pernah `root.children.forEach(s => s.remove())`.

## Penempatan board root

Jangan menimpa board lain. Cari ruang kosong di kanan konten yang ada:

```js
const kids = penpot.root.children;
const right = kids.length ? Math.max(...kids.map(s => s.x + s.width)) : 0;
const top = kids.length ? Math.min(...kids.map(s => s.y)) : 0;
b.x = right + 200; b.y = top;
```

Beberapa layar berurutan: tiap board berikutnya `x = prev.x + prev.width + 120`.

## Jebakan yang sering terjadi

| Gejala | Penyebab | Perbaikan |
|---|---|---|
| Teks terpotong/overflow | `resize()` membuat growType `fixed` | set `growType = 'auto-height'` lagi |
| Urutan children acak setelah layout | `addFlexLayout()` pada board berisi | `penpotUtils.addFlexLayout(board, dir)` |
| Board kolaps / 0 px | parent `auto` + semua child `fill` | beri ukuran tetap atau satu child `auto` |
| x/y child tidak berefek | parent punya layout | pakai gap/padding, atau `layoutChild.absolute` |
| Mengubah fill tidak berefek | mutasi elemen array | ganti seluruh `fills` |
| Shape muncul di page lain | page baru belum di-`openPage` | `await penpot.openPage(page)` dulu |
| "plugin tab suspended" | tab Penpot di background | fokuskan tab Penpot (penpot-cli menunggu otomatis s.d. 3 menit) |
| "Cannot modify a page that is not currently active" | mutasi page lain | `await penpot.openPage(pg)` + cek `currentPage.id` |
| Board auto tidak ikut isi | belum reflow | settle (nudge rotasi) dari terdalam ke luar |
| Kartu/teks membengkak | rotasi parent diterapkan sebelum anak dibangun | rotasi paling akhir |
| Urutan flex tidak berubah | `setParentIndex` | `insertChild(i, shape)` |
| Font salah ("Anton SC", "Caveat Brush") | `findByName` cocok sebagian | cari nama persis |
| Respons > 15 MB | export/snapshot board raksasa | per section |
| HTTP 413 PayloadTooLarge | kode `exec` > ±100 KB (biasanya base64) | `penpot-cli upload` untuk gambar; pecah kode |
| "Permission user:read is not granted" | akses `penpot.currentUser` | jangan baca data user; plugin tidak diberi izin itu |
| Rotasi kebalikan dari .pen | Penpot CW, pen.dev CCW | balik tanda rotation |
| Hapus child instance gagal | instance komponen | `enabled/visible=false` atau `detach()` |
