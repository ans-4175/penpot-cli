package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// sync: perubahan di tab Penpot dikirim ke server lewat antrean simpan di
// browser. Kalau antrean macet (mis. shape ditolak server: teks tinggi
// 70 juta px) perubahan berikutnya HANYA ada di tab — export (baca dari
// server) tampak tidak berubah, saveVersion menggantung, dan reload tab
// membuang semuanya. `sync` membuktikan perubahan benar-benar tersimpan.

const syncHelp = `penpot-cli sync [--wait 15s]

Cek bahwa tab Penpot menyimpan ke server: menulis penanda kecil (plugin data
file), lalu menunggu revn di server naik.

  OK        revn naik → perubahan tersimpan; export & backup memakai data terkini
  TIDAK     revn tetap → antrean simpan tab macet: perubahan belum tersimpan.
            Jangan lanjut membangun. Cari shape rusak (teks lebar 0 / tinggi
            ekstrem), hapus per id; kalau tetap, minta user reload tab (Cmd+R)
            — perubahan yang belum tersimpan akan hilang.

Jalankan setelah tiap slide/section besar dan sebelum export QA.
`

func serverRevn(fid string) (int, string, error) {
	var f rpcFile
	err := rpcCall("get-file", map[string]string{"id": fid, "features": strings.Join(rpcFeatures, ",")}, nil, &f)
	return f.Revn, f.Modified, err
}

func cmdSync(argv []string) error {
	o, err := parseArgs(argv, 60*time.Second)
	if err != nil {
		return err
	}
	if helpOr(o, syncHelp) {
		return nil
	}
	if err := o.only("wait"); err != nil {
		return err
	}
	wait := 15 * time.Second
	if v := o.flags["wait"]; v != "" {
		if d, e := time.ParseDuration(v); e == nil {
			wait = d
		}
	}
	skipPin = true
	fid, err := focusedFileID()
	if err != nil {
		return err
	}
	before, mod, err := serverRevn(fid)
	if err != nil {
		return err
	}
	stamp := time.Now().Format("2006-01-02T15:04:05")
	if _, _, err := runCode(fmt.Sprintf("penpot.currentFile.setPluginData('penpot-cli.sync', %q); return 1", stamp), 30*time.Second); err != nil {
		return err
	}
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		time.Sleep(1500 * time.Millisecond)
		now, m2, err := serverRevn(fid)
		if err == nil && now > before {
			fmt.Printf("OK: tersimpan ke server (revn %d → %d, %s)\n", before, now, shortTime(m2))
			return nil
		}
	}
	fmt.Fprintf(os.Stderr, "TIDAK TERSIMPAN: revn server tetap %d (terakhir disimpan %s) setelah %s.\n"+
		"Antrean simpan tab macet. Cek shape rusak (teks lebar 0 / tinggi ekstrem) dan hapus per id; kalau tetap,\n"+
		"minta user reload tab (Cmd+R) — perubahan yang belum tersimpan akan hilang.\n", before, shortTime(mod), wait)
	return errors.New("sync gagal")
}

func shortTime(iso string) string {
	if len(iso) >= 19 {
		return iso[11:19] + " UTC"
	}
	return iso
}

// saveStatusLine untuk doctor: umur simpan terakhir di server (info, bukan bukti —
// gunakan `penpot-cli sync` untuk bukti aktif).
func saveStatusLine(fid string) string {
	if _, _, err := rpcConfig(); err != nil {
		return "simpan: — (butuh PENPOT_PERSONAL_TOKEN untuk cek server)"
	}
	rev, mod, err := serverRevn(fid)
	if err != nil {
		return "simpan: ? (" + err.Error() + ")"
	}
	age := ""
	if t, e := time.Parse(time.RFC3339Nano, mod); e == nil {
		age = fmt.Sprintf(", %s lalu", time.Since(t).Round(time.Second))
	}
	return fmt.Sprintf("simpan: revn %d, terakhir %s%s (penpot-cli sync = bukti aktif)", rev, shortTime(mod), age)
}
