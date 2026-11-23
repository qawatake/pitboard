// Code: a fenced block. It keeps its lines as written and never scrolls:
// a line longer than the box wraps anywhere and goes on under the line's
// own indent (after a diff's sign), as on the terminal. A diff is colored
// by page's meaning (DiffColor: + move, - bad, @@ quiet), all one weight.

import { el } from '../dom.js';

const TAB = 4; // the CSS tab-size

// hangOf is where a wrapped line goes on: past its indentation (tabs as
// drawn) and, in a diff, its sign. Counted in columns of the mono font.
export function hangOf(line, diff) {
  let col = 0;
  let i = 0;
  if (diff && /^[+\- ]/.test(line)) { col = 1; i = 1; }
  for (; i < line.length; i++) {
    if (line[i] === ' ') col++;
    else if (line[i] === '\t') col += TAB - (col % TAB);
    else break;
  }
  return i >= line.length ? 0 : col;
}

export function diffClass(l) {
  if (l.startsWith('@@')) return 'quiet';
  if (l.startsWith('+')) return 'move';
  if (l.startsWith('-')) return 'bad';
  return '';
}

export function draw(cx, t) {
  const pre = el('pre', 'code');
  const code = el('code');
  const diff = t.Lang === 'diff';
  for (const l of t.Code || []) {
    const line = el('span', 'cl' + (diff && diffClass(l) ? ' ' + diffClass(l) : ''), l);
    const h = Math.min(hangOf(l, diff), 24);
    if (h) line.style.setProperty('--hang', String(h));
    code.appendChild(line);
  }
  pre.appendChild(code);
  return pre;
}

const bodyCode = (item, lang) => (p) => {
  const it = p.Things.flatMap(function all(t) { return [t, ...(t.Kids || []).flatMap(all)]; }).find((t) => t.Target === item);
  return it && (it.Body || []).find((b) => b.Kind === 'code' && (lang == null || b.Lang === lang));
};

export const covers = {"kind": {"code": ["plain"]}};

export const variants = [
  { name: 'plain', pick: (p) => p.Things.flatMap((t) => t.Kids || []).find((t) => t.Kind === 'code') },
  { name: 'diff', pick: bodyCode('olddiff', 'diff') },
  { name: 'longer than its box: wraps under its indent', fixture: 'long', pick: bodyCode('url') },
];
