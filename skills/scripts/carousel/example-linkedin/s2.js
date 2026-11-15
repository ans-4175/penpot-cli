const pg = penpotUtils.getPageByName("Carousel"); await penpot.openPage(pg);
if (penpot.currentPage.id !== pg.id) throw new Error('page aktif salah');
if (penpot.root.children.some(c => c.name.startsWith('Slide 2 '))) throw new Error('Slide 2 sudah ada');
const s = SLIDE('Slide 2 - System', 1200, false, '2', '5');
CHIP(s, 'THE SYSTEM');
const tb = BOX(s, { name: 'Title Block' });
TXT(tb, 'A system is', { wt: 800, size: 80, ls: -2.4, lh: 1.05, name: 'Title 1' });
const tr = BOX(tb, { name: 'Title Row', dir: 'row', ai: 'center', pad: [10, 0, 0, 0] });
MARK(tr, 'four parts.', 80, -1);
const list = BOX(s, { name: 'Parts', w: 'fill' });
const parts = [
  ['palette', 'Tokens', 'Colors, type and spacing, named once and reused everywhere.'],
  ['component', 'Components', 'Buttons, cards and forms built once, used on every screen.'],
  ['layout-template', 'Patterns', 'Proven layouts for flows people repeat: sign-up, checkout, settings.'],
  ['book-open', 'Docs', 'Rules and examples, so the next designer doesn\u2019t have to guess.'],
];
parts.forEach(([ic, name, desc], i) => {
  const row = BOX(list, { name: name, dir: 'row', gap: 24, ai: 'start', pad: [14, 0, 14, 0], w: 'fill' });
  const ib = BOX(row, { name: 'Icon Box', w: 84, h: 84, fill: i === 0 ? C.ink : C.peach, r: 20, ai: 'center', jc: 'center' });
  ICON(ib, ic, 44, i === 0 ? C.marker : C.accent);
  const body = BOX(row, { name: 'Body', w: 'fill', gap: 6 });
  TXT(body, name, { wt: 800, size: 44, ls: -1, w: 'fill', name: 'Name' });
  TXT(body, desc, { size: 28, color: C.sub, lh: 1.35, w: 'fill', name: 'Desc' });
  if (i < parts.length - 1) BOX(list, { name: 'Divider', w: 'fill', h: 2, fill: C.hair });
});
const same = BOX(s, { name: 'Same Parts In', gap: 12, pad: [6, 0, 0, 0] });
TXT(same, 'SAME PARTS IN', { fam: J, wt: 700, size: 20, color: C.foot, ls: 2, name: 'Label' });
const cr = BOX(same, { name: 'Chips', dir: 'row', gap: 12, ai: 'center' });
[['monitor', 'Web'], ['smartphone', 'iOS'], ['smartphone', 'Android'], ['mail', 'Email']].forEach(([k, n]) => {
  const c = BOX(cr, { name: 'Chip ' + n, dir: 'row', gap: 8, pad: [10, 14, 10, 14], ai: 'center', fill: C.white, r: 14, stroke: C.hair, sw: 2 });
  ICON(c, k, 24, C.accent); TXT(c, n, { wt: 800, size: 22, name: 'Name' });
});
await wait(200); await SETTLE(s); await ROT();
const kids = s.children.filter(c => c.name !== 'Footer URL' && c.name !== 'Page Number');
const last = kids[kids.length - 1];
return { id: s.id, lastBottom: Math.round(last.y + last.height - s.y), footerTop: 1270 };
