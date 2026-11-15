# Design-to-code workflow

Use this workflow to implement a Penpot board in the **existing codebase**, following its framework, styling system, file layout, and component conventions. Do not force a generic React/Tailwind template onto a different project.

## Contract

**Input:** a selected Penpot board/component plus a target repository.

**Output:** reviewed source changes that reproduce the design using the repository's native patterns, with visual QA at the target viewport.

Penpot-generated HTML/CSS is evidence, not deployable application code. The agent owns semantic component mapping and adaptation to the target repository.

## 1. Inspect the repository before the design

```bash
penpot-cli project inspect . > /tmp/project-manifest.json
```

Then inspect the actual repository files named by the manifest:

- `package.json` and scripts;
- framework entrypoints/routes (`app`, `pages`, `src/app`, etc.);
- existing component directories and nearby analogous components;
- styling/theme files, tokens, globals, and Tailwind config when present;
- lint, formatting, testing, and asset conventions.

Follow the existing project. Reuse its components and tokens first. Do not add a new framework, CSS library, design system, or dependency unless the user asks.

## 2. Extract a design evidence bundle

Verify the focused Penpot file/page, then export a bundle from the target board:

```bash
penpot-cli doctor
penpot-cli overview
penpot-cli codegen bundle <board-id> --out /tmp/penpot-bundle
```

The bundle contains:

| File | Purpose |
|---|---|
| `design.json` | recursive editable design tree, text, style, component state, embedded-image metadata |
| `markup.html` | Penpot's structural HTML evidence |
| `styles.css` | Penpot's generated CSS evidence |
| `manifest.json` | board/file metadata and explicit usage note |

Read `design.json` first. Use markup/CSS only to validate geometry/style details. Export individual image-bearing shapes when an actual source asset is needed; never use a flattened full-board export as a page image.

## 3. Build an implementation map

Before edits, state a concise map:

```text
Penpot component → existing/new code component
Penpot variant → prop/state in code
Penpot token/style → existing CSS variable/theme/token/utility
Penpot board → route/page/layout entry
Penpot image fill → local/public asset strategy
```

If a component already exists in the repository, adapt/reuse it rather than recreating it. If design semantics are ambiguous, ask before guessing product behavior.

## 4. Implement in vertical slices

1. Add/reuse tokens and base primitives only when the repo’s conventions support them.
2. Implement one meaningful page section/component.
3. Run the repo’s formatter, typecheck, and focused tests.
4. Render at the Penpot target viewport.
5. Compare screenshot to an exported Penpot section/board.
6. Iterate, then proceed to the next slice.

Keep layout responsive from the start. Do not use hardcoded absolute positioning just because Penpot exposes x/y geometry.

## 5. Validate and report

Validate:

- target route renders without console/type/lint errors;
- semantic HTML, keyboard behavior, and existing accessibility conventions are preserved;
- design component variants map to code props/states;
- assets are optimized and referenced through the project’s normal asset pipeline;
- visual result matches hierarchy, spacing, typography, colour, and responsive behavior.

Report the code paths changed, verification commands, visual differences still intentional, and assumptions made.

## 6. Code-gen quality rules

When generating markup/components from the design bundle:

- Map repeated canvas structures to one component + props — never paste the
  same block N times.
- Turn board palette/repeats into tokens or theme variables first; hardcode
  hexes only for one-off accents.
- Preserve layer names as semantic class/component names (the bundle keeps
  them when `includeLayerNames` is on).
- Do not mirror absolute canvas x/y into CSS absolute positioning; rebuild
  layout with the repo’s layout system (flex/grid) from the visual hierarchy.
- Copy: transcribe exactly; do not rewrite or localize.
- Icons/photos: export the individual shape; never embed a flattened page
  screenshot.
