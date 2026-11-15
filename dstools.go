package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"
)

// settle & tokens: dua pola yang sebelumnya harus ditulis manual di exec.

const settleHelp = `penpot-cli settle <id|selection> [--passes 3]

Paksa Penpot menghitung ulang board flex "auto" (bug: board auto tidak
membesar/mengecil setelah anak ditambah lewat API). Dari board terdalam ke
luar: rotation +0.01 lalu kembali, diulang sampai ukuran stabil.
Pakai setelah build manual via exec; pen2penpot sudah melakukannya sendiri.
`

const settleJS = `const root = __find(input.id);
const all = [root, ...penpotUtils.findShapes(s => s.type === 'board', root)];
const auto = all.filter(b => b.type === 'board' && b.flex &&
  (b.flex.horizontalSizing === 'auto' || b.flex.verticalSizing === 'auto'));
const depth = s => { let d = 0; for (let p = s.parent; p; p = p.parent) d++; return d; };
auto.sort((a, b) => depth(b) - depth(a));
const dims = () => auto.map(b => Math.round(b.width) + 'x' + Math.round(b.height)).join(',');
const before = { w: Math.round(root.width), h: Math.round(root.height) };
let passes = 0, prev = dims();
for (; passes < input.passes; passes++) {
  for (const b of auto) { const r = b.rotation || 0; b.rotation = r + 0.01; b.rotation = r; }
  await new Promise(r => setTimeout(r, 80));
  const now = dims(); if (now === prev && passes > 0) { passes++; break; } prev = now;
}
return { id: root.id, name: root.name, autoBoards: auto.length, passes,
  before, after: { w: Math.round(root.width), h: Math.round(root.height) } };`

func cmdSettle(argv []string) error {
	o, err := parseArgs(argv, 120*time.Second)
	if err != nil {
		return err
	}
	if helpOr(o, settleHelp) {
		return nil
	}
	if err := o.only("passes"); err != nil {
		return err
	}
	if len(o.pos) != 1 {
		fmt.Fprint(os.Stderr, settleHelp)
		return errors.New("butuh <id|selection>")
	}
	passes := 3
	if v := o.flags["passes"]; v != "" {
		fmt.Sscanf(v, "%d", &passes)
	}
	return printCode(jsInput(map[string]any{"id": o.pos[0], "passes": passes})+jsFind+settleJS, o)
}

const tokensHelp = `penpot-cli tokens list | export [--out tokens.json]

  list     set, theme, dan token (nama · tipe · nilai → resolved)
  export   JSON bergaya W3C DTCG ({"set": {"color": {"navy": {"$type","$value"}}}})
           — untuk design-to-code / backup sebelum "Import tokens" di file lain.

Membuat/menerapkan token: exec (lihat --skills limits §7: applyToken props
array; font = ['fontFamily']; radius = 4 sudut; ubah nilai → toggle set 2×).
`

const tokensListJS = `const cat = penpot.library.local.tokens;
return { themes: cat.themes.map(t => ({ group: t.group, name: t.name, active: t.active, sets: t.activeSets.map(s => s.name) })),
  sets: cat.sets.map(s => ({ name: s.name, active: s.active,
    tokens: s.tokens.map(t => [t.name, t.type, t.value, t.resolvedValueString ?? t.resolvedValue ?? null]) })) };`

const tokensExportJS = `const cat = penpot.library.local.tokens;
const out = {};
for (const s of cat.sets) {
  const root = out[s.name] = {};
  for (const t of s.tokens) {
    const parts = t.name.split('.'); let node = root;
    for (const p of parts.slice(0, -1)) node = node[p] = node[p] || {};
    node[parts[parts.length - 1]] = { $type: t.type, $value: t.value };
  }
}
out.$themes = cat.themes.map(t => ({ group: t.group, name: t.name, selectedTokenSets: Object.fromEntries(t.activeSets.map(s => [s.name, 'enabled'])) }));
return out;`

func cmdTokens(argv []string) error {
	o, err := parseArgs(argv, 60*time.Second)
	if err != nil {
		return err
	}
	if helpOr(o, tokensHelp) {
		return nil
	}
	if err := o.only("out"); err != nil {
		return err
	}
	if len(o.pos) != 1 {
		fmt.Fprint(os.Stderr, tokensHelp)
		return errors.New("butuh list | export")
	}
	switch o.pos[0] {
	case "list":
		return printCode(tokensListJS, o)
	case "export":
		if out := o.flags["out"]; out != "" {
			raw, _, err := runCode(tokensExportJS, o.timeout)
			if err != nil {
				return err
			}
			var buf bytes.Buffer
			if json.Indent(&buf, raw, "", "  ") != nil {
				buf.Reset()
				buf.Write(raw)
			}
			buf.WriteByte('\n')
			if err := os.WriteFile(expandHome(out), buf.Bytes(), 0o644); err != nil {
				return err
			}
			fmt.Println("tokens →", out)
			return nil
		}
		return printCode(tokensExportJS, o)
	}
	fmt.Fprint(os.Stderr, tokensHelp)
	return fmt.Errorf("aksi %q tidak dikenal", o.pos[0])
}
