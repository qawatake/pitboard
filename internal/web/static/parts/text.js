// Text: the page's title, paragraphs (and small headings) and list lines.
// A "- " list line of the report (it can be hidden) is a row; the others,
// and every line of text, are lines one points at too (cx.line: only a
// comment acts on them). Long words break anywhere: nothing is ever cut.

import { el, spans } from '../dom.js';
import { row } from './row.js';
import * as code from './code.js';

export function title(cx, t) {
  return cx.line(t, spans(el('h1', 'ptitle'), t.Title));
}

// headed is text t (a list line's or a paragraph's) into box: run-in
// "head: body" when it has a head word (page.HeadWord); render.fit turns
// it into a definition (the head alone, the body one step in under it)
// when it does not fit on one line.
function headed(box, t) {
  if (!t.HeadWord) return spans(box, t.Title);
  box.classList.add('hasHead');
  box.append(el('strong', 'hw', t.HeadWord), el('span', 'hsep', ': '));
  box.appendChild(spans(el('span', 'hb'), t.Title));
  return box;
}

// block draws a line of text: a paragraph, a list line (with the lines
// under it), code.
export function block(cx, t) {
  switch (t.Kind) {
    case 'code': return cx.line(t, code.draw(cx, t));
    case 'list': {
      const d = el('div', 'li');
      const l = el('div', 'list' + (t.Gap ? ' gap' : ''));
      l.appendChild(el('span', 'lm', t.Num || (t.Level === 1 ? '-' : '·')));
      l.appendChild(headed(el('span', 'lt'), t));
      d.appendChild(cx.line(t, l));
      if (t.Subs && t.Subs.length) {
        const s = el('div', 'subs');
        for (const x of t.Subs) s.appendChild(block(cx, x));
        d.appendChild(s);
      }
      return d;
    }
    default:
      return cx.line(t, headed(el('div', 'para' + (t.Head ? ' head' : '') + (t.Gap ? ' gap' : '')), t));
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

export const covers = {"kind": {"title": ["title"], "para": ["paragraph: bold, code, link"], "list": ["list line"]}, "style": {"plain": ["paragraph: bold, code, link"], "bold": ["paragraph: bold, code, link"], "code": ["paragraph: bold, code, link"], "link": ["paragraph: bold, code, link"]}};

export const differences = {"list-tight": ["a list whose lines fit: tight"], "list-loose": ["a list with a line that wraps: loose"], "head-runin": ["head word that fits: run-in"], "head-def": ["head word that does not fit: a definition"]};

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
  { name: 'a list whose lines fit: tight', fixture: 'lists', pick: 'tight', open: true },
  { name: 'a list with a line that wraps: loose', fixture: 'lists', pick: 'loose', open: true, responsive: true },
  { name: 'head word that fits: run-in', fixture: 'lists', pick: 'runin', open: true },
  { name: 'head word that does not fit: a definition', fixture: 'lists', pick: 'def', open: true, responsive: true },
];
