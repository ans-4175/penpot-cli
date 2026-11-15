# penpot-cli — catatan pengembang

> Ringkasan untuk pengguna ada di [README](../README.md). Dokumen ini: arsitektur, limit terukur, dan orientasi kode.

CLI untuk Penpot MCP (`design.penpot.app/mcp/stream`) dengan arsitektur
daemon + client. Saudara dari CLI untuk pen.dev: konsep sama,
transport beda.

```
penpot-cli (thin client)        penpot-cli serve (daemon)            Penpot MCP (remote)
      │                                │                                     │
      │── unix socket ───────────────▶ │ ── HTTPS keep-alive ──────────────▶ │ ──▶ plugin di tab
      │  ~/.penpot-cli/daemon.sock     │    Mcp-Session-Id persisten         │     browser Penpot
```

- Handshake (`initialize` → `notifications/initialized` → `tools/list`)
  **sekali**, malas (lazy) pada request pertama. Daftar tools di-cache.
- `Mcp-Session-Id` + koneksi TLS dipakai ulang → panggilan berikutnya ±0,3 s
  (vs ±1,5 s per command di CLI lama yang handshake ulang tiap kali).
- Sesi hilang di server (HTTP 400/404) → **re-initialize otomatis** sekali, lalu
  request diulang.
- Panggilan MCP **diserialisasi** (satu dalam satu waktu).
- Timeout hanya membatalkan request itu — sesi tetap hidup. (Catatan: kode yang
  sudah terkirim ke plugin bisa tetap jalan.)
- `storage` Penpot (objek persisten di plugin) ikut bertahan lintas command.
- Daemon berhenti sendiri setelah idle 15 menit / SIGTERM; saat berhenti
  mengirim `DELETE` untuk menutup sesi di server.
- Client auto-start daemon kalau socket mati/stale.
- **Keepalive**: selama ada request agent dalam 10 menit terakhir, daemon
  mengirim `return 1` tiap 25 s supaya tab plugin tidak di-suspend browser
  (Chrome Memory Saver). `PENPOT_KEEPALIVE=0` mematikan, `=20s` mengubah interval.
- Skill penpot di-embed di binary (`penpot-cli --skills`).
- Limit server terukur: body request ±100 KB (413), respons 15 MB, eksekusi
  120 s. Workaround otomatis: `exec` > 90 KB dipecah & dirakit di `storage`
  lalu `eval`; `upload` bertahap + resize (`--max`) + manifest di plugin data
  file (skip duplikat, pulih setelah tab reload); tab plugin di-suspend →
  tunggu s.d. 3 menit lalu ulang (aman: server menolak sebelum eksekusi).
  Detail: `skills/references/limits-and-gotchas.md`.

### Beda dengan pen-cli

| | pen-cli (pen.dev) | penpot-cli (Penpot) |
|---|---|---|
| MCP | binary lokal di Pen.app, stdio | remote streamable HTTP + SSE |
| "Persisten" | child process hidup | Mcp-Session-Id + keep-alive + re-init |
| Konteks | argumen `<file.pen>` tiap command | file/page **fokus** di tab yang terkonek plugin |
| Retry quirk | "wrong .pen file" | sesi kedaluwarsa (400/404) |
| Error tool | `isError` | juga teks `Tool execution failed:` tanpa `isError` → dinormalisasi |
| QA visual | browser terintegrasi Pen.app | `export` + `scripts/visual_diff.py` |

## Prasyarat

1. Akun Penpot → **Your account → Integrations → MCP Server**: aktifkan, salin key.
2. Tulis config `~/.config/ans/penpot` (chmod 600):
   ```
   PENPOT_DOMAIN=design.penpot.app
   PENPOT_MCP_TOKEN=<key>
   ```
   Alternatif: env `PENPOT_MCP_TOKEN`/`PENPOT_DOMAIN`, atau `PENPOT_MCP_URL`
   (URL penuh, mis. MCP lokal `http://localhost:4401/mcp`).
3. Opsional (untuk `library`, `backup`, `sync`, `app --file`): tambahkan
   `PENPOT_PERSONAL_TOKEN=<token>` (Your account → Access tokens).
4. Buka Penpot dengan **`penpot-cli app --file <fileId>`** — jendela Chrome
   khusus (profil `~/.penpot-chrome`) dengan flag anti-throttling, supaya plugin
   tetap jalan walau jendela disembunyikan. Login sekali, lalu **File → MCP
   Server → Connect**. Tutup dengan `penpot-cli app --quit`.
   (Tab Chrome biasa melambat dan Safari berhenti total saat tersembunyi — teruji.)
5. `penpot-cli doctor`.

Token tidak pernah dicetak: log/output memakai endpoint tanpa query string.

## Build

```sh
cd <repo-penpot-cli>
gofmt -w . && go vet ./... && go test ./... && go build -o ~/.local/bin/penpot-cli .
```

Go 1.23, stdlib saja (zero dependency). Setelah mengubah daemon: `penpot-cli daemon stop`
supaya binary baru yang dipakai.

Rilis: push tag `vX.Y.Z` → GitHub Actions (`.github/workflows/release.yml`) build
6 target (darwin/linux/windows × amd64/arm64), bundel `SHA256SUMS.txt`, lalu publish
GitHub Release. Versi diinjeksi lewat `-ldflags "-X main.version=..."`.
Kode khusus OS: `daemon_unix.go` / `daemon_windows.go`; `upload --max` hanya macOS (sips). `app` jalan di macOS/Linux/Windows (Linux & Windows belum teruji manual).

## Pakai

```sh
penpot-cli doctor                         # daemon → MCP → plugin (file/page fokus)
penpot-cli state [--depth 2]              # file, pages, root boards, seleksi, library
penpot-cli overview                       # aturan resmi API (sekali per sesi kerja)
penpot-cli api FlexLayout dir             # dokumentasi Plugin API

penpot-cli exec 'return penpotUtils.shapeStructure(penpot.root, 2)'
penpot-cli exec -f build-hero.js          # kode panjang
cat x.js | penpot-cli exec -              # stdin

penpot-cli upload ~/img/hero.png --key hero    # → storage.media['hero'] (bertahap)
penpot-cli upload assets --base . --max 1400   # folder; key = path relatif, skip yang sudah ada
penpot-cli export <id|selection|page> --out /tmp/hero.png
penpot-cli snapshot <id> --out /tmp/board.json

penpot-cli reference import shot.png --name 'Reference — LP' --x -1600
penpot-cli reference clone selection
penpot-cli component list
penpot-cli component create <id> --name 'Button / Primary'
penpot-cli codegen bundle <board-id> --out /tmp/bundle

penpot-cli source scan ./src --out /tmp/ui.json   # offline
penpot-cli project inspect .                      # offline

penpot-cli tools
penpot-cli call penpot_api_info '{"type":"Board"}'

penpot-cli daemon start|stop|status|reconnect
penpot-cli --skills [name|--all|--list]
penpot-cli skills --sync                  # refresh skills/penpot-overview.md dari server
```

Contoh `daemon status`:

```
daemon: pong pid=31497 uptime=2m calls=14 endpoint=https://design.penpot.app/mcp/stream session=c59c7a0a… server="penpot 1.0.0" tools=4 inits=1
```

`inits=1` = sesi belum pernah dibuat ulang.

## Layout

| Lokasi | Isi |
|---|---|
| `<repo-penpot-cli>` | Source |
| `skills/` | Skill penpot (SKILL.md workflow A–G + references) — di-embed via go:embed, juga di-symlink ke `~/.agents/skills/penpot` |
| `scripts/visual_diff.py` | Lembar QA Reference \| Penpot \| Difference (butuh Pillow) |
| `~/.local/bin/penpot-cli` | Binary |
| `~/.penpot-cli/` | Runtime: `daemon.sock`, `daemon.pid`, `daemon.log` |
| `~/.config/ans/penpot` | Config token (dibagi dengan CLI `penpot` lama) |

## Orientasi kode

`main.go` → `client.go` → `daemon.go` → `mcp.go`, plus `workflows.go`,
`local.go`, `skills.go`.

- **`main.go`** — dispatch subcommand + `usage()`.
- **`client.go`** — `callDaemon` (satu request newline-JSON ke socket),
  `dialDaemon`/`startDaemon` (spawn `serve` detached via `Setsid`),
  `parseArgs` (flag di posisi mana pun), `runCode`/`printCode` (bongkar
  envelope `{"result","log"}` execute_code), `daemon`/`doctor`/`tools`/`call`,
  teks `--help`.
- **`daemon.go`** — server unix socket: single-instance, idle watchdog,
  SIGTERM, tool internal `ping`/`tools`/`reconnect`, sisanya diteruskan ke
  `callTool`. Cleanup: DELETE sesi, hapus sock+pid.
- **`mcp.go`** — sesi streamable HTTP: `resolveEndpoint` (config/env),
  `post` (User-Agent wajib — tanpa UA server membalas 403), `readSSE`
  (ReadString, aman untuk baris base64 multi-MB), `connect`, `callTool`
  (serial, re-init saat 400/404, normalisasi "Tool execution failed:").
- **`workflows.go`** — wrapper berbasis execute_code: state, overview, api,
  exec, export, snapshot, reference, component, codegen.
- **`local.go`** — `source scan` & `project inspect` (offline, port dari CLI lama).
- **`media.go`** — `upload`: resize (sips), base64 bertahap → `storage.media[key]`, manifest `penpot-cli.media`.
- **`keepalive.go`** — ping berkala selama agent aktif (tab tidak tidur).
- **`chunk.go`** — `exec` di atas batas request → potongan → `eval` (meneruskan `storage`).
- **`skills/scripts/pen2penpot.py`** — porter `.pen` → Penpot (embedded; `scripts/` = symlink).
- **`skills.go`** — embed `skills/`, `--skills`, `skills --sync`.
- **`mcp_test.go`** — fake MCP server: reuse sesi, re-init, isError, SSE besar, parser flag.

## Porting .pen → Penpot

```sh
S=<repo-penpot-cli>/scripts/pen2penpot.py      # atau: penpot-cli --skills --porter /tmp/p.py
python3 $S file.pen NODE --media                      # daftar gambar
python3 $S file.pen DSNODE --page "DS" --shallow --x 0 --y 0 --emit /tmp/a.js
python3 $S file.pen SECTION --into <boardId> --components --emit /tmp/b.js
python3 $S file.pen STORY --page "Story Ads" --x 4660 --y 1100 --emit /tmp/c.js
penpot-cli exec -f /tmp/c.js
```
Lihat Workflow H di `skills/SKILL.md`.

## Riwayat

Menggantikan CLI `penpot` lama (`~/.local/bin/penpot`, `~/src/penpot-cli`) yang
sudah dihapus — semua kemampuannya diport ke sini.
