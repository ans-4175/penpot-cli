package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Pin: MCP Penpot selalu bekerja pada file yang sedang FOKUS di tab user.
// Kalau user pindah file (mis. buka file lain saat agent sedang mengerjakan
// "Ads"), exec berikutnya diam-diam menulis ke file yang salah. Pin mengunci
// file target: setiap exec diawali guard yang menolak jalan kalau file fokus
// berbeda — sebelum satu baris pun kode user dieksekusi.
//
//	penpot-cli pin            kunci ke file yang sedang fokus
//	penpot-cli pin --show     tampilkan pin
//	penpot-cli pin --clear    lepas
//	PENPOT_PIN=off            abaikan pin untuk satu perintah

type pinInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	At   string `json:"at"`
}

func pinPath() string { return filepath.Join(runtimeDir(), "pin.json") }

func loadPin() *pinInfo {
	if v := os.Getenv("PENPOT_PIN"); v == "off" || v == "0" {
		return nil
	}
	b, err := os.ReadFile(pinPath())
	if err != nil {
		return nil
	}
	var p pinInfo
	if json.Unmarshal(b, &p) != nil || p.ID == "" {
		return nil
	}
	return &p
}

// skipPin dimatikan untuk perintah read-only yang justru dipakai untuk
// melihat file fokus (doctor, state, pin).
var skipPin bool

func pinGuard(code string) string {
	if skipPin {
		return code
	}
	p := loadPin()
	if p == nil {
		return code
	}
	id, _ := json.Marshal(p.ID)
	name, _ := json.Marshal(p.Name)
	return `if (penpot.currentFile?.id !== ` + string(id) + `) throw new Error("[pin] file fokus \"" + (penpot.currentFile?.name ?? "-") + "\" ≠ file yang di-pin \"" + ` + string(name) + ` + "\" — minta user membuka file itu di tab Penpot, atau penpot-cli pin --clear");
` + code
}

const pinHelp = `penpot-cli pin [--show | --clear]

Kunci file target. Selama pin aktif, setiap exec/settle/export/... menolak
jalan kalau file yang fokus di tab Penpot bukan file yang di-pin (cegah
menulis ke file yang salah saat user berpindah file).

  penpot-cli pin           pin file yang sedang fokus
  penpot-cli pin --show    tampilkan pin
  penpot-cli pin --clear   lepas pin
  PENPOT_PIN=off           abaikan pin untuk satu perintah
`

func cmdPin(argv []string) error {
	o, err := parseArgs(argv, 30*time.Second, "show", "clear")
	if err != nil {
		return err
	}
	if helpOr(o, pinHelp) {
		return nil
	}
	if err := o.only("show", "clear"); err != nil {
		return err
	}
	if len(o.pos) > 0 {
		return errors.New("pin tidak menerima argumen; buka file target di tab Penpot lalu jalankan penpot-cli pin")
	}
	if o.flags["clear"] != "" {
		if err := os.Remove(pinPath()); err != nil && !os.IsNotExist(err) {
			return err
		}
		fmt.Println("pin dilepas")
		return nil
	}
	if o.flags["show"] != "" {
		if p := loadPin(); p != nil {
			fmt.Printf("pin: %q (%s) sejak %s\n", p.Name, p.ID, p.At)
		} else {
			fmt.Println("pin: tidak ada")
		}
		return nil
	}
	skipPin = true
	raw, _, err := runCode("return { id: penpot.currentFile?.id ?? null, name: penpot.currentFile?.name ?? null }", o.timeout)
	if err != nil {
		return err
	}
	var p pinInfo
	if json.Unmarshal(raw, &p) != nil || p.ID == "" {
		return errors.New("tidak ada file fokus")
	}
	p.At = time.Now().Format("2006-01-02 15:04")
	b, _ := json.MarshalIndent(p, "", "  ")
	if err := os.MkdirAll(runtimeDir(), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(pinPath(), b, 0o600); err != nil {
		return err
	}
	fmt.Printf("pin: %q (%s)\n", p.Name, p.ID)
	return nil
}

// pinStatusLine untuk doctor.
func pinStatusLine(focusID string) string {
	p := loadPin()
	if p == nil {
		return "pin:    — (tidak dikunci; penpot-cli pin untuk mengunci file target)"
	}
	if strings.EqualFold(p.ID, focusID) {
		return fmt.Sprintf("pin:    %q ✓ cocok dengan file fokus", p.Name)
	}
	return fmt.Sprintf("pin:    %q ✗ file fokus BERBEDA — exec akan ditolak", p.Name)
}
