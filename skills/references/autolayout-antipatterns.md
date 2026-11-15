# Autolayout antipatterns & section checklist (Penpot)

Penpot flex/grid pitfalls with Plugin API fixes. Read once before large
`penpot-cli exec` builds; run the checklist after every section. API details:
[plugin-api-rules.md](plugin-api-rules.md).

## Antipatterns

### 1. Percentage / viewport sizing
Wrong: computing `"100%"`, `vh`, `calc()` widths in code.
Right: `child.layoutChild.horizontalSizing = 'fill'` inside a flex parent;
fixed px via `resize()` otherwise.

### 2. Circular auto/fill dependency
Wrong: parent `horizontalSizing = 'auto'` where every child is `'fill'` —
collapses to zero.
Right: give the parent a fixed size (`'fix'` + `resize`) on that axis, or let at
least one child be `'auto'`/`'fix'`.

### 3. Padding on text
Wrong: trying to pad a Text shape.
Right: wrap it in a board with flex layout and set `topPadding`… on the board.

### 4. Guessing text dimensions
Wrong: `resize()` on text to a guessed width/height (also forces `growType = 'fixed'`).
Right: `growType = 'auto-width'` for labels; `'auto-height'` + `layoutChild.horizontalSizing = 'fill'`
for paragraphs. Read `textBounds` after a ~100ms wait if you need the size.

### 5. Positioning children of a layout board with x/y
Wrong: setting `x`/`y` on children of a flex/grid board (layout overrides it).
Right: gaps/padding/alignment on the parent; for overlays set
`child.layoutChild.absolute = true` (x/y then relative to parent).

### 6. Adding flex to a board that already has children
Wrong: `board.addFlexLayout()` on a populated board — children get reordered.
Right: `penpotUtils.addFlexLayout(board, 'column')`.

### 7. Grids forced into one flex axis
Wrong: one row-flex board expected to wrap into columns by accident.
Right: `addGridLayout()` with explicit rows/columns, or row boards inside a
column-flex parent. (`flex.wrap = 'wrap'` exists — use it deliberately.)

### 8. Hardcoding the same width on siblings
Wrong: `resize(parentWidth, …)` on every child.
Right: `horizontalSizing = 'fill'` on children + `columnGap`/`rowGap` on the parent.

### 9. Expecting auto boards to reflow by themselves
Wrong: build children into an `auto` board and trust its size immediately.
Right: start `auto` axes at 1px, then **settle** (nudge `rotation` r+0.01 → r on
every flex board, deepest first, 2 passes). See limits-and-gotchas §4.

### 10. Rotating before the content exists
Wrong: set `rotation` on a card, then append its children (they inherit the
rotation and inflate the bbox).
Right: build + settle first, apply all rotations last.

### 11. Reordering with setParentIndex
Wrong: `shape.setParentIndex(i)` inside a flex board (no visual effect).
Right: `parent.insertChild(i, shape)`.

### 12. Mutating fill objects
Wrong: `shape.fills[0].fillColor = '#…'` (array contents are read-only).
Right: replace the array: `shape.fills = [{ fillColor: '#…', fillOpacity: 1 }]`.

## Section verification checklist

Run after every section (not at the end of the whole build):

- [ ] `penpot-cli export <section-id>` looked at — layout not collapsed, nothing at zero size
- [ ] No content clipped outside its board (`penpotUtils.analyzeDescendants` + `isContainedIn`)
- [ ] Text grows correctly (no `fixed` text overflowing after a resize)
- [ ] Contrast sufficient on text, chips, buttons
- [ ] Alignment/spacing consistent with sibling sections
- [ ] Intent matches the visual (what you built is what was planned)
- [ ] Layer names semantic; `WIP — ` prefix removed when the board is done
- [ ] Fixed by updating existing shapes — never delete-and-rebuild
- [ ] Fonts are the exact family (not "Anton SC" / "Caveat Brush" look-alikes)
- [ ] Work landed on the page the user asked for
