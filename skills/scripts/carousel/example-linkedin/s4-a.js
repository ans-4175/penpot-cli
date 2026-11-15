const pg = penpotUtils.getPageByName("Carousel"); await penpot.openPage(pg);
if (penpot.currentPage.id !== pg.id) throw new Error('page aktif salah');

const CARDS = [["user", "Solo designer", ["One file, one person", "Edits any style or component", "Easy to change, easy to drift."]], ["users", "Product team", ["A shared library with one owner", "Proposes changes, the owner approves", "Review keeps every screen consistent."]], ["building-2", "Whole company", ["Several libraries, one core team", "Brand rules locked, products extend them", "Versioned releases, nothing breaks silently."]]];
const ROWS = [['map-pin','Lives in'],['hand','Can change'],['shield','Safety']];
const CARD = (s, [icon, name, vals]) => {
  const c = BOX(s, { name: 'Card ' + name, w: 'fill', gap: 10, pad: [20, 26, 20, 26], fill: C.white, r: 22, stroke: C.hair, sw: 2 });
  const h = BOX(c, { name: 'Head', dir: 'row', gap: 14, ai: 'center' });
  ICON(h, icon, 36, C.accent); TXT(h, name, { wt: 800, size: 34, ls: -0.6, name: 'Name' });
  vals.forEach((v, i) => {
    const r = BOX(c, { name: 'Row ' + ROWS[i][1], dir: 'row', gap: 16, ai: 'center', pad: [10, 14, 10, 14], w: 'fill', fill: i === 2 ? C.peach : C.card, r: 14 });
    const lb = BOX(r, { name: 'Label', dir: 'row', gap: 10, ai: 'center', w: 178, h: 34 });
    ICON(lb, ROWS[i][0], 26, C.accent); TXT(lb, ROWS[i][1], { wt: 800, size: 22, name: 'Label text' });
    TXT(r, v, { size: 23, color: C.ink, lh: 1.25, w: 'fill', name: 'Value' });
  });
  return c;
};

if (penpot.root.children.some(c => c.name.startsWith('Slide 4 '))) throw new Error('Slide 4 sudah ada');
const s = SLIDE('Slide 4 - Who owns it', 3600, false, '4', '5');
CHIP(s, 'WHO OWNS IT');
const tb = BOX(s, { name: 'Title Block' });
TXT(tb, 'Three teams.', { wt: 800, size: 72, ls: -2.2, lh: 1.05, name: 'Title 1' });
const tr = BOX(tb, { name: 'Title Row', dir: 'row', ai: 'center', pad: [8, 0, 0, 0] });
MARK(tr, 'Three ways to run it.', 72, -1);
CARD(s, CARDS[0]);
await wait(200); await SETTLE(s); await ROT();
const kids = s.children.filter(c => c.name !== 'Footer URL' && c.name !== 'Page Number');
const last = kids[kids.length - 1];
return { id: s.id, bottom: Math.round(last.y + last.height - s.y) };
