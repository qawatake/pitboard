// Fold: lines folded under a title. At the top level it is a division of
// the page, drawn as a Section heading (with the same gap above it);
// inside, a line like the others. Opened: the comments on it, then what
// it holds, one step in.

import { el } from '../dom.js';
import { row } from './row.js';
import * as comment from './comment.js';

export function draw(cx, t, opts) {
  return row(cx, t, opts, (box, open) => {
    if (!open) return null;
    box.appendChild(comment.on(cx, t, 'body'));
    const kids = el('div', 'kids');
    box.appendChild(kids);
    return kids;
  });
}

const top = (p) => p.Things.find((t) => t.Kind === 'fold');

export const covers = {"kind": ["fold"]};

export const variants = [
  { name: 'top level, closed (a heading)', pick: top, heading: true },
  { name: 'top level, open: nested Fold open', pick: top, heading: true, open: true, openAlso: ['中の折りたたみ'] },
  { name: 'inside, closed', pick: '中の折りたたみ' },
  { name: 'with a comment, open', fixture: 'comments', pick: 'f1', open: true },
  { name: 'hidden (H)', fixture: 'hidden', pick: 'h4', heading: true },
];
