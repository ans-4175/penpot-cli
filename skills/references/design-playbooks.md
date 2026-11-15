# Design playbooks (per artifact)

Compact playbooks for common artifact types. Each assumes Workflow B rules (SKILL.md)
(small reversible sections, semantic names, verify per section) and pairs with
[style-presets.md](style-presets.md) for direction and
[autolayout-antipatterns.md](autolayout-antipatterns.md) for pitfalls.

## Landing page (desktop 1440 + mobile 390 boards)

- Section order that converts: Navbar → Hero → Social proof → Problem →
  Solution/features → Differentiator → Testimonials → Pricing → FAQ → Final
  CTA → Footer. Cut sections rather than inventing filler.
- Hero: one promise, one primary CTA, one secondary. Social proof badge above
  the fold when real numbers exist.
- Mobile board is not a squeezed desktop: reflow to single column,CTA
  full-width, stack proof cards.
- Verify: hero reads in 3 seconds; CTA visible without scroll on mobile.

## Web app / dashboard (1440)

- Shell first: sidebar (240) + topbar + content area, then screens inside.
- Density: table rows 44–56, cards padding 16–24, one accent for actions.
- Every list needs empty, loading, and error states — even static wireframes
  should show them as separate small frames.
- Verify: primary action reachable in one glance; numbers aligned right,
  monospaced-feel tabular.

## Mobile app (390×844 frames)

- Thumb zone: primary actions bottom 25% of the screen.
- One task per screen; navigation via bottom bar (3–5 items).
- Status bar: leave top 50px free of interactive content.
- Verify: no tap target under 44×44; no horizontal overflow.

## Slides (1920×1080)

- One message per slide; headline is the takeaway, not the topic.
- Type scale: title 64+, body 32+ (projection distance).
- Consistent margins (120+), consistent footer/page number position.
- Verify: readable from 3 meters (zoom out to 30%).

## Dense table / data view

- Column order: identity → key value → status → actions.
- Status = colored chip (semantic: green ok, amber warn, red fail).
- Alignment: text left, numbers right, actions right.
- Verify: headers visible on scroll or repeated; row hover state defined.
