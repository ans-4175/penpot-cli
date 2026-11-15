package main

// Primitive lokal (tanpa MCP/daemon) untuk code-to-design & design-to-code.
// Port apa adanya dari ~/src/penpot-cli (penpot CLI lama).

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type sourceFile struct {
	Path               string         `json:"path"`
	Components         []string       `json:"components,omitempty"`
	JSXComponents      []string       `json:"jsxComponents,omitempty"`
	TailwindClasses    []string       `json:"tailwindClasses,omitempty"`
	TailwindCategories map[string]int `json:"tailwindCategories,omitempty"`
}

type sourceManifest struct {
	SchemaVersion string       `json:"schemaVersion"`
	Framework     string       `json:"framework"`
	Root          string       `json:"root"`
	Files         []sourceFile `json:"files"`
	Summary       struct {
		Files              int            `json:"files"`
		Components         []string       `json:"components"`
		TailwindClasses    []string       `json:"tailwindClasses"`
		TailwindCategories map[string]int `json:"tailwindCategories"`
	} `json:"summary"`
}

type projectManifest struct {
	Root           string            `json:"root"`
	Name           string            `json:"name,omitempty"`
	PackageManager string            `json:"packageManager,omitempty"`
	Frameworks     []string          `json:"frameworks"`
	Styling        []string          `json:"styling"`
	Language       []string          `json:"language"`
	Scripts        map[string]string `json:"scripts,omitempty"`
	UIPaths        []string          `json:"uiPaths"`
	ConfigFiles    []string          `json:"configFiles"`
}

var (
	componentRE    = regexp.MustCompile(`(?m)(?:export\s+)?(?:default\s+)?(?:function|const)\s+([A-Z][A-Za-z0-9_]*)`)
	classNameRE    = regexp.MustCompile(`className\s*=\s*(?:"([^"]*)"|'([^']*)')`)
	jsxComponentRE = regexp.MustCompile(`<([A-Z][A-Za-z0-9.]*)\b`)
)

func uniqueSorted(values []string) []string {
	set := map[string]bool{}
	for _, value := range values {
		if value != "" {
			set[value] = true
		}
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func tailwindCategory(class string) string {
	base := class[strings.LastIndex(class, ":")+1:]
	switch {
	case strings.HasPrefix(base, "bg-"), strings.HasPrefix(base, "border-"), strings.HasPrefix(base, "ring-"), strings.HasPrefix(base, "from-"), strings.HasPrefix(base, "to-"), strings.HasPrefix(base, "via-"):
		return "color"
	case strings.HasPrefix(base, "text-") && !regexp.MustCompile(`^text-(xs|sm|base|lg|xl|[2-9]xl|\[)`).MatchString(base):
		return "color"
	case strings.HasPrefix(base, "p-"), strings.HasPrefix(base, "px-"), strings.HasPrefix(base, "py-"), strings.HasPrefix(base, "pt-"), strings.HasPrefix(base, "pr-"), strings.HasPrefix(base, "pb-"), strings.HasPrefix(base, "pl-"), strings.HasPrefix(base, "m-"), strings.HasPrefix(base, "mx-"), strings.HasPrefix(base, "my-"), strings.HasPrefix(base, "gap-"), strings.HasPrefix(base, "space-"):
		return "spacing"
	case strings.HasPrefix(base, "rounded"):
		return "radius"
	case strings.HasPrefix(base, "shadow"):
		return "shadow"
	case strings.HasPrefix(base, "font-"), strings.HasPrefix(base, "leading-"), strings.HasPrefix(base, "tracking-"), strings.HasPrefix(base, "text-"):
		return "typography"
	case strings.HasPrefix(base, "flex"), strings.HasPrefix(base, "grid"), strings.HasPrefix(base, "items-"), strings.HasPrefix(base, "justify-"), strings.HasPrefix(base, "place-"):
		return "layout"
	case strings.HasPrefix(base, "w-"), strings.HasPrefix(base, "h-"), strings.HasPrefix(base, "min-"), strings.HasPrefix(base, "max-"):
		return "sizing"
	default:
		return "other"
	}
}

func scanReactTailwind(path string) (sourceManifest, error) {
	root, err := filepath.Abs(path)
	if err != nil {
		return sourceManifest{}, err
	}
	manifest := sourceManifest{SchemaVersion: "1", Framework: "react-tailwind", Root: root}
	allComponents, allClasses := []string{}, []string{}
	allCategories := map[string]int{}
	readFile := func(file string) error {
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		if len(data) > 1<<20 {
			return nil
		}
		rel, err := filepath.Rel(root, file)
		if err != nil || rel == "." {
			rel = filepath.Base(file)
		}
		record := sourceFile{Path: filepath.ToSlash(rel), TailwindCategories: map[string]int{}}
		for _, match := range componentRE.FindAllStringSubmatch(string(data), -1) {
			record.Components = append(record.Components, match[1])
		}
		for _, match := range jsxComponentRE.FindAllStringSubmatch(string(data), -1) {
			record.JSXComponents = append(record.JSXComponents, match[1])
		}
		for _, match := range classNameRE.FindAllStringSubmatch(string(data), -1) {
			classes := match[1]
			if classes == "" {
				classes = match[2]
			}
			for _, class := range strings.Fields(classes) {
				record.TailwindClasses = append(record.TailwindClasses, class)
				record.TailwindCategories[tailwindCategory(class)]++
				allCategories[tailwindCategory(class)]++
			}
		}
		record.Components = uniqueSorted(record.Components)
		record.JSXComponents = uniqueSorted(record.JSXComponents)
		record.TailwindClasses = uniqueSorted(record.TailwindClasses)
		if len(record.Components) == 0 && len(record.JSXComponents) == 0 && len(record.TailwindClasses) == 0 {
			return nil
		}
		manifest.Files = append(manifest.Files, record)
		allComponents = append(allComponents, record.Components...)
		allClasses = append(allClasses, record.TailwindClasses...)
		return nil
	}
	info, err := os.Stat(root)
	if err != nil {
		return manifest, err
	}
	if !info.IsDir() {
		if err := readFile(root); err != nil {
			return manifest, err
		}
	} else {
		skip := map[string]bool{".git": true, ".next": true, "build": true, "coverage": true, "dist": true, "node_modules": true, "vendor": true}
		err = filepath.WalkDir(root, func(file string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() && skip[entry.Name()] {
				return filepath.SkipDir
			}
			if entry.IsDir() {
				return nil
			}
			switch strings.ToLower(filepath.Ext(file)) {
			case ".tsx", ".jsx", ".ts", ".js":
				return readFile(file)
			}
			return nil
		})
		if err != nil {
			return manifest, err
		}
	}
	sort.Slice(manifest.Files, func(i, j int) bool { return manifest.Files[i].Path < manifest.Files[j].Path })
	manifest.Summary.Files = len(manifest.Files)
	manifest.Summary.Components = uniqueSorted(allComponents)
	manifest.Summary.TailwindClasses = uniqueSorted(allClasses)
	manifest.Summary.TailwindCategories = allCategories
	return manifest, nil
}

func existingPaths(root string, paths ...string) []string {
	result := []string{}
	for _, path := range paths {
		if info, err := os.Stat(filepath.Join(root, path)); err == nil && info.IsDir() {
			result = append(result, filepath.ToSlash(path))
		}
	}
	return result
}

func existingFiles(root string, paths ...string) []string {
	result := []string{}
	for _, path := range paths {
		if _, err := os.Stat(filepath.Join(root, path)); err == nil {
			result = append(result, filepath.ToSlash(path))
		}
	}
	return result
}

func inspectProject(path string) (projectManifest, error) {
	root, err := filepath.Abs(path)
	if err != nil {
		return projectManifest{}, err
	}
	if info, err := os.Stat(root); err != nil {
		return projectManifest{}, err
	} else if !info.IsDir() {
		root = filepath.Dir(root)
	}
	manifest := projectManifest{Root: root, Frameworks: []string{}, Styling: []string{}, Language: []string{}, UIPaths: []string{}, ConfigFiles: []string{}}
	if data, err := os.ReadFile(filepath.Join(root, "package.json")); err == nil {
		var pkg struct {
			Name            string            `json:"name"`
			Scripts         map[string]string `json:"scripts"`
			Dependencies    map[string]string `json:"dependencies"`
			DevDependencies map[string]string `json:"devDependencies"`
		}
		if err := json.Unmarshal(data, &pkg); err != nil {
			return manifest, fmt.Errorf("decode package.json: %w", err)
		}
		manifest.Name, manifest.Scripts = pkg.Name, pkg.Scripts
		deps := map[string]bool{}
		for name := range pkg.Dependencies {
			deps[name] = true
		}
		for name := range pkg.DevDependencies {
			deps[name] = true
		}
		if deps["next"] {
			manifest.Frameworks = append(manifest.Frameworks, "next")
		}
		if deps["react"] {
			manifest.Frameworks = append(manifest.Frameworks, "react")
		}
		if deps["vite"] {
			manifest.Frameworks = append(manifest.Frameworks, "vite")
		}
		if deps["@remix-run/react"] {
			manifest.Frameworks = append(manifest.Frameworks, "remix")
		}
		if deps["tailwindcss"] {
			manifest.Styling = append(manifest.Styling, "tailwind")
		}
		if deps["styled-components"] {
			manifest.Styling = append(manifest.Styling, "styled-components")
		}
		if deps["@emotion/react"] {
			manifest.Styling = append(manifest.Styling, "emotion")
		}
		if deps["sass"] {
			manifest.Styling = append(manifest.Styling, "sass")
		}
	}
	if len(existingFiles(root, "pnpm-lock.yaml")) > 0 {
		manifest.PackageManager = "pnpm"
	}
	if len(existingFiles(root, "yarn.lock")) > 0 {
		manifest.PackageManager = "yarn"
	}
	if len(existingFiles(root, "package-lock.json")) > 0 {
		manifest.PackageManager = "npm"
	}
	if len(existingFiles(root, "bun.lockb", "bun.lock")) > 0 {
		manifest.PackageManager = "bun"
	}
	manifest.ConfigFiles = existingFiles(root, "package.json", "tsconfig.json", "tailwind.config.js", "tailwind.config.ts", "vite.config.ts", "vite.config.js", "next.config.js", "next.config.mjs", "next.config.ts")
	if len(existingFiles(root, "tsconfig.json")) > 0 {
		manifest.Language = append(manifest.Language, "typescript")
	}
	manifest.UIPaths = existingPaths(root, "app", "pages", "src/app", "src/pages", "components", "src/components", "src/features", "src/ui", "src/styles")
	manifest.Frameworks, manifest.Styling, manifest.Language = uniqueSorted(manifest.Frameworks), uniqueSorted(manifest.Styling), uniqueSorted(manifest.Language)
	return manifest, nil
}

const sourceHelp = `penpot-cli source — inventaris React/Tailwind statis (offline, tanpa MCP)

Usage:
  penpot-cli source scan <path> [--framework react-tailwind] [--out manifest.json] [--json]

Deklarasi komponen, komponen JSX yang dipakai, dan className Tailwind statis.
Heuristik: class dinamis/runtime tetap butuh baca source + QA visual.
`

const projectHelp = `penpot-cli project — deteksi konvensi repo target (offline, tanpa MCP)

Usage:
  penpot-cli project inspect <path>

Framework, styling, bahasa, package manager, scripts, path UI, file config.
Dipakai sebelum design-to-code.
`

func cmdSource(argv []string) error {
	o, err := parseArgs(argv, 0)
	if err != nil {
		return err
	}
	if helpOr(o, sourceHelp) {
		return nil
	}
	if err := o.only("framework", "out"); err != nil {
		return err
	}
	if len(o.pos) != 2 || o.pos[0] != "scan" {
		return errors.New("usage: penpot-cli source scan <path> [--framework react-tailwind] [--out manifest.json]")
	}
	if fw := o.flags["framework"]; fw != "" && fw != "react-tailwind" {
		return fmt.Errorf("framework %q belum didukung; tersedia: react-tailwind", fw)
	}
	m, err := scanReactTailwind(expandHome(o.pos[1]))
	if err != nil {
		return err
	}
	data, _ := json.MarshalIndent(m, "", "  ")
	if out := expandHome(o.flags["out"]); out != "" {
		if dir := filepath.Dir(out); dir != "." {
			os.MkdirAll(dir, 0o755)
		}
		if err := os.WriteFile(out, append(data, '\n'), 0o644); err != nil {
			return err
		}
	}
	if o.json {
		fmt.Println(string(data))
		return nil
	}
	fmt.Printf("%d file, %d komponen, %d class Tailwind statis\n", m.Summary.Files, len(m.Summary.Components), len(m.Summary.TailwindClasses))
	if out := o.flags["out"]; out != "" {
		fmt.Println(out)
	}
	return nil
}

func cmdProject(argv []string) error {
	o, err := parseArgs(argv, 0)
	if err != nil {
		return err
	}
	if helpOr(o, projectHelp) {
		return nil
	}
	if len(o.pos) != 2 || o.pos[0] != "inspect" {
		return errors.New("usage: penpot-cli project inspect <path>")
	}
	m, err := inspectProject(expandHome(o.pos[1]))
	if err != nil {
		return err
	}
	data, _ := json.MarshalIndent(m, "", "  ")
	fmt.Println(string(data))
	return nil
}
