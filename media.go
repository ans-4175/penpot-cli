package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Server MCP Penpot menolak request > ±100 KB (HTTP 413), jadi gambar tidak
// bisa dikirim dalam satu execute_code. uploadMedia memecah base64 menjadi
// potongan yang ditampung di storage plugin, lalu satu panggilan terakhir
// menyatukan + uploadMediaData. Hasil (ImageData) disimpan di
// storage.media[key] — pakai di exec: shape.fills = [{ fillOpacity: 1, fillImage: storage.media[key] }].
const uploadChunk = 64 << 10 // karakter base64 per request (aman di bawah batas)

var keyRE = regexp.MustCompile(`[^A-Za-z0-9_-]+`)

func mediaKey(path string) string {
	k := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return strings.Trim(keyRE.ReplaceAllString(k, "-"), "-")
}

func uploadMedia(path string, data []byte, mime, key string, timeout time.Duration) (string, error) {
	if key == "" {
		key = mediaKey(path)
	}
	b64 := base64.StdEncoding.EncodeToString(data)
	kq, _ := json.Marshal(key)
	if _, _, err := runCodeDirect("storage.__up = storage.__up ?? {}; storage.__up["+string(kq)+"] = []; return true", timeout); err != nil {
		return "", err
	}
	n := 0
	for i := 0; i < len(b64); i += uploadChunk {
		end := min(i+uploadChunk, len(b64))
		cq, _ := json.Marshal(b64[i:end])
		if _, _, err := runCodeDirect("storage.__up["+string(kq)+"].push("+string(cq)+"); return storage.__up["+string(kq)+"].length", timeout); err != nil {
			return "", fmt.Errorf("upload potongan %d: %w", n+1, err)
		}
		n++
	}
	nq, _ := json.Marshal(filepath.Base(path))
	mq, _ := json.Marshal(mime)
	final := `const k = ` + string(kq) + `;
const b64 = storage.__up[k].join('');
delete storage.__up[k];
const media = await penpot.uploadMediaData(` + string(nq) + `, Uint8Array.from(atob(b64), c => c.charCodeAt(0)), ` + string(mq) + `);
storage.media = storage.media ?? {};
storage.media[k] = media;
return { key: k, id: media.id, w: media.width, h: media.height };`
	if _, _, err := runCodeDirect(final, timeout); err != nil {
		return "", err
	}
	fmt.Fprintf(os.Stderr, "[upload] %s → storage.media[%q] (%d KB, %d potongan)\n", filepath.Base(path), key, len(data)>>10, n)
	return key, nil
}

const uploadHelp = `penpot-cli upload — upload gambar lokal ke file Penpot (media)

Usage:
  penpot-cli upload <image|dir>... [--key nama] [--max 1080] [--quality 85] [--force]

- Dikirim bertahap (batas request MCP ±100 KB) → ImageData di storage.media[key].
- --max N   : kecilkan sisi terpanjang ke N px sebelum upload (pakai sips;
              PNG transparan tetap PNG, lainnya → JPEG --quality). Default:
              tanpa resize.
- Manifest  : setiap upload dicatat di plugin data FILE (penpot-cli.media),
              key → {ImageData, sha1}. Upload berikutnya dengan isi sama
              dilewati, dan storage.media dipulihkan dari manifest setelah tab
              di-reload — tanpa upload ulang. --force = upload ulang.
- key default = nama file tanpa ekstensi (folder → path relatif tanpa ekstensi).
- --base DIR: key = path relatif ke DIR dengan ekstensi, mis. dengan
  --base ~/project → "assets/analogi/magic-com.png" (sama
  dengan url image fill di .pen — dipakai porter).

Pakai di exec:
  shape.fills = [{ fillOpacity: 1, fillImage: storage.media['magic-com'] }]
`

const manifestJS = `const raw = penpot.currentFile.getPluginData('penpot-cli.media');
const m = raw ? JSON.parse(raw) : {};
storage.media = storage.media ?? {};
for (const [k, v] of Object.entries(m)) if (!storage.media[k]) storage.media[k] = v.img;
`

// mediaManifest mengembalikan key→sha1 dari plugin data file, sekaligus
// memulihkan storage.media.
func mediaManifest(timeout time.Duration) (map[string]string, error) {
	raw, _, err := runCodeDirect(manifestJS+`return Object.fromEntries(Object.entries(m).map(([k, v]) => [k, v.sha1]));`, timeout)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	json.Unmarshal(raw, &out)
	return out, nil
}

func recordManifest(key, sha string, timeout time.Duration) error {
	kq, _ := json.Marshal(key)
	sq, _ := json.Marshal(sha)
	_, _, err := runCodeDirect(manifestJS+`m[`+string(kq)+`] = { sha1: `+string(sq)+`, img: JSON.parse(JSON.stringify(storage.media[`+string(kq)+`])) };
penpot.currentFile.setPluginData('penpot-cli.media', JSON.stringify(m));
return Object.keys(m).length;`, timeout)
	return err
}

var imgExt = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true}

// shrink: kecilkan ke sisi terpanjang max px via sips (macOS). PNG dengan
// alpha tetap PNG; selain itu JPEG. Mengembalikan data + mime baru.
func shrink(path string, data []byte, max, quality int) ([]byte, string, error) {
	if runtime.GOOS != "darwin" {
		return nil, "", errors.New("--max butuh `sips` (macOS); upload tanpa --max, atau kecilkan gambar dulu")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return data, "", err
	}
	if cfg.Width <= max && cfg.Height <= max && format != "png" {
		return data, "image/" + format, nil
	}
	keepPNG := format == "png" && hasAlpha(data)
	tmp, err := os.CreateTemp("", "penpot-cli-*")
	if err != nil {
		return nil, "", err
	}
	tmp.Close()
	defer os.Remove(tmp.Name())
	args := []string{}
	if cfg.Width > max || cfg.Height > max {
		args = append(args, "-Z", strconv.Itoa(max))
	}
	mime := "image/png"
	if keepPNG {
		args = append(args, "-s", "format", "png")
	} else {
		args = append(args, "-s", "format", "jpeg", "-s", "formatOptions", strconv.Itoa(quality))
		mime = "image/jpeg"
	}
	args = append(args, path, "--out", tmp.Name())
	if out, err := exec.Command("sips", args...).CombinedOutput(); err != nil {
		return nil, "", fmt.Errorf("sips: %v: %s", err, out)
	}
	small, err := os.ReadFile(tmp.Name())
	if err != nil {
		return nil, "", err
	}
	if len(small) >= len(data) && cfg.Width <= max && cfg.Height <= max {
		return data, "image/" + format, nil // tidak lebih kecil — pakai asli
	}
	return small, mime, nil
}

func hasAlpha(data []byte) bool {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return true
	}
	if _, ok := img.(interface{ Opaque() bool }); ok {
		return !img.(interface{ Opaque() bool }).Opaque()
	}
	return true
}

func cmdUpload(argv []string) error {
	o, err := parseArgs(argv, 120*time.Second, "force")
	if err != nil {
		return err
	}
	if helpOr(o, uploadHelp) {
		return nil
	}
	if err := o.only("key", "max", "quality", "force", "base"); err != nil {
		return err
	}
	if len(o.pos) == 0 {
		return errors.New("usage: penpot-cli upload <image|dir>... [--key nama] [--max 1080]")
	}
	maxSide, quality := 0, 85
	if v := o.flags["max"]; v != "" {
		if maxSide, err = strconv.Atoi(v); err != nil {
			return fmt.Errorf("--max: %w", err)
		}
	}
	if v := o.flags["quality"]; v != "" {
		if quality, err = strconv.Atoi(v); err != nil {
			return fmt.Errorf("--quality: %w", err)
		}
	}
	// kumpulkan file (folder → rekursif), key = path relatif tanpa ekstensi
	type item struct{ path, key string }
	var items []item
	for _, p := range o.pos {
		p = expandHome(p)
		st, err := os.Stat(p)
		if err != nil {
			return err
		}
		if !st.IsDir() {
			items = append(items, item{p, mediaKey(p)})
			continue
		}
		if o.flags["base"] != "" {
			base := expandHome(o.flags["base"])
			filepath.WalkDir(p, func(f string, d fs.DirEntry, err error) error {
				if err == nil && !d.IsDir() && imgExt[strings.ToLower(filepath.Ext(f))] {
					rel, _ := filepath.Rel(base, f)
					items = append(items, item{f, filepath.ToSlash(rel)})
				}
				return nil
			})
			continue
		}
		filepath.WalkDir(p, func(f string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() && imgExt[strings.ToLower(filepath.Ext(f))] {
				rel, _ := filepath.Rel(p, f)
				items = append(items, item{f, strings.TrimSuffix(filepath.ToSlash(rel), filepath.Ext(rel))})
			}
			return nil
		})
	}
	if b := o.flags["base"]; b != "" { // key = path relatif ke base, DENGAN ekstensi (= url fill di .pen)
		base := expandHome(b)
		for i := range items {
			if rel, err := filepath.Rel(base, items[i].path); err == nil && !strings.HasPrefix(rel, "..") {
				items[i].key = filepath.ToSlash(rel)
			}
		}
	}
	if k := o.flags["key"]; k != "" {
		if len(items) != 1 {
			return errors.New("--key hanya untuk satu file")
		}
		items[0].key = k
	}
	known, err := mediaManifest(o.timeout)
	if err != nil {
		return err
	}
	results := []map[string]any{}
	var sent, skipped, before, after int
	for _, it := range items {
		data, err := os.ReadFile(it.path)
		if err != nil {
			return err
		}
		sum := sha1.Sum(data)
		sha := hex.EncodeToString(sum[:]) + fmt.Sprintf("@%d", maxSide)
		if known[it.key] == sha && o.flags["force"] == "" {
			skipped++
			results = append(results, map[string]any{"key": it.key, "status": "skip (sudah ada)"})
			continue
		}
		mime := ""
		before += len(data)
		if maxSide > 0 {
			if data, mime, err = shrink(it.path, data, maxSide, quality); err != nil {
				return fmt.Errorf("%s: %w", it.path, err)
			}
		} else if cfg, format, err := image.DecodeConfig(bytes.NewReader(data)); err == nil {
			_ = cfg
			mime = "image/" + format
		} else if strings.EqualFold(filepath.Ext(it.path), ".webp") {
			mime = "image/webp"
		} else {
			return fmt.Errorf("%s: bukan gambar yang valid: %w", it.path, err)
		}
		after += len(data)
		if len(data) > 10<<20 {
			return fmt.Errorf("%s: %d bytes > 10 MiB (pakai --max)", it.path, len(data))
		}
		if _, err := uploadMedia(it.path, data, mime, it.key, o.timeout); err != nil {
			return fmt.Errorf("%s: %w", it.path, err)
		}
		if err := recordManifest(it.key, sha, o.timeout); err != nil {
			return fmt.Errorf("manifest %s: %w", it.key, err)
		}
		sent++
		results = append(results, map[string]any{"key": it.key, "kb": len(data) >> 10})
	}
	if len(items) == 1 && sent == 1 {
		out, _ := json.MarshalIndent(map[string]any{"key": items[0].key, "use": fmt.Sprintf("storage.media[%q]", items[0].key)}, "", "  ")
		fmt.Println(string(out))
		return nil
	}
	for _, r := range results {
		fmt.Printf("%-60v %v\n", r["key"], func() any {
			if s, ok := r["status"]; ok {
				return s
			}
			return fmt.Sprintf("%v KB", r["kb"])
		}())
	}
	fmt.Printf("\n%d diupload, %d dilewati", sent, skipped)
	if before > 0 {
		fmt.Printf(" · %.1f MB → %.1f MB", float64(before)/1e6, float64(after)/1e6)
	}
	fmt.Println()
	return nil
}
