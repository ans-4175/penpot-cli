---
name: penpot
description: "Drive Penpot (design.penpot.app) via penpot-cli — persistent MCP session daemon: inspect/edit the focused file & page, build boards/landing/ads with the Plugin API, design-to-design from screenshots, code-to-design (React/Tailwind), design-to-code, visual QA via export. Requires a Penpot tab connected through the MCP plugin."
allowed-tools: Bash, Read, AskUserQuestion
---

# Penpot workflow skill

Work with Penpot through **`penpot-cli`** (`~/.local/bin/penpot-cli`, source
the penpot-cli source repo). A local daemon keeps **one persistent MCP session**
to the remote Penpot MCP (`design.penpot.app/mcp/stream`): initialize once,
Mcp-Session-Id + keep-alive reused, auto re-initialize if the server drops the
session, calls serialized, timeouts never kill the session. First call ≈1s,
later calls ≈0.3s.

Penpot is **not** CSS/HTML and **not** pen.dev: you write JavaScript against the
**Penpot Plugin API** (`penpot`, `penpotUtils`, `storage`) executed inside the
user's browser tab. Flex/grid layouts exist but with their own API.

```
kamu → agent → penpot-cli (daemon, 1 sesi MCP) → Penpot MCP server (remote)
     → plugin di TAB BROWSER user → file Penpot
```
Semua kerja canvas dieksekusi plugin di tab user. **Buka Penpot dengan
`penpot-cli app`** (jendela Chrome khusus, anti-throttling): teruji tetap
normal walau jendela disembunyikan 10 menit. Tab Chrome biasa melambat, Safari
berhenti total setelah ±30 s tersembunyi. Daemon menjaga tab tetap bangun dengan keepalive ringan
selama agent aktif; kalau tetap tertidur, CLI menunggu s.d. 3 menit.

## Aturan WAJIB (pelanggaran = kerusakan nyata)

1. **Page aktif saja.** Sebelum menyentuh page lain: `await penpot.openPage(pg)`
   lalu cek `penpot.currentPage.id === pg.id`. `page.root.id` sama untuk semua
   page — jangan dipakai sebagai identitas.
2. **Jangan ganti page tujuan user.** Tanya/konfirmasi page untuk tiap jenis
   artefak (mis. story → page "Story Ads" yang sudah ada; carousel → page baru).
   Jangan membuat page "sementara" tanpa izin.
3. **Hapus = per id terverifikasi** + `penpot-cli backup save "<label>"` dulu kalau besar. Tidak ada
   `forEach(remove)` massal. Tidak ada rollback otomatis.
4. **Font dengan nama persis** (`findAllByName(n).find(f => f.name === n)`) —
   `findByName` cocok sebagian ("Anton" → "Anton SC").
5. **Layout: ukuran awal → isi → settle → rotasi terakhir.** Lihat
   [limits-and-gotchas.md](references/limits-and-gotchas.md) §4.
6. **Urutan flex pakai `insertChild(i, s)`**, bukan `setParentIndex`.
7. **Gambar lokal → `penpot-cli upload`**, bukan base64 di `exec`.
8. **Verifikasi visual per section** (export → bandingkan dengan referensi),
   nama file export unik.

**Ready:** state inspection, canvas design via `exec`, visual QA via `export`
(+ `visual_diff.py`), ads formula, design-to-design, code-to-design (React +
Tailwind), design-to-code.

## Context — tidak ada argumen file

MCP Penpot selalu bekerja pada **file + page yang sedang fokus** di tab Penpot
yang terkonek plugin (File → MCP Server → Connect). Jadi:

- Selalu jalankan `penpot-cli state` dan **konfirmasi ke user** file/page yang
  terbaca memang target yang dimaksud. Jangan menebak.
- Berpindah page dari kode boleh (`penpot.openPage(page)`), tapi umumkan dulu.
- Penpot multiplayer: dokumen bisa berubah saat kamu bekerja. Re-read saat
  terasa stale, jangan re-create, jangan pernah mengubah pekerjaan user.

## CLI

| Command | Fungsi |
|---|---|
| `penpot-cli doctor` | daemon → sesi MCP → plugin (file/page fokus) |
| `penpot-cli state [--depth N]` | file, page aktif, pages, root boards, seleksi, library |
| `penpot-cli overview [--cached]` | Penpot High-Level Overview (aturan API resmi) |
| `penpot-cli api <Type> [member]` | Dokumentasi Plugin API (`Board`, `FlexLayout`, `Text`, `TokenCatalog`…) |
| `penpot-cli exec '<js>'` · `-f file.js` · `-` | Jalankan JS (execute_code); return → stdout, console → stderr |
| `penpot-cli upload <img\|dir>... [--base D] [--max 1400]` | Upload bertahap + resize + manifest (skip yang sudah ada; pulih setelah tab reload) → `storage.media[key]` |
| `penpot-cli export <id\|selection\|page> --out f.png` | Render PNG/SVG untuk QA |
| `penpot-cli snapshot <id\|selection> [--out f.json]` | Pohon shape mendalam (layout, teks, komponen, tokens) |
| `penpot-cli reference import <img>` / `clone <id>` | Board referensi terkunci |
| `penpot-cli component list` / `create <id> --name` | Komponen local library |
| `penpot-cli codegen bundle <id> --out DIR` | Evidence design-to-code |
| `penpot-cli source scan <path>` · `project inspect <path>` | Primitive offline code↔design |
| `penpot-cli app [--file ID [--page ID]] \| --status \| --quit` | **Cara wajib membuka Penpot** (Workflow 0): jendela Chrome khusus, anti-throttling. Tidak membuka jendela kedua; `--quit` menutup prosesnya |
| `penpot-cli sync` | **Bukti perubahan tersimpan ke server** (revn naik). Jalankan tiap slide/section & sebelum export QA |
| `penpot-cli backup save "<label>"\|list\|restore <id>` | Snapshot file lewat REST (instan). Tidak bergantung tab |
| `penpot-cli pin [--show\|--clear]` | **Kunci file target**: exec ditolak kalau user pindah file di tab |
| `penpot-cli settle <id\|selection>` | Reflow board flex auto setelah build manual |
| `penpot-cli tokens list\|export [--out f.json]` | Design tokens (W3C DTCG) |
| `penpot-cli library status\|list\|publish\|unpublish\|link\|unlink` | Shared Library lewat REST (butuh `PENPOT_PERSONAL_TOKEN`) |
| `penpot-cli tools` · `call <tool> '{json}'` | Escape hatch MCP mentah |
| `penpot-cli daemon start\|stop\|status\|reconnect` | Kontrol daemon |
| `penpot-cli --skills [name\|--all\|--list]` | Skill ini (embedded di binary) |
| `python3 <repo-penpot-cli>/scripts/pen2penpot.py` | Porter node `.pen` → Penpot (lihat Workflow H) |

Semua command punya `--help`. Flag umum: `--json`, `--timeout 120s`.

## Required reading

| Read | Before | Why |
|---|---|---|
| this SKILL.md | any Penpot work | Aturan inti + workflow |
| `penpot-cli overview` (sekali per sesi) | Workflow B/E/F | Aturan resmi server: shape, layout, text, utils, library, tokens |
| [references/limits-and-gotchas.md](references/limits-and-gotchas.md) | **sebelum kerja apa pun** | Limit terukur (100 KB/15 MB/120 s), page aktif, layout settle, font, komponen |
| [references/plugin-api-rules.md](references/plugin-api-rules.md) | Workflow B/E/F/H | Pola `exec` yang teruji + helper |
| [references/autolayout-antipatterns.md](references/autolayout-antipatterns.md) | build besar | Jebakan layout + checklist per section |
| [references/design-playbooks.md](references/design-playbooks.md) | landing/app/slides/table | Struktur per artefak |
| [references/style-presets.md](references/style-presets.md) | tanpa brand | Arah visual siap pakai |
| `penpot-cli api <Type>` | properti yang belum yakin | Jangan menebak API |

## Key canvas rules

- Konfirmasi file/page fokus sebelum menulis (`state`).
- Board root baru diberi prefix **`WIP — `** selama dikerjakan, hapus saat
  selesai. Board layar: `clipContent = true`. Root page cuma berisi board
  (screen/komponen/referensi) — jangan taruh teks/ikon lepas di root.
- Bangun per section kecil; verifikasi tiap section begitu selesai
  (`export` + checklist), jangan nunggu akhir.
- Perbaiki dengan **mengubah shape yang ada** — jangan delete-and-rebuild.
- Layout: flex (`addFlexLayout`) / grid; sizing `fix|auto|fill` lewat
  `layoutChild`. Children di board flex: urutan `appendChild` = urutan visual.
- Teks: `growType` `auto-width`/`auto-height`; `resize()` membuat teks
  `fixed` — set ulang growType. Jangan menebak dimensi teks.
- Warna hex huruf besar (`#FF5533`). `fills`/`strokes` diganti seluruh array.
- UI berulang → komponen (`component create`) + instance (`component.instance()`).
- Simpan id penting di `storage` (persisten lintas `exec` dalam sesi daemon).
- Gambar lokal → `penpot-cli upload`, jangan base64 di dalam `exec` (HTTP 413).
- Porting dari .pen: rotasi Penpot searah jarum jam → balik tanda.
- Logo/ilustrasi: jangan digambar manual dari path. Pakai aset (upload
  gambar) atau placeholder berlabel; ikon via SVG (`penpot.createShapeFromSvg`).
- Hindari bungkus tiap elemen dalam card; gradient/shadow seperlunya.

## Operating rules

1. `penpot-cli state` sebelum writes; verifikasi file/page/board target, lalu
   `penpot-cli pin`. Error `[pin] file fokus … ≠ …` → user pindah file: STOP,
   minta user kembali ke file target (jangan `PENPOT_PIN=off` tanpa izin).
2. Multi-step mutations: sampaikan rencana (page + board + section), minta
   konfirmasi, bangun per section kecil yang reversible.
3. Satu perintah penpot-cli dalam satu waktu (daemon memang serial).
4. "Plugin tab suspended", timer lambat, atau teks baru terukur 1×1 → Penpot
   dibuka di browser biasa yang menidurkan tab. Minta user pakai **`penpot-cli
   app`** (lihat Workflow 0). Keepalive daemon + tunggu 3 menit hanya penolong
   untuk tab biasa. Plugin tidak konek → File → MCP Server → Connect.
5. Timeout (120 s server) → sesi tetap hidup, **tapi kode bisa sudah jalan**:
   cek `state`/`snapshot` sebelum mengulang (hindari shape dobel). Bersihkan
   shape `tmp-*` sisa eksperimen.
6. `exec` gagal (exception) → seluruh kode tidak di-rollback otomatis: cek apa
   yang sudah tercipta, lalu lanjutkan/perbaiki — jangan kirim ulang buta.
7. Jangan ubah board yang tidak terkait, jangan hapus pekerjaan lama, jangan
   bikin aset workspace-wide tanpa persetujuan.

## Workflow 0 — Buka Penpot (jendela khusus, opsi 3)

Teruji Okt 2026 dengan jendela disembunyikan (Cmd+H), tes timer 5×60 ms + ukur
teks "Hello":

| Browser | Tersembunyi | Hasil |
|---|---|---|
| Tab Chrome biasa | 100 ms → 0,8–17 s; teks 1×1 | ✗ build macet/timeout |
| Safari | tidak menjawab setelah ±30 s | ✗ |
| **`penpot-cli app`** (Chrome profil terpisah + anti-throttling) | 305–308 ms, teks 31×17, 10 menit | ✓ |

1. `penpot-cli app --file <fileId> [--page <pageId>]` — buka jendela khusus
   (profil `~/.penpot-chrome`, tanpa address bar). Sekali per profil: login
   (Google login boleh). Tidak membuka jendela kedua kalau sudah jalan.
2. Di jendela itu: File → MCP Server → Connect (titik MCP hijau).
3. `penpot-cli doctor` → harus `plugin: connected` di file yang benar.
4. Pastikan Penpot **tidak** terbuka juga di tab/jendela lain (dua sesi; MCP
   hanya di satu, bisa membingungkan target).
5. Selesai: `penpot-cli app --quit` (menutup jendela terakhir di macOS tidak
   menghentikan proses profil). `penpot-cli app --status` untuk cek.

Batasan: domain belum terkunci ketat — Google Chrome mengabaikan
`--load-extension`, jadi link di dalam Penpot masih bisa membuka situs lain.
CLI hanya menolak URL selain domain Penpot saat membuka.

## Workflow A — Context-first

1. Penpot dibuka lewat `penpot-cli app` (Workflow 0). `penpot-cli doctor` →
   `penpot-cli state` → konfirmasi target ke user →
   **`penpot-cli pin`** (kunci file; doctor menampilkan ✓/✗).
2. `penpot-cli overview` (sekali per sesi) + baca referensi relevan.
3. Baru rencanakan tugas.

## Workflow B — Canvas design (`exec`)

Baca overview + [plugin-api-rules.md](references/plugin-api-rules.md) dulu.

1. Board root: `penpot.createBoard()`, nama `WIP — <Screen>`, posisi di
   ruang kosong (lihat pola `freeSpot` di plugin-api-rules), `clipContent`.
2. Token/warna/typography dulu bila multi-screen (library local / tokens —
   cek skema via `penpot-cli api TokenCatalog`).
3. Primitives → komponen → section → komposisi. Satu section per `exec`
   (pakai `-f file.js` untuk kode panjang), simpan id ke `storage`.
4. Tiap section: `penpot-cli export <id> --out /tmp/x.png` → lihat → checklist
   [autolayout-antipatterns](references/autolayout-antipatterns.md).
5. Selesai: hapus prefix `WIP — `.

## Workflow C — Visual QA bridge (canvas ↔ live site)

Penpot tidak punya browser terintegrasi (beda dengan pen.dev). Jembatannya:

1. Screenshot live/localhost via browser tool (agent-headless-browser /
   chrome-devtools) di viewport yang sama dengan board.
2. Opsional taruh di canvas: `penpot-cli reference import shot.png --x … --y …`
   (terkunci, berdampingan dengan desain).
3. `penpot-cli export <board> --out /tmp/design.png` lalu
   `python3 <repo-penpot-cli>/scripts/visual_diff.py /tmp/live.png /tmp/design.png --out /tmp/qa.png`
   (Reference | Penpot | Difference).
4. Bandingkan placement, spacing, warna, copy → perbaiki kode/desain → ulangi.

## Workflow D — Ads guidelines (teruji)

Formula pemenang Meta ads, safe zones story (y=260–1610), warna semantik,
landing 7-section: [references/landing-formula.md](references/landing-formula.md).
Kalau project punya konvensi sendiri (export, penamaan, dll.), ikuti `AGENTS.md` project itu.

## Workflow E — Design-to-design

**Baca [references/design-to-design.md](references/design-to-design.md) in full.**
Kontrak: referensi terkunci (`reference import`/`clone`), rebuild
section-per-section dengan `exec`, komponen untuk pola berulang, checklist
verifikasi, QA side-by-side (`export` + `visual_diff.py`).

## Workflow F — Code-to-design (React/Tailwind → canvas)

**Baca [references/code-to-design-react-tailwind.md](references/code-to-design-react-tailwind.md).**

1. Evidence: `penpot-cli source scan ./src --out /tmp/ui.json` + baca route,
   komponen, tailwind config; screenshot route yang jalan.
2. Design IR ([template](references/code-to-design-ir-template.json)):
   route → board; komponen source → komponen Penpot; theme → tokens;
   props/state → variants. Abaikan wrapper non-visual.
3. Build di page khusus `Code → Design — <feature>`: tokens → primitives →
   komponen → board route.
4. Verifikasi: screenshot route vs `export` board (`visual_diff.py`).
5. Guardrails: jangan infer secrets/user data; region dinamis → state statis
   representatif; tanya sebelum bikin library workspace-wide.

## Workflow G — Design-to-code

**Baca [references/design-to-code.md](references/design-to-code.md).**

1. `penpot-cli project inspect .` — ikuti konvensi repo target.
2. `penpot-cli codegen bundle <board> --out /tmp/bundle` → baca `design.json`
   dulu; markup/CSS hanya evidence.
3. Peta implementasi (komponen/variant/token/aset) → implement per slice.
4. Verifikasi: render localhost di viewport target → Workflow C.

## Workflow H — Porting `.pen` (pen.dev) → Penpot

Teruji: design system (±560 node, 22 komponen), story & carousel,
deck 16 slide 1920×1080 (1.211 node, 2–16 s/slide).

1. **Konfirmasi tujuan**: file Penpot (yang terkonek plugin) + page per artefak.
   Contoh konvensi page: DS di page **"DS — Design System"**,
   story di page **"Story Ads"** (jangan ganti), carousel di page
   **"Feed 4:5 · Carousel"** (satu baris per carousel).
2. **Aset**: daftar gambar yang dibutuhkan lalu upload sekali:
   ```bash
   S=<repo-penpot-cli>/scripts/pen2penpot.py
   for id in NODE1 NODE2; do python3 $S file.pen $id --media; done | sort -u > /tmp/media.txt
   penpot-cli upload $(cat /tmp/media.txt) --base . --max 1400
   ```
3. **DS dulu** (komponen harus ada sebelum instance):
   - kerangka: `python3 $S file.pen DSNODE --page "DS — …" --shallow --x 0 --y 0 --emit /tmp/ds.js` → `penpot-cli exec -f /tmp/ds.js`
   - per section: `python3 $S file.pen SECTION --into <id-board-DS> --components --emit /tmp/s.js` → exec → export → bandingkan.
   - komponen dari library `.pen` lain (imports, mis. `5:wIFJg`): `python3 $S file.pen "5:A,5:B" --section "Nama" --into … --components`.
   - susun ulang urutan section dengan `insertChild` (urutan `.pen`).
4. **Artefak** (instance otomatis dari peta `penpot-cli.pen-map`):
   - story: `python3 $S file.pen STORYNODE --page "Story Ads" --x X --y Y --emit /tmp/st.js` → exec.
   - carousel (≤ 6 slide, 1 exec): `python3 $S file.pen S1,S2,S3,S4,S5 --row --page "Feed 4:5 · Carousel" --x 0 --y Y --emit /tmp/cr.js` → `penpot-cli exec -f /tmp/cr.js`.
     Timeout 120 s → slide mungkin sudah jadi; cek page sebelum mengulang.
5. **QA**: export tiap board → side-by-side dengan export asli (`story/`, `post/`
   atau `pencil_execute Export` dari Pen.app). Perbaiki porter, bukan menambal
   manual satu-satu, kalau polanya berulang.

Yang dipetakan porter: frame/flex (gap, padding, align, sizing fix/auto/fill),
layout none (posisi absolut), text (font persis, size, weight, lineHeight,
letterSpacing, align, growType), fill warna/alpha/gradient/image, stroke,
shadow, radius per sudut, opacity, hidden, rotasi (dibalik, diterapkan
terakhir), icon lucide (SVG), reusable → komponen (non-board dibungkus board),
ref → instance + override `descendants` (content/fontSize/fontWeight/fill/enabled/width),
`$variable` diresolve. Belum: replace descendant (`type` di override), path
kompleks, ellipse innerRadius/sweep, note/prompt.

## Workflow J — Carousel/slide langsung di Penpot (tanpa .pen)

Teruji: carousel 5 slide 1080×1350 (contoh netral lengkap:
`skills/scripts/carousel/example-linkedin/`). Helper: `skills/scripts/carousel/lib.js`
(`BOX`, `TXT`, `ICON`, `CHIP`, `MARK`, `SLIDE`, `SETTLE`, `ROT`); aset:
`make_assets.py`. Jalankan dengan `cat lib.js slide.js | penpot-cli exec -`.

1. Konfirmasi page tujuan + `penpot-cli pin`; `penpot-cli backup save "…"`.
2. Aset: `penpot-cli upload` gambar bila perlu; ikon lucide →
   `python3 make_assets.py [--icons a,b,c] > assets.json`
   lalu `storage.liAssets = JSON.stringify(ASSETS)` (komentar `<!-- -->` SVG sudah dibuang).
3. **Satu slide per exec**, kerangka dulu, isi bertahap; slide padat dipecah per
   kartu/bagian. Target < 30 s per exec. Semua kotak lebar **fix**; teks `fill`
   hanya di board lebar fix/fill (lihat gotchas).
4. Tiap slide: `penpot-cli sync` (harus OK) → export → lihat → cek `bottom` isi <
   footer (1270 untuk 1350). Export yang hash-nya sama dengan sebelumnya = perubahan
   belum tersimpan, bukan "tidak berubah".
5. Marker miring: `MARK` + `ROT` (kunci ukuran lalu putar).
6. Ekspor akhir (contoh konvensi): JPEG q85 scale 1 per slide + satu PDF
   multi-halaman untuk LinkedIn (Pillow `save_all=True`).

## Workflow I — Design system: tokens, variants, shared library

1. **Tokens**: set "Brand Kit" (warna palet + alias `color.brand → {color.terracotta}`,
   radius, font) + theme `brand/default`. Pasang ke main component:
   `applyToken(t, ['fill'])`, font `['fontFamily']`, radius 4 sudut. Ubah nilai
   → toggle set 2× → semua instance ikut.
2. **Variants**: gabungkan komponen sejenis (Bubble In/Out → "Bubble",
   properti `Arah`). `penpot-cli backup save` dulu, **satu exec khusus**, verifikasi di exec
   berikutnya (operasi berat; bisa memutus plugin sementara). Instance lama
   tetap tersambung ke komponen variannya.
3. **Shared Library**: konfirmasi user dulu (file jadi terlihat di seluruh
   team) → `penpot-cli library publish` → di file lain `penpot-cli library link <id>`.

## Penpot vs pen.dev — kapan pakai yang mana

| | pen.dev | Penpot |
|---|---|---|
| Eksekusi | MCP lokal, deklaratif (JSON node) | remote → plugin di tab, imperatif (Plugin API) |
| Komponen | `reusable` + `ref` per file; tidak lintas file (harus copy) | library komponen; **Shared Library** lintas file/tim; perubahan main menyebar |
| Variants | tidak ada sistem variant (pakai komponen terpisah / theme axis) | Variants native (properti × nilai, `switchVariant`, swap UI) |
| Token | variables + themes (`$var`, axis) | Design Tokens (W3C-ish): sets, themes, referensi `{color.primary}`, `applyToken` per properti |
| Kecepatan agent | cepat; layout benar sekali deklarasi | 3–5× lebih lambat; butuh settle, aset harus upload |
| Pilih | eksplorasi, iterasi cepat, export kreatif | DS bersama tim, handoff dev, kolaborasi web |

Alur yang disarankan: desain/iterasi di pen.dev → porting (Workflow H) ke
Penpot untuk DS & aset tim.

## References

- [references/limits-and-gotchas.md](references/limits-and-gotchas.md) — limit terukur & jebakan (WAJIB)
- [references/plugin-api-rules.md](references/plugin-api-rules.md) — pola exec teruji, helper, hapus aman
- [references/autolayout-antipatterns.md](references/autolayout-antipatterns.md) — jebakan layout + checklist
- [references/design-to-design.md](references/design-to-design.md) — kontrak screenshot → hi-fi
- [references/code-to-design-react-tailwind.md](references/code-to-design-react-tailwind.md) — React/Tailwind → Penpot
- [references/design-to-code.md](references/design-to-code.md) — Penpot → repo
- [references/design-playbooks.md](references/design-playbooks.md) — per artefak
- [references/style-presets.md](references/style-presets.md) — 6 preset gaya
- [references/landing-formula.md](references/landing-formula.md) — formula landing 7-section & iklan (generik)
- `penpot-overview.md` — snapshot High-Level Overview (`penpot-cli overview --cached`)

## Status

- **v2.6**: Workflow 0 + `penpot-cli app [--status|--quit]` (jendela Chrome khusus
  anti-throttling, cegah jendela ganda); teruji Chrome biasa / Safari / Chrome+flag.
- **v2.5**: `penpot-cli backup` (snapshot REST), `penpot-cli sync` (bukti simpan ke
  server) + doctor menampilkan umur simpan; diagnosis antrean simpan macet;
  gotcha teks `fill` di board `auto`; carousel LinkedIn 1080×1350 langsung di Penpot.
- **v2.4**: `pin` (kunci file target), `settle`, `tokens list|export`, exec
  baca stdin tanpa `-`; porter: path stroke (panah/konektor), `__rotate`
  (board auto miring tidak membengkak).
- **v2.3**: `penpot-cli library` (Shared Library via REST), Workflow I
  (tokens/variants/library) dengan temuan nama properti token yang benar.
- **v2.2**: keepalive daemon (tab tidak tidur selama agent kerja), porter:
  rotasi relatif top-down, fill sumbu utama, `--row` carousel 1 exec;
  `waitForLayoutUpdate` diuji & ditolak; tabel Penpot vs pen.dev.
- **v2.1**: limit terukur + auto-workaround di CLI (exec chunked >90 KB, upload
  bertahap + manifest, tunggu tab suspended); Workflow H porting `.pen`;
  aturan wajib page aktif / hapus aman / font persis / settle layout.
- **v2.0**: transport = penpot-cli daemon (persistent MCP session); struktur
  workflow A–G diselaraskan dengan skill `pen`.
