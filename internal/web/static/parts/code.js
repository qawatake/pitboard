// Code: a fenced block. It keeps its lines as written and scrolls
// sideways inside its own box (the only part that may); a diff is colored.

import { el } from '../dom.js';

export function draw(cx, t) {
  const pre = el('pre', 'code');
  const code = el('code');
  for (const l of t.Code || []) {
    let cls = '';
    if (t.Lang === 'diff') cls = l.startsWith('+') ? 'move' : l.startsWith('-') ? 'no' : l.startsWith('@@') ? 'optkey' : '';
    code.appendChild(el('span', cls, l + '\n'));
  }
  pre.appendChild(code);
  // A box that scrolls must be reachable from the keyboard.
  pre.tabIndex = 0;
  return pre;
}

const bodyCode = (item, lang) => (p) => {
  const it = p.Things.flatMap(function all(t) { return [t, ...(t.Kids || []).flatMap(all)]; }).find((t) => t.Target === item);
  return it && (it.Body || []).find((b) => b.Kind === 'code' && (lang == null || b.Lang === lang));
};

export const covers = {"kind": ["code"]};

export const variants = [
  { name: 'plain', pick: (p) => p.Things.flatMap((t) => t.Kids || []).find((t) => t.Kind === 'code') },
  { name: 'diff', pick: bodyCode('olddiff', 'diff') },
  { name: 'longer than its box (scrolls)', fixture: 'long', pick: bodyCode('url') },
];
