# Style presets

Curated style starting points for Penpot boards. A preset is a **starting
direction, not a brand**: adjust palette/fonts to the user's brand before
building. Apply colors/fonts explicitly per shape, or via token catalogs after
inspecting the live schema (`penpot-cli api TokenCatalog` — never guess its shape).

## How to use

1. Pick the preset closest to the product's tone; confirm with the user.
2. Apply the palette to the board's primitives (backgrounds, text, buttons).
3. Keep one accent as the dominant color; the rest are supporting neutrals.
4. Record the choice in the board name or a small style frame (swatches + type
   samples) so later sections stay consistent.

## Presets

### 1. Clean SaaS Light
- Canvas `#FFFFFF`, section alt `#F8FAFC`, border `#E2E8F0`
- Text `#0F172A` / `#475569`, accent emerald `#059669`, accent-2 blue `#2563EB`
- Font: Plus Jakarta Sans (700 headings / 400 body), radius 16, shadows rare
- Use: SaaS landing, dashboards, B2B tools

### 2. Warm Editorial
- Canvas `#FDFBF7`, alt `#F7F3EC`, border `#E7E0D4`
- Text `#292524`, accent terracotta `#C2542D`, accent-2 olive `#6B7F3E`
- Font: serif display (Lora/Playfair 700) + sans body (Inter), radius 8
- Use: storytelling landing, blogs, portfolio, kuliner brand

### 3. Dark Product
- Canvas `#0B1220`, card `#111A2C`, border `#1E2A44`
- Text `#F1F5F9` / `#94A3B8`, accent cyan `#22D3EE`, warning amber `#F59E0B`
- Font: Space Grotesk (700) + Inter body, radius 12, glow sparingly
- Use: developer tools, fintech dashboards, data products

### 4. Playful Consumer
- Canvas `#FFFBF5`, alt `#FFF1E6`, border `#FFE0C7`
- Text `#3F2E20`, accent coral `#FF6B4A`, accent-2 teal `#14B8A6`, yellow `#F2C94C`
- Font: Nunito/Baloo (800 headings), radius 24, soft shadows ok
- Use: consumer apps, promos, campaign/urgency visuals

### 5. Corporate Trust
- Canvas `#FFFFFF`, alt `#F1F5F9`, border `#D8DEE7`
- Text `#1E293B`, accent navy `#1D4ED8`, neutral gold `#B48A2F`
- Font: IBM Plex Sans (600 headings), radius 6, no playful effects
- Use: legal, finance formal, government-facing, B2B enterprise

### 6. Minimal Mono
- Canvas `#FAFAFA`, border `#E5E5E5`
- Text `#111` / `#666`, single accent black; color only for status
- Font: Inter/Neue Haas, tight tracking, radius 0–4, hairline dividers
- Use: portfolios, docs, premium minimal brands

## Rules

- One accent dominant; never light up every section with all palette colors.
- Tint neutrals toward the accent hue (e.g. emerald-tinted grays) — pure
  `#000`/`#FFF` never appears.
- Record the final palette as swatch frame + type sample on the canvas.
