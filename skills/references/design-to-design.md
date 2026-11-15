# Design-to-design workflow

Use this workflow for **image/screenshot/wireframe → editable Penpot hi-fi**. The output is not a flattened image: it must use semantic boards, reusable components, and styles/tokens where repetition exists.

## Contract

**Input:** local PNG/JPEG/GIF screenshot, wireframe, or an image visible to the agent.

**Output:** a named Penpot page/board with:

- a locked source/reference board retained for QA;
- an editable hi-fi board built from Penpot shapes;
- semantic components for repeated UI;
- a documented token proposal or applied local-library token set;
- an exported PNG checkpoint and a visual comparison artifact.

Do not promise pixel-perfect recreation from a screenshot alone. Infer the design system, preserve editable intent, and iterate section-by-section.

## 0. Preconditions

1. Confirm the user wants a reconstruction, not merely a pasted screenshot.
2. Confirm the correct Penpot file is connected through **File → MCP Server → Connect**.
3. Run:

```bash
penpot-cli doctor
penpot-cli overview
penpot-cli state
```

4. For a multi-step write, state the board/page plan and get confirmation before creating or changing designs.
5. Never modify unrelated existing boards. Create a dedicated page for the clone/build.

## 1. Bring in the source

For a local image, make it a locked reference board on the dedicated page:

```bash
penpot-cli reference import ./reference-mobile.png \
  --name 'Reference — Mobile landing' --x 0 --y 0
```

The command supports PNG, JPEG, and GIF up to 10 MiB. It embeds the asset with Penpot's media API and locks the board so it cannot be edited accidentally.

For an image/board that already exists on the focused Penpot page, preserve the asset without re-uploading it:

```bash
penpot-cli reference clone <shape-id|selection> --name 'Reference — Existing asset'
```

When a screenshot is only attached in chat/browser output, analyse it visually first. Save/capture it locally before importing whenever visual overlay or repeatable QA is needed.

## 2. Infer before drawing

Write a short internal design brief before creating shapes:

- viewport and target board dimensions;
- section order and conversion hierarchy;
- visual mood, palette, type scale, spacing rhythm, radius/shadow rules;
- asset inventory: logo, product screenshot, icon, photo, decorative background;
- repeated patterns that should become components;
- responsive/variant requirements.

Do not turn every visual group into a component. Componentize when it repeats, has variants, or is a reusable design primitive.

Recommended naming:

```text
Button / Primary
Button / Secondary
Card / Testimonial
Card / Pricing
Nav / Mobile
Input / Text / Default
```

## 3. Establish the system first

Build in this order:

1. page and target viewport board;
2. palette/type/spacing token proposal;
3. primitives (button, text styles, card, input, badge);
4. components and variants;
5. unique sections;
6. page composition.

Penpot supports components via its local library. After selecting or identifying a component root:

```bash
penpot-cli component create <shape-id> --name 'Button / Primary'
penpot-cli component list
```

Before applying actual Penpot tokens, inspect the current API with `penpot-cli api TokenCatalog` / `penpot-cli api TokenSet`; do not guess the token schema.

## 4. Build one section, then compare

Do not bulk-generate an entire marketing page from a screenshot. Work in small reversible checkpoints:

1. build navbar + hero;
2. export its board/crop;
3. compare it to the corresponding reference fold;
4. correct hierarchy, line wraps, spacing, colour, and image treatment;
5. move to the next section.

Export a board or selected section:

```bash
penpot-cli export <shape-id> --format png --mode shape --out /tmp/candidate.png
```

Create a visual QA sheet with the repository helper:

```bash
python3 <repo-penpot-cli>/scripts/visual_diff.py \
  /tmp/reference-fold.png /tmp/candidate.png \
  --out /tmp/qa-hero.png
```

The output is `Reference | Penpot export | Difference`. Compare matching folds or crops; do not use the metric as a replacement for visual judgement.

## 5. Inspect, validate, and hand off

Use a deep snapshot rather than the shallow `state` output when analysing a board or preparing later code generation:

```bash
penpot-cli snapshot <board-id> --out /tmp/design-snapshot.json
```

Check:

- the reference board remains locked;
- layer names are semantic;
- repeated elements are component instances where appropriate;
- text is real text, not outlined/image text;
- imagery is an embedded asset, not an accidental flat screenshot of the whole UI;
- exported board visually matches the intended reference at the target viewport;
- older experiments are preserved or explicitly cleaned only with user approval.

Run the section checklist in [autolayout-antipatterns.md](autolayout-antipatterns.md) after every rebuilt section, and keep frame names prefixed `WIP — ` while a board is still being worked on (remove when done — Penpot is collaborative and WIP markers signal state to the user).

Style direction: pick from [style-presets.md](style-presets.md) when the user has no brand direction. Artifact-specific structure: [design-playbooks.md](design-playbooks.md).

