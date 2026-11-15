package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// app: buka Penpot di jendela Chrome khusus (profil sendiri, tanpa address bar)
// dengan flag anti-throttling. Teruji Okt 2026: jendela disembunyikan 2 menit →
// timer & pengukuran teks tetap normal (Safari: berhenti menjawab setelah ±30 s;
// Chrome biasa: timer melambat ±1 s per tick, teks terukur 1×1).

const appHelp = `penpot-cli app [url|--file ID [--page ID]] [--chrome NAME|PATH]

Buka Penpot di jendela Chrome khusus (profil ~/.penpot-chrome, mode aplikasi)
dengan flag anti-throttling, supaya plugin MCP tetap jalan saat jendela
disembunyikan / tertutup aplikasi lain.

  penpot-cli app                          buka dashboard Penpot
  penpot-cli app --file <fileId>          buka file tertentu (team dibaca otomatis)
  penpot-cli app --file <id> --page <id>  buka page tertentu
  penpot-cli app "<url penpot>"           buka URL apa adanya
  --new                                   tetap buka jendela baru walau sudah jalan
  --status                                jalan atau tidak (pid)
  --quit                                  tutup jendela Penpot khusus (proses profil
                                          tetap hidup di latar setelah jendela ditutup)
  --chrome NAME|PATH                      macOS: nama aplikasi ("Google Chrome");
                                          Linux/Windows: path executable Chrome/Chromium
                                          (default: dicari otomatis)

Setelah terbuka: login (sekali per profil) → File → MCP Server → Connect.
Teruji (Okt 2026, jendela disembunyikan): normal 10 menit; Chrome biasa
melambat, Safari berhenti total setelah ±30 s. Catatan: penguncian domain lewat
extension TIDAK bisa — Google Chrome mengabaikan --load-extension.
`

var appFlags = []string{
	"--no-first-run",
	"--no-default-browser-check",
	"--disable-background-timer-throttling",
	"--disable-backgrounding-occluded-windows",
	"--disable-renderer-backgrounding",
}

func appProfileDir() string { return filepath.Join(homeDir(), ".penpot-chrome") }

func cmdApp(argv []string) error {
	o, err := parseArgs(argv, 30*time.Second, "new", "quit", "status")
	if err != nil {
		return err
	}
	if helpOr(o, appHelp) {
		return nil
	}
	if err := o.only("file", "page", "chrome", "new", "quit", "status"); err != nil {
		return err
	}
	if o.flags["status"] != "" {
		if pid := appRunning(); pid != "" {
			fmt.Printf("jendela Penpot khusus: jalan (pid %s, profil %s)\n", pid, appProfileDir())
		} else {
			fmt.Println("jendela Penpot khusus: tidak jalan")
		}
		return nil
	}
	if o.flags["quit"] != "" {
		// Menutup jendela terakhir di macOS tidak menghentikan Chrome: proses
		// profil ini tetap jalan di latar. Hentikan proses utamanya saja
		// (Chrome biasa user tidak tersentuh — profil berbeda).
		pid := appRunning()
		if pid == "" {
			fmt.Println("jendela Penpot khusus tidak jalan")
			return nil
		}
		n, _ := strconv.Atoi(pid)
		if err := terminateProcess(n); err != nil {
			return fmt.Errorf("hentikan pid %s: %w", pid, err)
		}
		for i := 0; i < 20 && appRunning() != ""; i++ {
			time.Sleep(500 * time.Millisecond)
		}
		if appRunning() != "" {
			return errors.New("proses belum berhenti; coba lagi")
		}
		fmt.Printf("jendela Penpot khusus ditutup (pid %s)\n", pid)
		return nil
	}
	base, _, _ := rpcConfig()
	domain := "design.penpot.app"
	if base != "" {
		domain = strings.TrimSuffix(strings.TrimPrefix(base, "https://"), "/api/rpc/command/")
	}
	url := "https://" + domain + "/"
	switch {
	case len(o.pos) == 1:
		url = o.pos[0]
		if !strings.HasPrefix(url, "https://"+domain+"/") {
			return fmt.Errorf("hanya URL %s yang dibuka di jendela ini", domain)
		}
	case o.flags["file"] != "":
		var f rpcFile
		if err := rpcCall("get-file", map[string]string{"id": o.flags["file"], "features": strings.Join(rpcFeatures, ",")}, nil, &f); err != nil {
			return fmt.Errorf("baca file: %w", err)
		}
		url = fmt.Sprintf("https://%s/#/workspace?team-id=%s&file-id=%s", domain, f.TeamID, f.ID)
		if p := o.flags["page"]; p != "" {
			url += "&page-id=" + p
		}
	}
	// Chrome selalu membuka JENDELA BARU untuk --app, walau profil sudah jalan.
	// Dua jendela = dua sesi Penpot di file yang sama dan hanya satu yang MCP.
	if running := appRunning(); running != "" && o.flags["new"] == "" {
		fmt.Printf("jendela Penpot khusus sudah jalan (pid %s) — tidak membuka jendela baru.\n", running)
		fmt.Println("Pindah file/page dari jendela itu, atau pakai --new untuk jendela tambahan (MCP hanya di satu jendela).")
		return nil
	}
	if err := launchChrome(o.flags["chrome"], url); err != nil {
		return err
	}
	fmt.Println("dibuka:", url)
	fmt.Println("profil:", appProfileDir(), "· flag:", strings.Join(appFlags[2:], " "))
	if _, err := os.Stat(filepath.Join(appProfileDir(), "Default", "Cookies")); errors.Is(err, os.ErrNotExist) {
		fmt.Println("profil baru → login dulu, lalu File → MCP Server → Connect")
	} else {
		fmt.Println("lalu: File → MCP Server → Connect (kalau belum)")
	}
	return nil
}

// launchChrome membuka Chrome dengan profil khusus. macOS lewat `open -na` (-n:
// instance baru per profil; kalau profil sudah jalan, --app diteruskan ke proses
// yang ada). Linux/Windows menjalankan executable langsung, terlepas dari terminal.
func launchChrome(override, url string) error {
	args := append([]string{"--user-data-dir=" + appProfileDir(), "--app=" + url}, appFlags...)
	if runtime.GOOS == "darwin" {
		name := override
		if name == "" {
			name = "Google Chrome"
		}
		full := append([]string{"-na", name, "--args"}, args...)
		if out, err := exec.Command("open", full...).CombinedOutput(); err != nil {
			return fmt.Errorf("open: %v %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	}
	exe, err := findChrome(override)
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, args...)
	detachDaemonProcess(cmd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("jalankan %s: %w", exe, err)
	}
	return cmd.Process.Release()
}

// chromeCandidates: lokasi umum Chrome/Chromium di luar macOS (urutan prioritas).
func chromeCandidates(goos string, getenv func(string) string) []string {
	if goos == "windows" {
		var out []string
		for _, env := range []string{"ProgramFiles", "ProgramFiles(x86)", "LocalAppData"} {
			if base := getenv(env); base != "" {
				out = append(out, filepath.Join(base, "Google", "Chrome", "Application", "chrome.exe"))
			}
		}
		return out
	}
	return []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser"}
}

func findChrome(override string) (string, error) {
	if override != "" {
		if p, err := exec.LookPath(override); err == nil {
			return p, nil
		}
		return "", fmt.Errorf("Chrome tidak ditemukan di %q", override)
	}
	for _, c := range chromeCandidates(runtime.GOOS, os.Getenv) {
		if p, err := exec.LookPath(c); err == nil {
			return p, nil
		}
	}
	return "", errors.New("Chrome/Chromium tidak ditemukan; pasang atau beri --chrome <path>")
}

// winRunningScript: PowerShell yang mencetak pid proses utama Chrome (tanpa
// --type=) berprofil `dir`. Contains() menghindari karakter wildcard -like.
func winRunningScript(dir string) string {
	q := strings.ReplaceAll(dir, "'", "''")
	return "Get-CimInstance Win32_Process -Filter \"Name='chrome.exe'\" | " +
		"Where-Object { $_.CommandLine -and $_.CommandLine.Contains('user-data-dir=" + q + "') -and -not $_.CommandLine.Contains('--type=') } | " +
		"Select-Object -First 1 -ExpandProperty ProcessId"
}

// appRunning: pid proses utama Chrome berprofil ~/.penpot-chrome, "" kalau tidak ada.
func appRunning() string {
	if runtime.GOOS == "windows" {
		out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", winRunningScript(appProfileDir())).Output()
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(out))
	}
	out, err := exec.Command("pgrep", "-f", "user-data-dir="+appProfileDir()).Output()
	if err != nil {
		return ""
	}
	for _, pid := range strings.Fields(string(out)) {
		cmd, _ := exec.Command("ps", "-o", "command=", "-p", pid).Output()
		if !strings.Contains(string(cmd), "--type=") {
			return pid
		}
	}
	return ""
}
