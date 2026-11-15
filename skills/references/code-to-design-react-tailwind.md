# Code-to-design: React + Tailwind v1

Use this workflow to turn an existing React/TSX UI into **editable Penpot components, screen structures, and pages**. It is semantic visual reconstruction, not a literal DOM dump or Tailwind-to-rectangle converter.

## Scope

First target: React/JSX with Tailwind. `penpot-cli source scan` is a deterministic inventory of static component declarations and Tailwind classes. The agent must still read the source, understand the UI, and—when possible—compare against a running route screenshot.

Do not claim to resolve runtime class construction, application data, interactions, arbitrary CSS, or browser-only layout automatically.

## Workflow

### 1. Gather visual and structural evidence

```bash
penpot-cli source scan ./src \
  --framework react-tailwind \
  --out /tmp/ui-manifest.json
```

Then read, in order:

1. route/page entry component;
2. directly rendered components and their imports;
3. Tailwind config, global CSS, and CSS-variable/theme files;
4. representative fixtures, Storybook stories, or static data;
5. the running route at its target viewport, when it can be launched safely.

The evidence answers different questions:

| Evidence | Use it for |
|---|---|
| JSX hierarchy | section, component, and page structure |
| Tailwind/config/CSS variables | token candidates and responsive rules |
| Props/conditionals | visually meaningful variants/states |
| Running screenshot | actual composition, rendered typography, and spacing |
| Fixtures/stories | representative content and edge states |

### 2. Write a design IR before touching Penpot

Create a small temporary design brief using [code-to-design-ir-template.json](code-to-design-ir-template.json). It must describe:

```text
route/screen → target Penpot board
source component → semantic Penpot component
Tailwind/theme values → token candidate
props/state → named visual variant
children/slots → nested editable content
runtime data → representative fixture
```

This is the key reasoning step. Ignore implementation-only wrappers: analytics, providers, routing shells, invisible divs, data fetches, and event plumbing. Preserve visual intent instead.

Ask for clarification if source cannot establish the target route, viewport, brand direction, or representative state.

### 3. Build the design system and component layer

1. Verify focused file/page with `penpot-cli doctor`, `penpot-cli overview`, and `penpot-cli state`.
2. Create a dedicated `Code → Design — <feature>` page; do not overwrite an existing design.
3. Establish palette, typography, spacing, radius, shadow, and breakpoint token candidates from repeated evidence.
4. Build base primitives first: Button, Input, Card, Badge, Nav, Table row, etc.
5. Promote genuine reuse to local-library components:

```bash
penpot-cli component create <shape-id> --name 'Button / Primary'
```

6. Create variants only for visual states supported by the source: default/hover/disabled, compact/expanded, selected/unselected, mobile/desktop.
7. Compose route boards from those components plus one-off section shapes.
8. Use semantic layer names reflecting product roles, not generated DOM structure.

### 4. Verify the rendered visual result

When the application can run safely, capture the actual route at the intended viewport with the approved browser workflow. Export the Penpot board and compare matching folds:

```bash
penpot-cli export <board-id> --format png --mode shape --out /tmp/penpot-screen.png
python3 <repo-penpot-cli>/scripts/visual_diff.py \
  /tmp/code-screen.png /tmp/penpot-screen.png --out /tmp/code-to-design-qa.png
```

Correct hierarchy, responsive layout, text wrapping, tokens, and asset treatment. The visual diff is a signal; the agent must judge composition and intent.

## Guardrails

- Represent runtime behavior as named static variants or prototype links only when needed.
- Do not infer secrets, user data, or inaccessible API states from source code.
- Ask before creating workspace-wide libraries or deleting/replacing existing components.
- If a source region is dynamic or ambiguous, document the assumption and use a representative static state.
