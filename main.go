// penpot-cli — daemon + client untuk Penpot MCP (remote streamable HTTP).
//
// Daemon (`serve`) memegang satu persistent MCP session: initialize +
// tools/list sekali, Mcp-Session-Id + koneksi keep-alive dipakai ulang,
// re-initialize otomatis kalau sesi hilang. Melayani client lewat unix
// socket sampai idle 15 menit. Client (`penpot-cli <cmd>`) tipis: konek ke
// socket, auto-start daemon kalau belum jalan.
package main

import (
	"fmt"
	"os"
)

// version diisi saat rilis: -ldflags "-X main.version=<tag>".
var version = "2.6.2"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	argv := os.Args[2:]
	var err error
	switch os.Args[1] {
	case "serve": // internal — dipanggil auto-start
		err = runDaemon()
	case "daemon":
		err = cmdDaemon(argv)
	case "doctor":
		err = cmdDoctor(argv)
	case "state":
		err = cmdState(argv)
	case "overview":
		err = cmdOverview(argv)
	case "api":
		err = cmdAPI(argv)
	case "exec":
		err = cmdExec(argv)
	case "upload":
		err = cmdUpload(argv)
	case "export":
		err = cmdExport(argv)
	case "snapshot":
		err = cmdSnapshot(argv)
	case "reference":
		err = cmdReference(argv)
	case "component":
		err = cmdComponent(argv)
	case "codegen":
		err = cmdCodegen(argv)
	case "app":
		err = cmdApp(argv)
	case "sync":
		err = cmdSync(argv)
	case "backup":
		err = cmdVersion(argv)
	case "pin":
		err = cmdPin(argv)
	case "settle":
		err = cmdSettle(argv)
	case "tokens":
		err = cmdTokens(argv)
	case "library", "lib":
		err = cmdLibrary(argv)
	case "source":
		err = cmdSource(argv)
	case "project":
		err = cmdProject(argv)
	case "tools":
		err = cmdTools(argv)
	case "call":
		err = cmdCall(argv)
	case "--skills", "--skill", "skills":
		err = cmdSkills(argv)
	case "version", "--version", "-v":
		fmt.Println("penpot-cli", version)
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "penpot-cli: perintah tidak dikenal %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "penpot-cli: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Print(`penpot-cli ` + version + ` — CLI untuk Penpot MCP (persistent session)

Penpot MCP bekerja pada file + page yang sedang FOKUS di tab Penpot yang
terkonek plugin MCP (File → MCP Server → Connect). Tidak ada argumen file.

Usage:
  penpot-cli doctor                     Cek daemon + sesi MCP + plugin (file/page fokus)
  penpot-cli state                      File, page aktif, pages, boards, seleksi, library
  penpot-cli overview                   Penpot High-Level Overview (aturan API, wajib baca)
  penpot-cli api <Type> [member]        Dokumentasi Plugin API (Board, FlexLayout, Text, ...)
  penpot-cli exec [js | -f file.js | -] Jalankan JavaScript di plugin (execute_code)
  penpot-cli upload <image> [--key K]   Upload gambar (bertahap) → storage.media[K]
  penpot-cli export <id|selection|page> [--format png|svg] [--mode shape|fill] [--out F]
  penpot-cli snapshot <id|selection>    Pohon shape mendalam (JSON) untuk analisis/QA
  penpot-cli reference import <img>     Board referensi terkunci dari PNG/JPEG/GIF lokal
        [--name N] [--x 0] [--y 0]
  penpot-cli reference clone <id|selection> [--name N] [--x X] [--y Y]
  penpot-cli component create <id|selection> [--name N]
  penpot-cli component list             Komponen local library
  penpot-cli codegen bundle <id|selection> --out DIR   design.json + markup/CSS evidence
  penpot-cli app [--file ID [--page ID]]   jendela Chrome khusus Penpot (anti-throttling)
  penpot-cli sync                          bukti perubahan tersimpan ke server (revn naik)
  penpot-cli backup save "<label>"|list|restore <id>    snapshot file via REST (tidak bergantung tab)
  penpot-cli pin [--show|--clear]         kunci file target (exec ditolak kalau user pindah file)
  penpot-cli settle <id|selection>        reflow board flex auto (setelah build manual)
  penpot-cli tokens list | export [--out f.json]   design tokens (W3C DTCG)
  penpot-cli library status|list|publish|unpublish|link|unlink   Shared Library (REST, PERSONAL_TOKEN)
  penpot-cli source scan <path> [--out F]  React/Tailwind inventory (offline)
  penpot-cli project inspect <path>     Konvensi repo target (offline)
  penpot-cli tools                      Daftar tools MCP live (cache sesi)
  penpot-cli call <tool> [json-args]    Escape hatch: panggil tool MCP apa pun
  penpot-cli daemon start|stop|status|reconnect
  penpot-cli --skills [name|--all|--list] Skill penpot embedded (default SKILL.md)
  penpot-cli --skills --porter [path]   Tulis porter pen2penpot.py (embedded) ke path
  penpot-cli skills --sync              Refresh skills/penpot-overview.md dari MCP

Flag umum: --json (output mentah), --timeout 120s.
Env: PENPOT_KEEPALIVE=0|20s (keepalive tab), PENPOT_SUSPEND_WAIT=3m.
Setiap subcommand punya help sendiri: penpot-cli <command> --help

Daemon start otomatis pada command pertama dan berhenti sendiri setelah
idle 15 menit. Config: ~/.config/ans/penpot (PENPOT_DOMAIN, PENPOT_MCP_TOKEN)
atau env PENPOT_MCP_URL. Runtime: ~/.penpot-cli/ (daemon.sock/.pid/.log)
`)
}
