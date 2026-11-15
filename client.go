package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func ctxTimeout(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}

// ---- client: satu request ke daemon, auto-start kalau perlu ----

func callDaemon(tool string, args map[string]any, timeout time.Duration) (*cliResp, error) {
	conn, err := dialDaemon()
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(timeout + 20*time.Second))

	data, _ := json.Marshal(cliReq{Tool: tool, Args: args, TimeoutSec: int(timeout.Seconds())})
	if _, err := conn.Write(append(data, '\n')); err != nil {
		return nil, err
	}
	line, err := bufio.NewReaderSize(conn, 64<<10).ReadBytes('\n')
	if err != nil && len(line) == 0 {
		return nil, errors.New("daemon menutup koneksi tanpa jawaban — cek " + logPath())
	}
	var resp cliResp
	if err := json.Unmarshal(line, &resp); err != nil {
		return nil, fmt.Errorf("jawaban daemon tidak valid: %w", err)
	}
	if resp.Error != "" {
		return nil, errors.New(resp.Error)
	}
	return &resp, nil
}

// callTool = callDaemon untuk tool MCP; isError → error berisi teksnya.
// Tab plugin suspended → server menolak SEBELUM eksekusi, jadi aman diulang:
// tunggu (beri tahu user) sampai maks suspendWait, lalu coba lagi.
func callTool(tool string, args map[string]any, timeout time.Duration) (*toolResult, error) {
	deadline := time.Now().Add(suspendWait)
	for warned := false; ; {
		res, err := callToolOnce(tool, args, timeout)
		if err == nil || !isSuspended(err) || time.Now().After(deadline) {
			return res, err
		}
		if !warned {
			fmt.Fprintf(os.Stderr, "[penpot-cli] tab Penpot di-suspend browser — fokuskan tab Penpot; menunggu s.d. %s…\n", suspendWait)
			warned = true
		}
		time.Sleep(4 * time.Second)
	}
}

var suspendWait = func() time.Duration {
	if d, err := time.ParseDuration(os.Getenv("PENPOT_SUSPEND_WAIT")); err == nil {
		return d
	}
	return 3 * time.Minute
}()

func isSuspended(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "suspended by the browser")
}

func callToolOnce(tool string, args map[string]any, timeout time.Duration) (*toolResult, error) {
	resp, err := callDaemon(tool, args, timeout)
	if err != nil {
		return nil, err
	}
	if resp.Result == nil {
		return &toolResult{}, nil
	}
	if resp.Result.IsError {
		return resp.Result, errors.New(hintFor(resp.Result.Text))
	}
	return resp.Result, nil
}

// hintFor menambahkan petunjuk tindakan untuk error plugin yang umum.
func hintFor(msg string) string {
	m := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(msg), "Tool execution failed:"))
	low := strings.ToLower(m)
	switch {
	case strings.Contains(low, "suspended"):
		return m + "\n→ klik/fokuskan tab Penpot di browser, lalu ulangi."
	case strings.Contains(low, "no plugin") || strings.Contains(low, "not connected") || strings.Contains(low, "no connected"):
		return m + "\n→ di tab Penpot: File → MCP Server → Connect (file yang dimaksud)."
	}
	return m
}

func dialDaemon() (net.Conn, error) {
	if c, err := net.Dial("unix", sockPath()); err == nil {
		return c, nil
	}
	if err := startDaemon(); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if c, err := net.Dial("unix", sockPath()); err == nil {
			return c, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil, errors.New("daemon tidak mau start — cek " + logPath())
}

func startDaemon() error {
	if _, err := resolveEndpoint(); err != nil {
		return err // gagal cepat dengan pesan config yang jelas
	}
	if err := os.MkdirAll(runtimeDir(), 0o700); err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	logf, err := os.OpenFile(logPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer logf.Close()
	cmd := exec.Command(exe, "serve")
	cmd.Stdout, cmd.Stderr = logf, logf
	detachDaemonProcess(cmd)
	return cmd.Start()
}

// ---- argumen ----

type opts struct {
	pos     []string
	flags   map[string]string
	json    bool
	timeout time.Duration
	help    bool
}

// parseArgs: flag boleh di mana saja (sebelum/sesudah positional).
// `--x 10`, `--x=10`; boolFlags tidak mengambil nilai. `--` = sisanya positional.
func parseArgs(argv []string, def time.Duration, boolFlags ...string) (*opts, error) {
	o := &opts{flags: map[string]string{}, timeout: def}
	isBool := map[string]bool{"json": true, "help": true}
	for _, b := range boolFlags {
		isBool[b] = true
	}
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		if a == "--" {
			o.pos = append(o.pos, argv[i+1:]...)
			break
		}
		if a == "-h" {
			o.help = true
			continue
		}
		if a == "-f" { // flag pendek: -f <file>
			if i+1 >= len(argv) {
				return nil, errors.New("flag -f butuh nilai")
			}
			i++
			o.flags["f"] = argv[i]
			continue
		}
		if !strings.HasPrefix(a, "--") || a == "-" {
			o.pos = append(o.pos, a)
			continue
		}
		name, val, hasVal := strings.Cut(strings.TrimPrefix(a, "--"), "=")
		if !hasVal && !isBool[name] {
			if i+1 >= len(argv) {
				return nil, fmt.Errorf("flag --%s butuh nilai", name)
			}
			i++
			val = argv[i]
		}
		switch name {
		case "json":
			o.json = true
		case "help":
			o.help = true
		case "timeout":
			d, err := time.ParseDuration(val)
			if err != nil {
				return nil, fmt.Errorf("--timeout: %w", err)
			}
			o.timeout = d
		default:
			if isBool[name] && !hasVal {
				val = "true"
			}
			o.flags[name] = val
		}
	}
	return o, nil
}

func (o *opts) only(allowed ...string) error {
	ok := map[string]bool{}
	for _, a := range allowed {
		ok[a] = true
	}
	for k := range o.flags {
		if !ok[k] {
			return fmt.Errorf("flag --%s tidak dikenal untuk command ini", k)
		}
	}
	return nil
}

func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		return filepath.Join(homeDir(), strings.TrimPrefix(p, "~"))
	}
	return p
}

// ---- output execute_code ----

// codeEnvelope: execute_code membalas teks JSON {"result": <return>, "log": "..."}.
type codeEnvelope struct {
	Result json.RawMessage `json:"result"`
	Log    string          `json:"log"`
}

func parseEnvelope(text string) (*codeEnvelope, bool) {
	var env codeEnvelope
	if json.Unmarshal([]byte(text), &env) != nil {
		return nil, false
	}
	return &env, true
}

// runCode menjalankan JS dan mengembalikan nilai `return`-nya (JSON mentah).
// Kode > ±90 KB otomatis dikirim bertahap (lihat chunk.go).
func runCode(code string, timeout time.Duration) (json.RawMessage, string, error) {
	code, err := prepareCode(code, timeout)
	if err != nil {
		return nil, "", err
	}
	return runCodeDirect(code, timeout)
}

func runCodeDirect(code string, timeout time.Duration) (json.RawMessage, string, error) {
	res, err := callTool("execute_code", map[string]any{"code": pinGuard(code)}, timeout)
	if err != nil {
		return nil, "", err
	}
	env, ok := parseEnvelope(res.Text)
	if !ok {
		return nil, "", fmt.Errorf("execute_code tidak mengembalikan JSON: %s", res.Text)
	}
	if len(env.Result) == 0 {
		env.Result = json.RawMessage("null")
	}
	return env.Result, env.Log, nil
}

func printJSON(raw json.RawMessage) {
	var v any
	if json.Unmarshal(raw, &v) != nil {
		fmt.Println(string(raw))
		return
	}
	if s, ok := v.(string); ok { // string → cetak apa adanya
		fmt.Println(s)
		return
	}
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(b))
}

// printCode: hasil return → stdout (pretty), console log → stderr.
func printCode(code string, o *opts) error {
	if o.json {
		code, err := prepareCode(code, o.timeout)
		if err != nil {
			return err
		}
		res, err := callTool("execute_code", map[string]any{"code": pinGuard(code)}, o.timeout)
		if err != nil {
			return err
		}
		fmt.Println(res.Text)
		return nil
	}
	raw, logs, err := runCode(code, o.timeout)
	if err != nil {
		return err
	}
	if strings.TrimSpace(logs) != "" {
		fmt.Fprintln(os.Stderr, "[console]", strings.TrimSpace(logs))
	}
	printJSON(raw)
	return nil
}

// ---- help ----

func helpOr(o *opts, text string) bool {
	if o.help {
		fmt.Print(text)
	}
	return o.help
}

const daemonHelp = `penpot-cli daemon — kontrol daemon manual

Usage:
  penpot-cli daemon start      Start daemon (biasanya tidak perlu — auto-start)
  penpot-cli daemon stop       Hentikan daemon (DELETE sesi MCP, cleanup socket+pid)
  penpot-cli daemon status     pid, uptime, sesi MCP, jumlah panggilan
  penpot-cli daemon reconnect  Paksa handshake ulang (sesi MCP baru)

Daemon berhenti sendiri setelah idle 15 menit.
Runtime: ~/.penpot-cli/ (daemon.sock, daemon.pid, daemon.log)
`

const doctorHelp = `penpot-cli doctor — cek koneksi end-to-end

Usage:
  penpot-cli doctor [--json]

Tiga lapis: daemon (auto-start) → sesi MCP (handshake, tools) → plugin
(file + page yang fokus di tab Penpot). Panggilan pertama ~1 detik
(handshake); berikutnya cepat karena sesi persisten.
`

const stateHelp = `penpot-cli state — ringkasan konteks kerja (read-only)

Usage:
  penpot-cli state [--json] [--depth N]

File + page aktif, semua page beserta board top-level, root shapes page
aktif (sampai depth N, default 1), seleksi, dan ringkasan local library
(komponen, warna, tipografi, token sets). Jalankan sebelum menulis.
`

const overviewHelp = `penpot-cli overview — Penpot High-Level Overview

Usage:
  penpot-cli overview [--cached]

Aturan resmi server MCP Penpot: struktur shape, layout flex/grid, text,
penpotUtils, library, komponen/variants, tokens. Wajib dibaca sekali per
sesi kerja sebelum exec. --cached = baca snapshot embedded (tanpa jaringan).
`

const apiHelp = `penpot-cli api — dokumentasi Penpot Plugin API

Usage:
  penpot-cli api <Type> [member]

Contoh:
  penpot-cli api Board
  penpot-cli api FlexLayout
  penpot-cli api Text growType
  penpot-cli api TokenCatalog

Daftar Type lengkap ada di "penpot-cli overview".
`

const execHelp = `penpot-cli exec — jalankan JavaScript di plugin Penpot (execute_code)

Usage:
  penpot-cli exec '<js>'
  penpot-cli exec -f file.js
  penpot-cli exec - < file.js          (stdin)
  penpot-cli exec ... [--json] [--timeout 120s]

Kode = body async function: pakai return untuk hasil (objek JS boleh,
tanpa JSON.stringify), await boleh. Global: penpot, penpotUtils, storage
(persist lintas panggilan — simpan id/helper di storage). Hasil return
dicetak pretty ke stdout; console.log ke stderr. --json = teks mentah MCP.

Contoh:
  penpot-cli exec 'return penpotUtils.shapeStructure(penpot.root, 2)'
  penpot-cli exec -f build-hero.js
`

const exportHelp = `penpot-cli export — render shape ke PNG/SVG

Usage:
  penpot-cli export <shapeId|selection|page> [--format png|svg] [--mode shape|fill] [--out FILE]

--mode fill = gambar mentah yang dipakai sebagai fill (PNG saja).
Tanpa --out: ./penpot-<id>-<waktu>.<ext>. Untuk QA visual per section.
`

const snapshotHelp = `penpot-cli snapshot — pohon shape mendalam (JSON)

Usage:
  penpot-cli snapshot <shapeId|selection> [--out FILE]

Isi per node: geometri, fills/strokes, radius, layout flex/grid + layoutChild,
teks (font, ukuran, growType), status komponen, tokens. Untuk analisis,
QA, dan sumber design-to-code.
`

const referenceHelp = `penpot-cli reference — board referensi terkunci (untuk QA design-to-design)

Usage:
  penpot-cli reference import <image> [--name N] [--x 0] [--y 0]
  penpot-cli reference clone <shapeId|selection> [--name N] [--x X] [--y Y]

import: PNG/JPEG/GIF lokal (maks 10 MiB) → board ukuran asli dengan image
fill, blocked (terkunci). clone: duplikat shape yang sudah ada di page,
dikunci, ditaruh di kanannya.
`

const componentHelp = `penpot-cli component — komponen local library

Usage:
  penpot-cli component list
  penpot-cli component create <shapeId|selection> [--name "Button / Primary"]
`

const codegenHelp = `penpot-cli codegen — bundle evidence untuk design-to-code

Usage:
  penpot-cli codegen bundle <shapeId|selection> --out DIR

Menulis design.json (snapshot), markup.html + styles.css (generateMarkup/
generateStyle Penpot), manifest.json. Evidence, bukan kode produksi.
`

const toolsHelp = `penpot-cli tools — daftar tools MCP live

Usage:
  penpot-cli tools [--json]
`

const callHelp = `penpot-cli call — escape hatch: panggil tool MCP apa pun

Usage:
  penpot-cli call <tool> ['{"json":"args"}'] [--out FILE] [--json]

Contoh:
  penpot-cli call penpot_api_info '{"type":"Board","member":"flex"}'
`

// ---- daemon ----

func cmdDaemon(argv []string) error {
	o, err := parseArgs(argv, 30*time.Second)
	if err != nil {
		return err
	}
	if helpOr(o, daemonHelp) {
		return nil
	}
	if len(o.pos) != 1 {
		fmt.Fprint(os.Stderr, daemonHelp)
		return errors.New("butuh start|stop|status|reconnect")
	}
	switch o.pos[0] {
	case "start":
		if c, err := net.Dial("unix", sockPath()); err == nil {
			c.Close()
			fmt.Println("daemon sudah jalan")
			return nil
		}
		c, err := dialDaemon()
		if err != nil {
			return err
		}
		c.Close()
		pid, _ := readPid()
		fmt.Printf("daemon jalan (pid %d)\n", pid)
		return nil

	case "stop":
		pid, err := readPid()
		if err != nil {
			os.Remove(sockPath())
			fmt.Println("daemon tidak jalan")
			return nil
		}
		if err := terminateProcess(pid); err != nil {
			os.Remove(sockPath())
			os.Remove(pidPath())
			fmt.Printf("pid %d tidak hidup — file basi dibersihkan\n", pid)
			return nil
		}
		for i := 0; i < 50; i++ { // tunggu cleanup (DELETE sesi) maks 5 detik
			if _, err := os.Stat(pidPath()); err != nil {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		fmt.Printf("daemon (pid %d) dihentikan\n", pid)
		return nil

	case "status":
		if c, err := net.Dial("unix", sockPath()); err != nil {
			fmt.Println("daemon: tidak jalan")
			return nil
		} else {
			c.Close()
		}
		resp, err := callDaemon("ping", nil, o.timeout)
		if err != nil {
			return err
		}
		fmt.Println("daemon:", resp.Result.Text)
		return nil

	case "reconnect":
		t0 := time.Now()
		resp, err := callDaemon("reconnect", nil, o.timeout)
		if err != nil {
			return err
		}
		fmt.Printf("sesi MCP baru: %d tools (%dms)\n", len(resp.Tools), time.Since(t0).Milliseconds())
		return nil
	}
	return errors.New("usage: penpot-cli daemon start|stop|status|reconnect")
}

// ---- doctor / tools / call ----

const doctorCode = `const f = penpot.currentFile, p = penpot.currentPage;
return { file: f?.name ?? null, fileId: f?.id ?? null, page: p?.name ?? null, pageId: p?.id ?? null, selection: (penpot.selection ?? []).length };`

func cmdDoctor(argv []string) error {
	o, err := parseArgs(argv, 30*time.Second)
	if err != nil {
		return err
	}
	if helpOr(o, doctorHelp) {
		return nil
	}
	report := map[string]any{}
	t0 := time.Now()
	if _, err := callDaemon("ping", nil, o.timeout); err != nil {
		return fmt.Errorf("daemon: %w", err)
	}
	report["daemon"] = "ok"

	t1 := time.Now()
	resp, err := callDaemon("tools", nil, o.timeout)
	if err != nil {
		report["mcp"] = "error: " + err.Error()
		return finishDoctor(o, report, err)
	}
	names := make([]string, 0, len(resp.Tools))
	for _, t := range resp.Tools {
		names = append(names, t.Name)
	}
	report["mcp"] = fmt.Sprintf("ok — %d tools (%s) %dms", len(names), strings.Join(names, ", "), time.Since(t1).Milliseconds())

	t2 := time.Now()
	skipPin = true
	raw, _, err := runCode(doctorCode, o.timeout)
	if err != nil {
		report["plugin"] = "error: " + err.Error()
		return finishDoctor(o, report, err)
	}
	var focused any
	json.Unmarshal(raw, &focused)
	report["plugin"] = fmt.Sprintf("connected %dms", time.Since(t2).Milliseconds())
	report["focused"] = focused
	report["total_ms"] = time.Since(t0).Milliseconds()
	return finishDoctor(o, report, nil)
}

func finishDoctor(o *opts, r map[string]any, err error) error {
	if o.json {
		b, _ := json.MarshalIndent(r, "", "  ")
		fmt.Println(string(b))
		if err != nil {
			os.Exit(1)
		}
		return nil
	}
	for _, k := range []string{"daemon", "mcp", "plugin"} {
		if v, ok := r[k]; ok {
			fmt.Printf("%-7s %v\n", k+":", v)
		}
	}
	if f, ok := r["focused"].(map[string]any); ok {
		fmt.Printf("fokus:  file %q · page %q · seleksi %v\n", f["file"], f["page"], f["selection"])
		id, _ := f["fileId"].(string)
		fmt.Println(pinStatusLine(id))
		fmt.Println(saveStatusLine(id))
	}
	if t, ok := r["total_ms"]; ok {
		fmt.Printf("total:  %vms\n", t)
	}
	if err != nil {
		return errors.New("doctor gagal (lihat di atas)")
	}
	return nil
}

func cmdTools(argv []string) error {
	o, err := parseArgs(argv, 30*time.Second)
	if err != nil {
		return err
	}
	if helpOr(o, toolsHelp) {
		return nil
	}
	resp, err := callDaemon("tools", nil, o.timeout)
	if err != nil {
		return err
	}
	if o.json {
		b, _ := json.MarshalIndent(resp.Tools, "", "  ")
		fmt.Println(string(b))
		return nil
	}
	for _, t := range resp.Tools {
		desc := strings.SplitN(strings.TrimSpace(t.Description), "\n", 2)[0]
		fmt.Printf("• %s\n    %s\n    args: %s\n", t.Name, desc, string(t.InputSchema))
	}
	return nil
}

func cmdCall(argv []string) error {
	o, err := parseArgs(argv, 120*time.Second)
	if err != nil {
		return err
	}
	if helpOr(o, callHelp) {
		return nil
	}
	if err := o.only("out"); err != nil {
		return err
	}
	if len(o.pos) < 1 {
		return errors.New("usage: penpot-cli call <tool> ['{json args}']")
	}
	args := map[string]any{}
	if len(o.pos) > 1 {
		if err := json.Unmarshal([]byte(o.pos[1]), &args); err != nil {
			return fmt.Errorf("args harus JSON object: %w", err)
		}
	}
	res, err := callTool(o.pos[0], args, o.timeout)
	if err != nil {
		return err
	}
	return printResult(res, o, o.flags["out"], o.pos[0])
}

// printResult: teks ke stdout, gambar ditulis ke file.
func printResult(res *toolResult, o *opts, out, stem string) error {
	if o.json {
		b, _ := json.Marshal(res)
		fmt.Println(string(b))
		return nil
	}
	if res.Text != "" {
		fmt.Println(res.Text)
	}
	files, err := writeImages(res.Images, out, stem)
	for _, f := range files {
		fmt.Println(f)
	}
	return err
}
