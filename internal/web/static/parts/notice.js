// Notice: what the AI pinned above the page. A title in yellow, when it
// went up at the right end (page.AtWord: the time today, else the day
// too); opened, its body. Pinned in the band at the top unless opened too
// tall for it (then it heads the page).

import { row } from './row.js';
import { contents } from './item.js';

export function draw(cx, t, opts) {
  return row(cx, t, opts, (box, open) => {
    if (open) box.appendChild(contents(cx, t));
  });
}

export const covers = {"kind": ["notice"]};

export const variants = [
  { name: 'opens, closed (today)', pick: 'restart' },
  { name: 'open', pick: 'restart', open: true },
  { name: 'does not open (another day)', pick: 'ci' },
  { name: 'hidden (H): faint', fixture: 'hidden', pick: 'n1' },
  { name: 'long, open', fixture: 'long', pick: 'long', open: true },
];
