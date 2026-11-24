// g: the page waits for a letter. Each place it can go (notices,
// Sections, top-level Folds) shows its letter in the gutter, where the
// cursor goes (nothing moves), and the bar lists the places, as the
// terminal's does.

import { el, keyLabel, plain } from '../dom.js';
import { index } from '../state.js';

// items is the bar while g waits: esc, then each place's letter and
// title, then the cursor line's links (their digits and where they go),
// then - back when a jump can be gone back.
export function items(cx, key) {
  const out = [key(cx, { Do: 'endGoto', Key: 'esc', Word: 'やめる' })];
  const walk = (ts) => {
    for (const t of ts || []) {
      if (t.JumpKey) {
        const b = key(cx, { Do: 'jumpTo', Key: t.JumpKey, Word: '' });
        b.replaceChildren(keyLabel(t.JumpKey, ''), el('span', 'jtitle', plain(t.Title)));
        out.push(b);
      }
      walk(t.Kids);
    }
  };
  walk(cx.page.Notices);
  walk(cx.page.Things);
  const i = index(cx.page);
  const t = i.things.get(cx.ui.cur);
  for (const l of (t && t.Links) || []) {
    if (!l.Num) continue;
    const to = i.things.get(i.byTarget.get(l.Target));
    const b = key(cx, { Do: 'follow', Key: String(l.Num), Word: '' });
    b.replaceChildren(keyLabel(String(l.Num), ''), el('span', 'jtitle', to ? plain(to.Title) : l.Text));
    out.push(b);
  }
  if ((cx.ui.back || []).length) out.push(key(cx, { Do: 'back', Key: '-', Word: cx.words('back') }));
  return out;
}

export const covers = {"mode": {"goto": ["g: letters in the gutter, the places in the bar"]}, "hint": {"back": ["g on a line with links: their digits, and - back"]}};

export const differences = {"link-digits": ["g on a line with links: their digits, and - back"]};

export const variants = [
  { name: 'g: letters in the gutter, the places in the bar', ui: { mode: 'goto' }, responsive: true },
  { name: 'g on a line with links: their digits, and - back', ui: { mode: 'goto', cur: 'reply', open: { reply: true }, back: [{ cur: 'hook', y: 0 }] }, responsive: true },
];
