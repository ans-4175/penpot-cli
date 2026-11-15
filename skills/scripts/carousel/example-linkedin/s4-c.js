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
const s = penpot.root.children.find(c => c.name === 'Slide 4 - Who owns it');
if (!s) throw new Error('slide 4 tidak ada');
CARD(s, CARDS[2]);
const tk = BOX(s, { name: 'Takeaway', dir: 'row', gap: 14, ai: 'center', w: 'fill', pad: [8, 0, 0, 0] });
ICON(tk, 'arrow-right', 32, C.accent);
TXT(tk, 'The more people use it, the more a change costs. Set the rules first.', { wt: 700, size: 26, w: 'fill', lh: 1.25, name: 'Takeaway text' });
await wait(200); await SETTLE(s);
const kids = s.children.filter(c => c.name !== 'Footer URL' && c.name !== 'Page Number');
const last = kids[kids.length - 1];
return { bottom: Math.round(last.y + last.height - s.y), kids: kids.length };
