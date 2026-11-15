package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

// Shared Library lewat Penpot REST RPC (bukan MCP): Plugin API tidak punya
// fungsi publish. Butuh PENPOT_PERSONAL_TOKEN (Penpot → Your account →
// Access tokens) di ~/.config/ans/penpot. Endpoint: /api/rpc/command/<name>.

// Fitur yang dideklarasikan client; tanpa ini get-file/create-file → 400
// "client declares no support for …".
var rpcFeatures = []string{"fdata/path-data", "design-tokens/v1", "variants/v1", "layout/grid",
	"components/v2", "fdata/shape-data-type", "fdata/objects-map", "fdata/pointer-map", "styles/v2"}

func rpcConfig() (base, token string, err error) {
	cfg := map[string]string{}
	if data, e := os.ReadFile(configPath()); e == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if i := strings.Index(line, "="); i > 0 && !strings.HasPrefix(line, "#") {
				cfg[strings.TrimSpace(line[:i])] = strings.Trim(strings.TrimSpace(line[i+1:]), `"'`)
			}
		}
	}
	get := func(k string) string {
		if v := os.Getenv(k); v != "" {
			return v
		}
		return cfg[k]
	}
	domain := get("PENPOT_DOMAIN")
	if domain == "" {
		domain = "design.penpot.app"
	}
	token = get("PENPOT_PERSONAL_TOKEN")
	if token == "" {
		return "", "", fmt.Errorf("PENPOT_PERSONAL_TOKEN tidak ada di %s (Penpot → Your account → Access tokens)", configPath())
	}
	return "https://" + domain + "/api/rpc/command/", token, nil
}

// rpc memanggil satu command. body nil → GET dengan query, selain itu POST JSON.
func rpcCall(name string, query map[string]string, body any, out any) error {
	base, token, err := rpcConfig()
	if err != nil {
		return err
	}
	url := base + name
	method := "GET"
	var rd io.Reader
	if body != nil {
		method = "POST"
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	} else if len(query) > 0 {
		var q []string
		for k, v := range query {
			q = append(q, k+"="+v)
		}
		url += "?" + strings.Join(q, "&")
	}
	req, err := http.NewRequest(method, url, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Token "+token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent())
	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if resp.StatusCode >= 400 {
		msg := strings.TrimSpace(string(data))
		if strings.Contains(msg, "<!DOCTYPE") {
			msg = "(halaman HTML — diblok Cloudflare / endpoint tidak ada)"
		}
		if len(msg) > 300 {
			msg = msg[:300] + "…"
		}
		return fmt.Errorf("%s: HTTP %d %s", name, resp.StatusCode, msg)
	}
	if out != nil && len(data) > 0 {
		return json.Unmarshal(data, out)
	}
	return nil
}

type rpcFile struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	IsShared  bool   `json:"isShared"`
	ProjectID string `json:"projectId"`
	TeamID    string `json:"teamId"`
	Revn      int    `json:"revn"`
	Modified  string `json:"modifiedAt"`
}

const libraryHelp = `penpot-cli library — Shared Library (REST RPC, butuh PENPOT_PERSONAL_TOKEN)

Usage:
  penpot-cli library status [fileId]          shared?, library yang terhubung (default: file fokus)
  penpot-cli library list [--team ID]         semua shared library di team
  penpot-cli library publish [fileId]         jadikan file Shared Library
  penpot-cli library unpublish [fileId]       batalkan shared (file pemakai jadi lepas!)
  penpot-cli library link <libId> [--file ID] hubungkan library ke file (default: file fokus)
  penpot-cli library unlink <libId> [--file ID]

Tanpa fileId → file yang sedang fokus di plugin (penpot.currentFile.id).
Setelah link, komponen library bisa di-instance dari file lain lewat
penpot.library.connected (exec), dan perubahan main component menyebar.
`

func focusedFileID() (string, error) {
	raw, _, err := runCode("return penpot.currentFile?.id ?? null", 30*time.Second)
	if err != nil {
		return "", err
	}
	var id string
	if json.Unmarshal(raw, &id) != nil || id == "" {
		return "", errors.New("tidak ada file fokus")
	}
	return id, nil
}

func cmdLibrary(argv []string) error {
	o, err := parseArgs(argv, 60*time.Second)
	if err != nil {
		return err
	}
	if helpOr(o, libraryHelp) || len(o.pos) == 0 {
		fmt.Print(libraryHelp)
		return nil
	}
	if err := o.only("team", "file"); err != nil {
		return err
	}
	fileArg := func(i int) (string, error) {
		if v := o.flags["file"]; v != "" {
			return v, nil
		}
		if len(o.pos) > i {
			return o.pos[i], nil
		}
		return focusedFileID()
	}
	getFile := func(id string) (*rpcFile, error) {
		var f rpcFile
		err := rpcCall("get-file", map[string]string{"id": id, "features": strings.Join(rpcFeatures, ",")}, nil, &f)
		return &f, err
	}
	printJSONv := func(v any) {
		b, _ := json.MarshalIndent(v, "", "  ")
		fmt.Println(string(b))
	}
	switch o.pos[0] {
	case "status":
		id, err := fileArg(1)
		if err != nil {
			return err
		}
		f, err := getFile(id)
		if err != nil {
			return err
		}
		var libs []rpcFile
		if err := rpcCall("get-file-libraries", map[string]string{"file-id": id}, nil, &libs); err != nil {
			return err
		}
		names := []string{}
		for _, l := range libs {
			names = append(names, l.Name+" ("+l.ID+")")
		}
		printJSONv(map[string]any{"file": f.Name, "id": f.ID, "shared": f.IsShared, "team": f.TeamID, "project": f.ProjectID, "linkedLibraries": names})
	case "list":
		team := o.flags["team"]
		if team == "" {
			id, err := focusedFileID()
			if err != nil {
				return err
			}
			f, err := getFile(id)
			if err != nil {
				return err
			}
			team = f.TeamID
		}
		var libs []rpcFile
		if err := rpcCall("get-team-shared-files", map[string]string{"team-id": team}, nil, &libs); err != nil {
			return err
		}
		for _, l := range libs {
			fmt.Printf("%s  %s\n", l.ID, l.Name)
		}
	case "publish", "unpublish":
		id, err := fileArg(1)
		if err != nil {
			return err
		}
		var f rpcFile
		if err := rpcCall("set-file-shared", nil, map[string]any{"id": id, "isShared": o.pos[0] == "publish"}, &f); err != nil {
			return err
		}
		fmt.Printf("%s: shared=%v\n", f.Name, f.IsShared)
	case "link", "unlink":
		if len(o.pos) < 2 {
			return errors.New("usage: penpot-cli library " + o.pos[0] + " <libraryId> [--file ID]")
		}
		lib := o.pos[1]
		id := o.flags["file"]
		if id == "" {
			if id, err = focusedFileID(); err != nil {
				return err
			}
		}
		cmd := "link-file-to-library"
		if o.pos[0] == "unlink" {
			cmd = "unlink-file-from-library"
		}
		if err := rpcCall(cmd, nil, map[string]any{"fileId": id, "libraryId": lib}, nil); err != nil {
			return err
		}
		fmt.Printf("%s ok: file %s ↔ library %s (reload tab Penpot agar penpot.library.connected ter-update)\n", o.pos[0], id, lib)
	default:
		fmt.Print(libraryHelp)
		return fmt.Errorf("aksi %q tidak dikenal", o.pos[0])
	}
	return nil
}

// ---- backup: snapshot file lewat REST ----
//
// REST create-file-snapshot instan dan tidak bergantung pada tab. (Catatan:
// penpot.currentFile.saveVersion() normal ±0,8 s saat sinkronisasi tab sehat;
// ia menggantung HANYA saat antrean simpan tab macet — lihat `penpot-cli sync`.)

const versionHelp = `penpot-cli backup save "<label>" | list | restore <id>

  save     simpan snapshot file yang sedang fokus (REST, instan, sebelum
           operasi destruktif: hapus/ganti slide, variant, import)
  list     snapshot buatan user (label, waktu, id)
  restore  kembalikan file ke snapshot (menimpa kondisi sekarang; konfirmasi user dulu)
`

type snapInfo struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	CreatedAt string `json:"createdAt"`
	CreatedBy string `json:"createdBy"`
}

func cmdVersion(argv []string) error {
	o, err := parseArgs(argv, 60*time.Second)
	if err != nil {
		return err
	}
	if helpOr(o, versionHelp) || len(o.pos) == 0 {
		fmt.Print(versionHelp)
		return nil
	}
	skipPin = true // pakai id file fokus; tidak menulis ke canvas
	fid, err := focusedFileID()
	if err != nil {
		return err
	}
	switch o.pos[0] {
	case "save":
		if len(o.pos) < 2 {
			return errors.New(`usage: penpot-cli backup save "<label>"`)
		}
		var res struct {
			ID string `json:"id"`
		}
		label := "penpot-cli: " + strings.TrimPrefix(strings.Join(o.pos[1:], " "), "penpot-cli: ")
		if err := rpcCall("create-file-snapshot", nil, map[string]any{"fileId": fid, "label": label}, &res); err != nil {
			return err
		}
		fmt.Printf("snapshot tersimpan: %q (%s)\n", label, res.ID)
	case "list":
		var all []snapInfo
		if err := rpcCall("get-file-snapshots", map[string]string{"file-id": fid}, nil, &all); err != nil {
			return err
		}
		sort.Slice(all, func(i, j int) bool { return all[i].CreatedAt > all[j].CreatedAt })
		n := 0
		for _, s := range all {
			if s.CreatedBy != "user" {
				continue
			}
			fmt.Printf("%s  %s  %s\n", strings.Replace(s.CreatedAt[:19], "T", " ", 1), s.ID, s.Label)
			if n++; n >= 15 {
				break
			}
		}
	case "restore":
		if len(o.pos) != 2 {
			return errors.New("usage: penpot-cli backup restore <snapshotId>")
		}
		if err := rpcCall("restore-file-snapshot", nil, map[string]any{"fileId": fid, "id": o.pos[1]}, nil); err != nil {
			return err
		}
		fmt.Println("file dikembalikan ke snapshot", o.pos[1], "— reload tab Penpot")
	default:
		fmt.Print(versionHelp)
		return fmt.Errorf("aksi %q tidak dikenal", o.pos[0])
	}
	return nil
}
