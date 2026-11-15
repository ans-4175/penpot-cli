package main

import (
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Skill penpot di-embed ke binary saat build — portable, bisa dibaca tanpa
// daemon/jaringan. Sumber kebenaran: ./skills/ di repo ini (juga di-symlink
// sebagai skill agent di ~/.agents/skills/penpot).
//
//go:embed skills
var skillsFS embed.FS

// skillsOrder: urutan dokumen untuk --skills --all.
var skillsOrder = []string{
	"SKILL.md",
	"references/limits-and-gotchas.md",
	"references/plugin-api-rules.md",
	"references/autolayout-antipatterns.md",
	"references/design-to-design.md",
	"references/code-to-design-react-tailwind.md",
	"references/design-to-code.md",
	"references/design-playbooks.md",
	"references/style-presets.md",
	"references/landing-formula.md",
	"penpot-overview.md",
}

const skillsHeader = `# penpot skill — snapshot embedded penpot-cli

Gabungan SKILL.md + file referensi. Untuk agent: baca SKILL.md dulu
(workflow A–G), lalu referensi per kebutuhan — tiap file diawali komentar
penanda nama file. penpot-overview.md = salinan "Penpot High-Level
Overview" dari server MCP (refresh: penpot-cli skills --sync).
`

func skillsNames() []string {
	var out []string
	var walk func(dir string)
	walk = func(dir string) {
		entries, _ := skillsFS.ReadDir(dir)
		for _, e := range entries {
			p := dir + "/" + e.Name()
			if e.IsDir() {
				walk(p)
			} else {
				out = append(out, strings.TrimPrefix(p, "skills/"))
			}
		}
	}
	walk("skills")
	return out
}

func skillsExists(name string) bool {
	_, err := skillsFS.ReadFile("skills/" + name)
	return err == nil
}

// resolveSkillName menerima nama longgar: "design-to-design",
// "references/design-to-design.md", "overview" → nama file embedded.
func resolveSkillName(arg string) (string, bool) {
	n := strings.TrimPrefix(arg, "skills/")
	if !strings.HasSuffix(n, ".md") && !strings.HasSuffix(n, ".json") {
		n += ".md"
	}
	if skillsExists(n) {
		return n, true
	}
	for _, f := range skillsNames() {
		base := filepath.Base(f)
		if base == n || strings.HasSuffix(base, "-"+n) || strings.HasPrefix(base, strings.TrimSuffix(n, ".md")) {
			return f, true
		}
	}
	return "", false
}

// writePorter menulis pen2penpot.py embedded ke path tujuan (portable:
// agent di mesin lain tidak perlu checkout repo).
func writePorter(dst string) error {
	b, err := skillsFS.ReadFile("skills/scripts/pen2penpot.py")
	if err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0o755)
}

func renderSkill(name string) string {
	b, _ := skillsFS.ReadFile("skills/" + name)
	return string(b)
}

func cmdSkills(argv []string) error {
	switch {
	case len(argv) > 0 && argv[0] == "--sync":
		return cmdSkillsSync()
	case len(argv) > 0 && argv[0] == "--porter":
		dst := "pen2penpot.py"
		if len(argv) > 1 {
			dst = expandHome(argv[1])
		}
		if err := writePorter(dst); err != nil {
			return err
		}
		fmt.Println(dst)
	case len(argv) > 0 && argv[0] == "--list":
		fmt.Println(strings.Join(skillsNames(), "\n"))
	case len(argv) > 0 && argv[0] == "--all":
		var b strings.Builder
		b.WriteString(skillsHeader)
		for _, n := range skillsOrder {
			if !skillsExists(n) {
				continue
			}
			b.WriteString("\n\n---\n\n<!-- skills/" + n + " -->\n\n")
			b.WriteString(renderSkill(n))
		}
		fmt.Print(b.String())
	case len(argv) == 0:
		fmt.Print(renderSkill("SKILL.md"))
	default:
		name, ok := resolveSkillName(argv[0])
		if !ok {
			return fmt.Errorf("skill %q tidak ada — tersedia:\n  %s\n(atau --all untuk gabungan)", argv[0], strings.Join(skillsNames(), "\n  "))
		}
		fmt.Print(renderSkill(name))
	}
	return nil
}

// cmdSkillsSync menarik ulang High-Level Overview dari server dan
// menulisnya ke ./skills/penpot-overview.md (jalankan dari root repo).
func cmdSkillsSync() error {
	if _, err := os.Stat("skills/SKILL.md"); err != nil {
		return errors.New("jalankan dari root repo penpot-cli (butuh ./skills/)")
	}
	res, err := callTool("high_level_overview", nil, 60*time.Second)
	if err != nil {
		return err
	}
	head := "<!-- Snapshot `high_level_overview` dari Penpot MCP, " + time.Now().Format("2006-01-02") +
		". Refresh: penpot-cli skills --sync, lalu rebuild. -->\n\n"
	if err := os.WriteFile("skills/penpot-overview.md", []byte(head+res.Text+"\n"), 0o644); err != nil {
		return err
	}
	fmt.Printf("OK   skills/penpot-overview.md (%d bytes)\n", len(res.Text))
	fmt.Println("\nrebuild: go build -o ~/.local/bin/penpot-cli .")
	return nil
}
