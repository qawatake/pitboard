// g: the page waits for a letter. Each place it can go (notices,
// Sections, top-level Folds) shows its letter in the gutter, where the
// cursor goes (nothing moves), and the bar lists the places, as the
// terminal's does.

import { el, keyLabel, plain } from '../dom.js';

// items is the bar while g waits: esc, then each place's letter and title.
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
  return out;
}

export const covers = {"mode": {"goto": ["g: letters in the gutter, the places in the bar"]}};

export const variants = [
  { name: 'g: letters in the gutter, the places in the bar', ui: { mode: 'goto' }, responsive: true },
];
