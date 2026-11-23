// Link: a title and where it goes, always shown under it on the title's
// text edge (a URL is a link; a path is text). It does not open.

import { el, isWeb } from '../dom.js';
import { row } from './row.js';

export function draw(cx, t, opts) {
  return row(cx, t, opts, (box) => {
    const tg = el('div', 'target');
    if (isWeb(t.URL)) {
      const a = el('a', '', t.URL);
      a.href = t.URL;
      a.rel = 'noopener noreferrer';
      a.target = '_blank';
      tg.appendChild(a);
    } else {
      tg.textContent = t.URL || t.Path;
    }
    box.appendChild(tg);
  });
}

export const covers = {"kind": {"link": ["path"]}};

export const variants = [
  { name: 'path', pick: 'spec' },
  { name: 'URL, reacted n, waiting', pick: 'pr' },
  { name: 'long URL', fixture: 'long', pick: 'longlink' },
  { name: 'hidden (H)', fixture: 'hidden', pick: 'h2' },
];
