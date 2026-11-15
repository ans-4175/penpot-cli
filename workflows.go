package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Wrapper workflow di atas execute_code. Semua JS dibangun di sini supaya
// agent tidak perlu menulis ulang boilerplate untuk operasi yang sering.

// jsFind: cari shape by id / "selection" di page aktif.
const jsFind = `const __find = (id) => {
  if (id === 'selection') { const s = (penpot.selection ?? [])[0]; if (!s) throw new Error('tidak ada shape terpilih'); return s; }
  const s = penpotUtils.findShapeById(id);
  if (!s) throw new Error('shape tidak ditemukan: ' + id);
  return s;
};
`

func jsInput(v any) string {
	b, _ := json.Marshal(v)
	return "const input = " + string(b) + ";\n"
}

// ---- state ----

const stateCode = `const f = penpot.currentFile, p = penpot.currentPage;
const box = s => ({ id: s.id, name: s.name, type: s.type, x: Math.round(s.x), y: Math.round(s.y), w: Math.round(s.width), h: Math.round(s.height) });
const lib = penpot.library.local;
let tokenSets = [];
try { tokenSets = (lib.tokens?.sets ?? []).map(s => ({ name: s.name, active: s.active, tokens: s.tokens.length })); } catch (e) {}
return {
  file: { id: f?.id ?? null, name: f?.name ?? null },
  page: { id: p?.id ?? null, name: p?.name ?? null },
  pages: (f?.pages ?? []).map(pg => ({ id: pg.id, name: pg.name, active: pg.id === p?.id,
    boards: (pg.root?.children ?? []).filter(s => s.type === 'board').length })),
  root: (p?.root?.children ?? []).map(s => input.depth > 0 ? { ...box(s), children: penpotUtils.shapeStructure(s, input.depth).children } : box(s)),
  selection: (penpot.selection ?? []).map(box),
  library: { components: lib.components.length, colors: lib.colors.length, typographies: lib.typographies.length, tokenSets,
    connected: (penpot.library.connected ?? []).map(l => l.name) }
};`

func cmdState(argv []string) error {
	o, err := parseArgs(argv, 60*time.Second)
	if err != nil {
		return err
	}
	if helpOr(o, stateHelp) {
		return nil
	}
	skipPin = true // state justru dipakai untuk melihat file fokus
	if err := o.only("depth"); err != nil {
		return err
	}
	depth := 0
	if v := o.flags["depth"]; v != "" {
		if depth, err = strconv.Atoi(v); err != nil {
			return fmt.Errorf("--depth: %w", err)
		}
	}
	return printCode(jsInput(map[string]any{"depth": depth})+stateCode, o)
}

// ---- overview / api ----

func cmdOverview(argv []string) error {
	o, err := parseArgs(argv, 60*time.Second, "cached")
	if err != nil {
		return err
	}
	if helpOr(o, overviewHelp) {
		return nil
	}
	if o.flags["cached"] != "" {
		fmt.Print(renderSkill("penpot-overview.md"))
		return nil
	}
	res, err := callTool("high_level_overview", nil, o.timeout)
	if err != nil {
		return err
	}
	fmt.Println(res.Text)
	return nil
}

func cmdAPI(argv []string) error {
	o, err := parseArgs(argv, 60*time.Second)
	if err != nil {
		return err
	}
	if helpOr(o, apiHelp) {
		return nil
	}
	if len(o.pos) < 1 || len(o.pos) > 2 {
		return errors.New("usage: penpot-cli api <Type> [member]")
	}
	args := map[string]any{"type": o.pos[0]}
	if len(o.pos) == 2 {
		args["member"] = o.pos[1]
	}
	res, err := callTool("penpot_api_info", args, o.timeout)
	if err != nil {
		return err
	}
	fmt.Println(res.Text)
	return nil
}

// ---- exec ----

func cmdExec(argv []string) error {
	o, err := parseArgs(argv, 120*time.Second)
	if err != nil {
		return err
	}
	if helpOr(o, execHelp) {
		return nil
	}
	if err := o.only("f", "file"); err != nil {
		return err
	}
	var code string
	file := o.flags["f"]
	if file == "" {
		file = o.flags["file"]
	}
	switch {
	case file != "":
		b, err := os.ReadFile(expandHome(file))
		if err != nil {
			return err
		}
		code = string(b)
	case len(o.pos) == 1 && o.pos[0] == "-":
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		code = string(b)
	case len(o.pos) >= 1:
		code = strings.Join(o.pos, " ")
	case stdinPiped():
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		code = string(b)
	default:
		return errors.New("usage: penpot-cli exec '<js>' | -f file.js | - (stdin)")
	}
	if strings.TrimSpace(code) == "" {
		return errors.New("kode kosong")
	}
	return printCode(code, o)
}

// ---- export ----

func cmdExport(argv []string) error {
	o, err := parseArgs(argv, 120*time.Second)
	if err != nil {
		return err
	}
	if helpOr(o, exportHelp) {
		return nil
	}
	if err := o.only("format", "mode", "out"); err != nil {
		return err
	}
	if len(o.pos) != 1 {
		return errors.New("usage: penpot-cli export <shapeId|selection|page> [--format png|svg] [--mode shape|fill] [--out FILE]")
	}
	args := map[string]any{"shapeId": o.pos[0]}
	if v := o.flags["format"]; v != "" {
		args["format"] = v
	}
	if v := o.flags["mode"]; v != "" {
		args["mode"] = v
	}
	res, err := callTool("export_shape", args, o.timeout)
	if err != nil {
		return err
	}
	out := expandHome(o.flags["out"])
	// SVG kadang datang sebagai teks — tulis ke file juga.
	if len(res.Images) == 0 && o.flags["format"] == "svg" && strings.Contains(res.Text, "<svg") {
		if out == "" {
			out = defaultName(o.pos[0], "svg")
		}
		if err := os.WriteFile(out, []byte(res.Text), 0o644); err != nil {
			return err
		}
		fmt.Println(out)
		return nil
	}
	if len(res.Images) == 0 {
		return fmt.Errorf("export tidak mengembalikan gambar: %s", res.Text)
	}
	files, err := writeImages(res.Images, out, o.pos[0])
	for _, f := range files {
		fmt.Println(f)
	}
	return err
}

func defaultName(stem, ext string) string {
	stem = strings.NewReplacer("/", "-", " ", "-").Replace(stem)
	if len(stem) > 12 {
		stem = stem[:12]
	}
	return fmt.Sprintf("penpot-%s-%s.%s", stem, time.Now().Format("150405"), ext)
}

func extFor(mime string) string {
	switch {
	case strings.Contains(mime, "svg"):
		return "svg"
	case strings.Contains(mime, "jpeg"), strings.Contains(mime, "jpg"):
		return "jpg"
	case strings.Contains(mime, "webp"):
		return "webp"
	}
	return "png"
}

func writeImages(imgs []imagePart, out, stem string) ([]string, error) {
	var files []string
	for i, im := range imgs {
		data, err := base64.StdEncoding.DecodeString(im.Data)
		if err != nil {
			return files, fmt.Errorf("decode gambar %d: %w", i, err)
		}
		path := out
		switch {
		case path == "":
			path = defaultName(stem, extFor(im.MimeType))
		case len(imgs) > 1:
			ext := filepath.Ext(path)
			path = fmt.Sprintf("%s-%d%s", strings.TrimSuffix(path, ext), i+1, ext)
		}
		if dir := filepath.Dir(path); dir != "." {
			os.MkdirAll(dir, 0o755)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return files, err
		}
		files = append(files, path)
	}
	return files, nil
}

// ---- snapshot ----

const snapshotJS = `const fill = f => f && ({ color: f.fillColor ?? null, opacity: f.fillOpacity ?? null,
  gradient: f.fillColorGradient ? { type: f.fillColorGradient.type, stops: f.fillColorGradient.stops } : null,
  image: f.fillImage ? { id: f.fillImage.id, name: f.fillImage.name ?? null, w: f.fillImage.width, h: f.fillImage.height } : null });
const stroke = s => s && ({ color: s.strokeColor ?? null, width: s.strokeWidth ?? null, opacity: s.strokeOpacity ?? null, style: s.strokeStyle ?? null, align: s.strokeAlignment ?? null });
const layout = s => {
  if (s.flex) { const l = s.flex; return { type: 'flex', dir: l.dir, rowGap: l.rowGap, columnGap: l.columnGap, alignItems: l.alignItems, justifyContent: l.justifyContent,
    padding: [l.topPadding, l.rightPadding, l.bottomPadding, l.leftPadding], wrap: l.wrap ?? null, hSizing: l.horizontalSizing, vSizing: l.verticalSizing }; }
  if (s.grid) { const g = s.grid; return { type: 'grid', rows: g.rows, columns: g.columns, rowGap: g.rowGap, columnGap: g.columnGap,
    padding: [g.topPadding, g.rightPadding, g.bottomPadding, g.leftPadding] }; }
  return null;
};
const child = s => { const c = s.layoutChild; return c ? { absolute: c.absolute, hSizing: c.horizontalSizing, vSizing: c.verticalSizing } : null; };
const snap = s => {
  const comp = typeof s.component === 'function' ? s.component() : null;
  const n = { id: s.id, name: s.name, type: s.type, x: s.x, y: s.y, w: s.width, h: s.height,
    hidden: s.hidden || undefined, blocked: s.blocked || undefined, opacity: s.opacity !== 1 ? s.opacity : undefined, rotation: s.rotation || undefined,
    radius: s.borderRadius || undefined, fills: Array.isArray(s.fills) && s.fills.length ? s.fills.map(fill) : undefined,
    strokes: Array.isArray(s.strokes) && s.strokes.length ? s.strokes.map(stroke) : undefined,
    shadows: Array.isArray(s.shadows) && s.shadows.length ? s.shadows : undefined,
    layout: layout(s) ?? undefined, layoutChild: s.parent && (s.parent.flex || s.parent.grid) ? child(s) : undefined,
    component: comp ? { id: comp.id, name: comp.name, main: s.isComponentMainInstance?.() ?? false } : undefined,
    tokens: s.tokens && Object.keys(s.tokens).length ? s.tokens : undefined };
  if (s.type === 'text') Object.assign(n, { text: s.characters, fontFamily: s.fontFamily, fontSize: s.fontSize, fontWeight: s.fontWeight,
    lineHeight: s.lineHeight, letterSpacing: s.letterSpacing, align: s.align, verticalAlign: s.verticalAlign, growType: s.growType });
  if (s.type === 'board') n.clip = s.clipContent;
  if (s.children?.length) n.children = s.children.map(snap);
  return n;
};
`

func snapshotCode(target string) string {
	return jsInput(map[string]any{"id": target}) + jsFind + snapshotJS +
		`return { file: penpot.currentFile?.name ?? null, page: penpot.currentPage?.name ?? null, snapshot: snap(__find(input.id)) };`
}

func cmdSnapshot(argv []string) error {
	o, err := parseArgs(argv, 120*time.Second)
	if err != nil {
		return err
	}
	if helpOr(o, snapshotHelp) {
		return nil
	}
	if err := o.only("out"); err != nil {
		return err
	}
	if len(o.pos) != 1 {
		return errors.New("usage: penpot-cli snapshot <shapeId|selection> [--out FILE]")
	}
	raw, _, err := runCode(snapshotCode(o.pos[0]), o.timeout)
	if err != nil {
		return err
	}
	if out := o.flags["out"]; out != "" {
		var buf bytes.Buffer
		json.Indent(&buf, raw, "", "  ")
		if err := os.WriteFile(expandHome(out), append(buf.Bytes(), '\n'), 0o644); err != nil {
			return err
		}
		fmt.Println(out)
		return nil
	}
	printJSON(raw)
	return nil
}

// ---- reference ----

const referenceImportJS = `const media = storage.media[input.key];
const board = penpot.createBoard();
board.name = input.name;
board.resize(input.w, input.h);
board.x = input.x; board.y = input.y;
board.fills = [{ fillOpacity: 1, fillImage: media }];
board.clipContent = true;
board.blocked = true;
board.setPluginData('penpot-cli.reference', JSON.stringify({ source: input.file, importedAt: new Date().toISOString() }));
return { id: board.id, name: board.name, x: board.x, y: board.y, w: board.width, h: board.height };`

const referenceCloneJS = `const src = __find(input.id);
const copy = src.clone();
(src.parent ?? penpot.root).appendChild(copy);
copy.name = input.name || ('Reference — ' + src.name);
copy.x = input.x ?? (src.x + src.width + 80);
copy.y = input.y ?? src.y;
copy.blocked = true;
copy.setPluginData('penpot-cli.reference', JSON.stringify({ clonedFrom: src.id, importedAt: new Date().toISOString() }));
return { id: copy.id, name: copy.name, sourceId: src.id, x: copy.x, y: copy.y, w: copy.width, h: copy.height };`

func cmdReference(argv []string) error {
	o, err := parseArgs(argv, 180*time.Second)
	if err != nil {
		return err
	}
	if helpOr(o, referenceHelp) {
		return nil
	}
	if err := o.only("name", "x", "y"); err != nil {
		return err
	}
	if len(o.pos) != 2 {
		fmt.Fprint(os.Stderr, referenceHelp)
		return errors.New("butuh import <image> atau clone <id|selection>")
	}
	num := func(k string) (*float64, error) {
		v, ok := o.flags[k]
		if !ok {
			return nil, nil
		}
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return nil, fmt.Errorf("--%s: %w", k, err)
		}
		return &f, nil
	}
	x, err := num("x")
	if err != nil {
		return err
	}
	y, err := num("y")
	if err != nil {
		return err
	}
	switch o.pos[0] {
	case "import":
		path := expandHome(o.pos[1])
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if len(data) > 10<<20 {
			return fmt.Errorf("gambar %d bytes; batas 10 MiB — kecilkan dulu", len(data))
		}
		cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			return fmt.Errorf("bukan PNG/JPEG/GIF yang valid: %w", err)
		}
		name := o.flags["name"]
		if name == "" {
			name = "Reference — " + filepath.Base(path)
		}
		zero := 0.0
		if x == nil {
			x = &zero
		}
		if y == nil {
			y = &zero
		}
		key, err := uploadMedia(path, data, "image/"+format, "", o.timeout)
		if err != nil {
			return err
		}
		in := map[string]any{"file": filepath.Base(path), "key": key, "w": cfg.Width, "h": cfg.Height, "x": *x, "y": *y, "name": name}
		return printCode(jsInput(in)+referenceImportJS, o)
	case "clone":
		in := map[string]any{"id": o.pos[1], "name": o.flags["name"], "x": x, "y": y}
		return printCode(jsInput(in)+jsFind+referenceCloneJS, o)
	}
	return errors.New("action reference: import | clone")
}

// ---- component ----

const componentListJS = `const lib = penpot.library.local;
return { library: lib.name, components: lib.components.map(c => ({ id: c.id, name: c.name, path: c.path, variant: c.isVariant?.() ?? false })) };`

const componentCreateJS = `const s = __find(input.id);
if (input.name) s.name = input.name;
const c = penpot.library.local.createComponent([s]);
if (input.name) c.name = input.name;
return { id: c.id, name: c.name, path: c.path, sourceShapeId: s.id };`

func cmdComponent(argv []string) error {
	o, err := parseArgs(argv, 60*time.Second)
	if err != nil {
		return err
	}
	if helpOr(o, componentHelp) {
		return nil
	}
	if err := o.only("name"); err != nil {
		return err
	}
	switch {
	case len(o.pos) == 1 && o.pos[0] == "list":
		return printCode(componentListJS, o)
	case len(o.pos) == 2 && o.pos[0] == "create":
		return printCode(jsInput(map[string]any{"id": o.pos[1], "name": o.flags["name"]})+jsFind+componentCreateJS, o)
	}
	fmt.Fprint(os.Stderr, componentHelp)
	return errors.New("butuh list | create <id|selection>")
}

// ---- codegen ----

const codegenJS = `const t = __find(input.id);
return { file: penpot.currentFile?.name ?? null, page: penpot.currentPage?.name ?? null,
  target: { id: t.id, name: t.name, type: t.type, w: t.width, h: t.height },
  markup: penpot.generateMarkup([t], { type: 'html' }) ?? '',
  styles: penpot.generateStyle([t], { type: 'css', withPrelude: true, includeChildren: true }) ?? '' };`

func cmdCodegen(argv []string) error {
	o, err := parseArgs(argv, 180*time.Second)
	if err != nil {
		return err
	}
	if helpOr(o, codegenHelp) {
		return nil
	}
	if err := o.only("out"); err != nil {
		return err
	}
	out := expandHome(o.flags["out"])
	if len(o.pos) != 2 || o.pos[0] != "bundle" || out == "" {
		return errors.New("usage: penpot-cli codegen bundle <shapeId|selection> --out DIR")
	}
	in := jsInput(map[string]any{"id": o.pos[1]}) + jsFind
	bundleRaw, _, err := runCode(in+codegenJS, o.timeout)
	if err != nil {
		return err
	}
	snapRaw, _, err := runCode(snapshotCode(o.pos[1]), o.timeout)
	if err != nil {
		return err
	}
	var b struct {
		File, Page     any
		Target         map[string]any
		Markup, Styles string
	}
	if err := json.Unmarshal(bundleRaw, &b); err != nil {
		return fmt.Errorf("decode bundle: %w", err)
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	var design bytes.Buffer
	json.Indent(&design, snapRaw, "", "  ")
	manifest, _ := json.MarshalIndent(map[string]any{"file": b.File, "page": b.Page, "target": b.Target,
		"generatedAt": time.Now().Format(time.RFC3339),
		"note":        "markup/CSS Penpot = evidence untuk agent yang mengadaptasi ke repo target, bukan kode produksi."}, "", "  ")
	files := []struct {
		name string
		data []byte
	}{
		{"design.json", append(design.Bytes(), '\n')},
		{"markup.html", []byte(b.Markup)},
		{"styles.css", []byte(b.Styles)},
		{"manifest.json", append(manifest, '\n')},
	}
	for _, f := range files {
		p := filepath.Join(out, f.name)
		if err := os.WriteFile(p, f.data, 0o644); err != nil {
			return err
		}
		fmt.Println(p)
	}
	return nil
}

// stdinPiped: true kalau stdin berupa pipe/file (bukan terminal), supaya
// `echo 'return 1' | penpot-cli exec` jalan tanpa harus menulis `-`.
func stdinPiped() bool {
	fi, err := os.Stdin.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice == 0
}
