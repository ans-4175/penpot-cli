# Limits & gotchas — Penpot MCP + Plugin API (terukur, Okt 2026)

Semua angka di bawah DIUKUR langsung di design.penpot.app lewat penpot-cli,
bukan dari dokumentasi. Baca file ini sebelum kerja besar (porting, build
banyak board, operasi hapus).

## 1. Limit transport

| Limit | Angka | Sumber | Workaround (sudah di penpot-cli) |
|---|---|---|---|
| Body request | **±100 KB** (99 KB lolos, 101 KB → HTTP 413) | server MCP (express body-parser) | `exec` > 90 KB otomatis dipecah 64 KB → dirakit di `storage.__code` → `eval`. Gambar: `penpot-cli upload` bertahap |
| Ukuran respons | **15 MB** ("Serialised response size … exceeds maximum of 15000000") | server MCP | export per board/section, bukan board raksasa (board 3600×17800 gagal) |
| Lama satu eksekusi | **120 s** ("Task … timed out after 120 seconds") | server MCP | ≤ 1 section per `exec`; jangan `setTimeout` panjang |
| Tab plugin tidur — **solusi: `penpot-cli app`** | Teruji Okt 2026, jendela disembunyikan: **Chrome biasa** timer melambat (100 ms → 0,8–17 s), teks baru terukur 1×1; **Safari** berhenti menjawab total setelah ±30 s (keepalive ikut timeout); **Chrome + `--disable-background-timer-throttling --disable-backgrounding-occluded-windows --disable-renderer-backgrounding`** (profil terpisah, mode `--app`) normal selama 10 menit (timer 305 ms, teks 31×17, respons 1–2 s). Kunci domain via extension TIDAK bisa: Chrome mengabaikan `--load-extension`. Jangan buka file yang sama di 2 jendela (2 sesi, MCP hanya di satu) | browser | `penpot-cli app` |
| Tab plugin tidur (lama) | suspend setelah ±60–90 s tanpa aktivitas — **walau di jendela sendiri** (Chrome Memory Saver / timer throttling) | browser | (1) **keepalive daemon**: ping `return 1` tiap 25 s selama ada aktivitas agent ≤10 menit terakhir (`PENPOT_KEEPALIVE=0` mematikan) — terbukti: idle 90/150 s tetap 0 suspend; (2) penpot-cli menunggu s.d. 3 menit kalau tetap suspend (`PENPOT_SUSPEND_WAIT`), aman karena server menolak SEBELUM eksekusi; (3) opsional user: Chrome → Settings → Performance → Memory Saver → "Always keep these sites active": `design.penpot.app` |
| Header | tanpa `User-Agent` → HTTP 403 | Cloudflare | penpot-cli selalu kirim UA |
| Export server timeout | export board berat kadang `:code :timeout` | backend Penpot | ulangi sekali; kecilkan target |

## 2. Model eksekusi

- `exec` = body async function. `return` → hasil; console → stderr.
- Global: `penpot`, `penpotUtils`, `storage`. **`storage` TIDAK terlihat di
  dalam `eval`** → teruskan sebagai parameter (penpot-cli melakukannya untuk
  kode chunked). `eval` & `new Function` jalan; `AsyncFunction` constructor
  tidak.
- `storage` bertahan lintas sesi MCP / restart daemon; **hilang kalau tab di-reload**.
- Tidak ada rollback: exception di tengah meninggalkan shape setengah jadi.
- Tidak ada `document`/`requestAnimationFrame` (sandbox).
- Izin: `user:read` TIDAK diberikan → jangan akses `penpot.currentUser`.

## 3. Page & destruktif — WAJIB

- **Mutasi hanya bisa pada page AKTIF** ("Cannot modify a page that is not
  currently active"). Sebelum menyentuh page lain:
  ```js
  const pg = penpotUtils.getPageByName('X');
  await penpot.openPage(pg);
  if (penpot.currentPage.id !== pg.id) throw new Error('page aktif salah');
  ```
- **`page.root.id` sama untuk SEMUA page** (`00000000-…`). Jangan pakai
  `root.id` untuk identifikasi; pakai `page.id`.
- Membaca `pg.root.children` page non-aktif bisa saja benar, tapi untuk
  **hapus** selalu aktifkan page dulu + verifikasi target per id
  (`getPluginData('penId')` / nama persis). Jangan `forEach(remove)` massal.
- Sebelum operasi destruktif besar: **`penpot-cli backup save "<label>"`** (REST,
  instan, tidak bergantung tab). `penpot.currentFile.saveVersion()` normal ±0,8 s;
  ia menggantung HANYA kalau antrean simpan tab macet (lihat di bawah) — itu
  gejala, bukan penyebab.
  Riwayat: `penpot.currentFile.findVersions()` (ada snapshot otomatis ±5 revisi).
- Jangan ganti page tempat user menaruh pekerjaan — tanya dulu page tujuan.

## 4. Layout flex — jebakan nyata

| Gejala | Penyebab | Perbaikan |
|---|---|---|
| Board `auto` tidak membesar/mengecil setelah anak ditambah | Penpot tidak reflow board `auto` dari API sampai ada perubahan geometri | **"settle"**: hanya board flex `auto`, dari terdalam ke luar, `b.rotation = r + 0.01; b.rotation = r`, ulang sampai ukuran stabil (maks 3 pass, jeda 60 ms). pen2penpot melakukannya otomatis. **`penpot.waitForLayoutUpdate()`** (ada di runtime 2.18.3, tidak di docs) **tidak menolong**: tanpa argumen timeout 30 s, dengan id/shape → "Value not valid" |
| Board `auto` macet di ukuran lama | `auto` tidak menyusut di bawah `resize()` terakhir | ukuran awal sumbu `auto` = **1px**; sumbu `fill` = ukuran parent |
| Kartu/teks membengkak tinggi | **rotasi parent diwariskan ke anak** saat anak masih dibangun → bbox anak membesar | terapkan `rotation` **paling akhir**, setelah layout & settle |
| Rotasi anak di induk berotasi jadi dobel/salah | `rotation` Penpot = sudut **absolut**, anak ikut berputar bersama induk | terapkan top-down: induk dulu, lalu `anak.rotation = anak.rotation + relatif` |
| Beberapa anak `fill` di baris flex: satu melebar, sisanya terjepit | ukuran awal anak `fill` di sumbu utama = lebar parent | sumbu utama → mulai 1px; sumbu silang → ukuran parent |
| Teks wrap 1 baris lebih banyak dari pen.dev | metrik font render beda tipis | normal; cek visual, jangan dikejar lewat angka |
| Urutan flex tidak berubah | `setParentIndex()` TIDAK mengubah urutan flex | `parent.insertChild(i, shape)`; urutan `children` = urutan visual (index 0 = pertama) |
| Anak teracak setelah layout | `addFlexLayout()` pada board berisi | `penpotUtils.addFlexLayout(board, dir)` |
| `fill` + parent `auto` sumbu sama | dependensi melingkar | parent fix, atau satu anak auto/fix |
| x/y anak flex tidak berefek | layout menang | gap/padding, atau `layoutChild.absolute = true` (x/y relatif) |
| Board baru putih | default fill board | `fills = []` untuk wrapper layout |

## 5. Teks & font

- `penpot.fonts.findByName()` = **pencocokan sebagian**: "Anton" → "Anton SC",
  "Caveat" → "Caveat Brush", "Inter" → "Inter Tight". Selalu cari nama persis:
  ```js
  const exact = n => penpot.fonts.findAllByName(n).find(f => f.name === n);
  ```
- `fontSize`, `fontWeight`, `lineHeight`, `letterSpacing` bertipe **string**.
- Pilih variant eksplisit: `f.variants.find(v => v.fontWeight === '700' && v.fontStyle !== 'italic')`.
- `applyToText()` me-reset ukuran/lineHeight → set ulang setelahnya.
- `resize()` memaksa `growType = 'fixed'` → set ulang growType.
- Lebar penuh: append ke parent flex → `layoutChild.horizontalSizing='fill'`
  → `growType='auto-height'`.

## 6. Komponen

- `createComponent([shape])` pada **non-board** (text/rect) → Penpot membungkus
  dengan board 1×1 yang jelek. Bungkus sendiri dengan board flex lebih dulu.
- Komponen shape (rect/ellipse) yang harus bisa di-resize (scrim, garis):
  wrapper `fix` + anak `fill` di kedua sumbu.
- Nama `Marketing / Link Sticker` → set `c.name = 'Link Sticker'`, `c.path = 'Marketing'`
  (kalau tidak, nama jadi dobel "Marketing / Marketing / …").
- Instance: `comp.instance()` lalu `parent.appendChild(inst)`; override via
  anak instance (`characters`, `fills`, `hidden`). Mengubah main component
  menyebar ke semua instance.
- `remove()` main instance menghapus komponen dari library.
- Peta pen→Penpot disimpan di plugin data file `penpot-cli.pen-map`.

- **Variants** (`penpotUtils.createVariantContainer([{shape, properties:{Arah:'In'}}, …])`)
  BERHASIL, tapi operasinya berat: di file besar (DS 560 node) satu panggilan
  bisa melewati 120 s dan **memutus plugin** ("No Penpot instance connected"
  — pulih sendiri ±1–2 menit). Aturan: `penpot-cli backup save` dulu, buat variant di
  **exec terpisah** (tanpa operasi lain), lalu verifikasi di exec berikutnya.
  Komponen baru hasil container bernama "Component" → rename setelahnya.

- **Board flex `auto` yang DIPUTAR** (marker/highlight miring): Penpot
  menghitung ulang ukurannya dari bounding box anak yang ikut miring → board
  membengkak tiap reflow (430×177 → 586×533) dan exec melambat (26 node =
  112 s, timeout). Fix (porter `__rotate`): tunggu teks anak terukur (>1px),
  kunci ukuran = isi + padding (auto → fix), baru putar. Jangan kunci pakai
  `b.width` saat itu — bisa masih 33×1 (belum reflow).

- **Antrean simpan tab macet → perubahan TIDAK tersimpan ke server** (teruji,
  Okt 2026): setelah shape rusak (teks lebar 0 / tinggi 70.519.818 px) server tidak
  lagi menaikkan `revn`; semua perubahan berikutnya hanya hidup di tab. Gejala:
  `export` (baca dari server) tidak berubah — hash PNG identik — sementara
  `snapshot`/`exec` di tab menunjukkan nilai baru; `saveVersion` & exec berulang
  timeout; reload tab membuang semuanya. Cek: **`penpot-cli sync`** (revn naik?),
  `doctor` menampilkan umur simpan terakhir. Kalau TIDAK: jangan lanjut membangun;
  hapus shape rusak per id; kalau tetap, minta user reload (Cmd+R) lalu bangun ulang
  bagian yang belum tersimpan. Jalankan `sync` setelah tiap slide/section besar dan
  SEBELUM export QA — jangan percaya gambar export kalau hash tidak berubah.
- **Teks `fill` di board `auto`** (lebar) → saling menunggu: board menyusut ke
  isi (mis. 44 px), teks fill jadi 0 px lebar, tinggi meledak (437 → 70.519.818 px),
  isi slide meluap dan exec timeout. Aturan: teks `fill` hanya di dalam board yang
  lebarnya **fix** atau **fill**; board `auto` hanya berisi teks `auto-width`.
  Untuk catatan satu baris di slide: board lebar fix + teks `fixed` (`resize`).
- **Sisa shape rusak setelah timeout**: cek `findShapes(t => t.type==='text' &&
  (t.width<=1.5 || t.height>5000))` sebelum mengulang; hapus per id.

## 7. Tokens (teruji)

- `lib.tokens.addSet({name, active:true})`, `set.addToken({type, name, value})`,
  alias `'{color.terracotta}'` → `resolvedValue` ikut. Theme: `addTheme({group, name})`
  + `theme.addSet(set)` + `toggleActive()`.
- `shape.applyToken(token, props)` — **props harus array**; nama properti di
  docs SALAH sebagian: font → `['fontFamily']` (bukan `fontFamilies`),
  radius → 4 sudut `borderRadiusTopLeft…` (bukan `borderRadius`/`all`).
  Color → `['fill']` / `['strokeColor']`.
- Ubah token: `token.value = '#…'` — nilai berubah tapi shape **tidak
  di-recompute** sampai set di-toggle: `set.toggleActive(); …; set.toggleActive()`.
  Setelah itu main component **dan semua instance** di page lain ikut (teruji:
  16 Brand Pill instance di carousel).

- **Library typography** (`lib.createTypography()` + `setFont(font, variant)`;
  `fontSize/lineHeight/letterSpacing` **string**, angka → "Value not valid").
  `typo.applyToText(text)` **mereset lineHeight ke 1.2** (bug) → simpan
  lineHeight dulu, set ulang setelahnya, lalu toggle `growType` + settle board.
- Library color: `lib.createColor()`; set `name`, `path`, `color` ('#hex'), `opacity`.

## 8. Shared Library (REST, bukan MCP)

Plugin API tidak bisa publish. `penpot-cli library publish|link|unlink|status|list`
memakai REST RPC `/api/rpc/command/*` dengan `PENPOT_PERSONAL_TOKEN`
(header `Authorization: Token …`, wajib `User-Agent`, `get-file`/`create-file`
wajib query/body `features`). Teruji: create-file → set-file-shared → muncul di
get-team-shared-files → link-file-to-library → unlink → delete-file.
Setelah link, reload tab supaya `penpot.library.connected` ter-update.
**Token TIDAK ikut tersinkron lewat library** (by design): komponen/colors/
typographies sinkron; token harus di-import di file pemakai (Assets →
Libraries → "Import tokens", menimpa semua token file itu) atau export/import JSON.

## 7b. Media

- `penpot-cli upload <file|dir>... --base <dir> --max 1400` → key = path
  relatif (= url fill di .pen). PNG transparan tetap PNG, lainnya JPEG q85.
- Manifest di plugin data file `penpot-cli.media` (ImageData + sha1):
  upload ulang isi yang sama dilewati; `storage.media` dipulihkan setelah
  tab reload **tanpa upload ulang**. Kecepatan: ±1 MB per 15 s.
- Pakai: `shape.fills = [{ fillOpacity: 1, fillImage: storage.media[key] }]`.

## 8b. Rotasi, warna, efek

- Rotasi Penpot **searah jarum jam** (pen.dev CCW) → balik tanda.
- Warna `#RRGGBBAA` → `fillColor '#RRGGBB'` + `fillOpacity` (alpha/255).
- Shadow: `{style:'drop-shadow', offsetX, offsetY, blur, spread, color:{color, opacity}}`.
- Gradient: `fillColorGradient:{type:'linear', startX,startY,endX,endY, width:1, stops:[{color,opacity,offset}]}`.
- Ikon lucide: `penpot.createShapeFromSvg(svg)` → Group; stroke-width diskala `2*size/24`.

## 9. Kecepatan (terukur, port carousel/story)

| Operasi | Waktu |
|---|---|
| exec kosong (round-trip) | ±0,4 s (pertama ±1 s) |
| create+append+remove 1 shape | ±14 ms (di dalam plugin) |
| settle 1 board (nudge) | ±13 ms; DS 231 board ≈ 40 s → settle hanya board `auto` |
| `findShapes` di board 560 node | ±4 s — jangan diulang dalam loop |
| port 1 story (40 shape) | 5–17 s |
| carousel 5 slide, 5 exec | ±144 s (banyak menunggu tab suspended) |
| carousel 6 slide, `--row` 1 exec | 1 round-trip; dibatasi 120 s server → ≤ ±6 slide per exec |
| upload | ±1 MB / 15 s (setelah resize `--max 1400`) |

Waktu terbesar hilang di **tab suspended** (menunggu fokus) — bukan eksekusi.

## 10. QA

- Export per board → bandingkan dengan export asli (`visual_diff.py` atau
  side-by-side). Jangan percaya angka width/height saja — tapi juga jangan
  percaya satu screenshot: viewer gambar bisa men-cache nama file yang sama,
  pakai nama unik per export.
- `penpotUtils.analyzeDescendants(root, (r,s) => !penpotUtils.isContainedIn(s,r) ? 'out' : null)`
  untuk cek clipping.
