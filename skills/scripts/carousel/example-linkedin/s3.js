const pg = penpotUtils.getPageByName("Carousel"); await penpot.openPage(pg);
if (penpot.currentPage.id !== pg.id) throw new Error('page aktif salah');
if (penpot.root.children.some(c => c.name.startsWith('Slide 3 '))) throw new Error('Slide 3 sudah ada');
const s = SLIDE('Slide 3 - Workspace', 2400, false, '3', '5');
CHIP(s, 'THE WORKSPACE');
const tb = BOX(s, { name: 'Title Block' });
TXT(tb, 'To scale design,', { wt: 800, size: 80, ls: -2.4, lh: 1.05, name: 'Title 1' });
TXT(tb, 'a team needs', { wt: 800, size: 80, ls: -2.4, lh: 1.05, name: 'Title 2' });
const tr = BOX(tb, { name: 'Title Row', dir: 'row', ai: 'center', pad: [10, 0, 0, 0] });
MARK(tr, 'one source.', 80, -1);
const dg = BOX(s, { name: 'Diagram', w: 'fill', gap: 14 });
const ag = BOX(dg, { name: 'The team', dir: 'row', gap: 18, pad: [18, 30, 18, 30], ai: 'center', fill: C.ink, r: 20 });
ICON(ag, 'users', 40, C.marker);
const agt = BOX(ag, { name: 'Agent text', gap: 2 });
TXT(agt, 'The team', { wt: 800, size: 38, color: C.white, name: 'Name' });
TXT(agt, 'designers + devs', { fam: J, wt: 500, size: 22, color: C.foot, name: 'Sub' });
const ar = BOX(dg, { name: 'Uses tools', dir: 'row', gap: 12, ai: 'center', pad: [0, 0, 0, 56] });
ICON(ar, 'arrow-down', 34, C.navy);
const pl = BOX(ar, { name: 'Pill', dir: 'row', pad: [6, 18, 6, 18], r: 999, stroke: C.navy, sw: 2, fill: C.white });
TXT(pl, 'works in', { wt: 700, size: 24, color: C.navy, name: 'Label' });
const fence = BOX(dg, { name: 'Workspace Fence', w: 'fill', gap: 16, pad: [26, 26, 26, 26], r: 24, stroke: C.accent, sw: 4, fill: C.white });
TXT(fence, 'THE WORKSPACE: ONE SHARED LIBRARY', { fam: J, wt: 700, size: 20, color: C.accent, ls: 1.5, name: 'Label' });
const tiles = [['palette', 'Styles', 'Colors, type, effects'], ['component', 'Components', 'Variants, ready to drop in'], ['git-branch', 'Versions', 'Review, branch, roll back'], ['users', 'Your team', 'Comments, handoff, docs']];
for (let r = 0; r < 2; r++) {
  const row = BOX(fence, { name: 'Tile Row', dir: 'row', gap: 16, w: 'fill' });
  for (const [ic, n, d] of tiles.slice(r * 2, r * 2 + 2)) {
    const t = BOX(row, { name: 'Tile ' + n, w: 'fill', gap: 10, pad: [22, 24, 22, 24], fill: C.card, r: 18, stroke: C.hair, sw: 2 });
    const h = BOX(t, { name: 'Head', dir: 'row', gap: 14, ai: 'center' });
    ICON(h, ic, 38, C.accent); TXT(h, n, { wt: 800, size: 38, ls: -0.5, name: 'Name' });
    TXT(t, d, { size: 28, color: C.sub, w: 'fill', name: 'Desc' });
  }
}
const note = BOX(s, { name: 'Fence note', dir: 'row', gap: 16, ai: 'center', pad: [6, 0, 0, 0] });
BOX(note, { name: 'Red line', w: 44, h: 6, fill: C.accent, r: 3 });
TXT(note, 'Red line = the fence: what is shared, and what stays local.', { size: 26, color: C.sub, w: 'fill', name: 'Note' });
await wait(200); await SETTLE(s); await ROT();
const kids = s.children.filter(c => c.name !== 'Footer URL' && c.name !== 'Page Number');
const last = kids[kids.length - 1];
return { id: s.id, lastBottom: Math.round(last.y + last.height - s.y), footerTop: 1270 };
