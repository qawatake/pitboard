// Text: the page's title, paragraphs (and small headings) and list lines.
// A list line one can point at (it can be hidden) is a row; the lines in
// a body are not. Long words break anywhere: nothing is ever cut.

import { el, spans } from '../dom.js';
import { row } from './row.js';
import * as code from './code.js';

export function title(cx, t) {
  return spans(el('h1', 'ptitle'), t.Title);
}

// block draws a line nobody points at: a paragraph, a list line (with the
// lines under it), code.
export function block(cx, t) {
  switch (t.Kind) {
    case 'code': return code.draw(cx, t);
    case 'list': {
      const d = el('div', 'li');
      const l = el('div', 'list' + (t.Gap ? ' gap' : ''));
      l.appendChild(el('span', 'lm', t.Num || (t.Level === 1 ? '-' : '·')));
      l.appendChild(spans(el('span', 'lt'), t.Title));
      d.appendChild(l);
      if (t.Subs && t.Subs.length) {
        const s = el('div', 'subs');
        for (const x of t.Subs) s.appendChild(block(cx, x));
        d.appendChild(s);
      }
      return d;
    }
    default:
      return spans(el('div', 'para' + (t.Head ? ' head' : '') + (t.Gap ? ' gap' : '')), t.Title);
  }
}

// listRow is a list line one can point at: its own sign (number, - or ·),
// and the lines nested under it.
export function listRow(cx, t, opts) {
  return row(cx, t, opts, (box) => {
    if (t.Subs && t.Subs.length) {
      const s = el('div', 'subs body');
      for (const x of t.Subs) s.appendChild(block(cx, x));
      box.appendChild(s);
    }
  });
}

const firstOf = (kind) => (p) => p.Things.find((t) => t.Kind === kind);

export const covers = {"kind": ["title", "para", "list"], "style": ["plain", "bold", "code", "link"]};

export const variants = [
  { name: 'title', pick: firstOf('title') },
  { name: 'paragraph: bold, code, link', pick: firstOf('para') },
  { name: 'list line', pick: 'ただの行' },
  { name: 'list line, hidden', fixture: 'hidden', pick: '隠したただの行' },
  { name: 'body: lists, numbers, a small heading', pick: (p) => {
    const web = p.Things.flatMap((t) => t.Kids || []).find((t) => t.Target === 'web');
    return web && { Kind: 'section', Key: 'body', Title: [{ Text: '(中身だけ)', Style: 'plain' }], Kids: web.Body };
  } },
  { name: 'long title', fixture: 'long', pick: firstOf('title') },
  { name: 'long list line', fixture: 'long', pick: (p) => p.Things.flatMap((t) => t.Kids || []).find((t) => t.Kind === 'list') },
];
