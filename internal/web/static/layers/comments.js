// The comment panel: every comment floats over the page (a dialog at the
// bottom right; a sheet the width of the screen on a phone), over the
// band and the bar, as tall as what it holds up to 70% of the screen, then
// scrolling inside. The cursor is in it while it is open; its keys are
// page's (CommentHints), and enter goes to the comment's line.

import { el } from '../dom.js';
import { panelRow } from '../parts/comment.js';
import { index, panelOrder } from '../state.js';

export function draw(cx, panel) {
  panel.hidden = !cx.ui.panel;
  const list = panel.querySelector('.clist');
  if (!cx.ui.panel) { list.replaceChildren(); return; }
  const cs = panelOrder(cx.page);
  const out = [];
  if (!cs.length) out.push(el('p', 'msg', cx.words('noComments')));
  // Said once for all, not under each comment.
  if (cs.some((c) => c.Note === cx.words('notSent'))) out.push(el('p', 'cnote quiet', cx.words('notSent')));
  const byTarget = index(cx.page).byTarget;
  for (const c of cs) {
    const key = '#l:' + c.N;
    const r = panelRow(cx, c, key, !!(c.Target && byTarget.has(c.Target)));
    cx.rows.set(key, r);
    out.push(r);
  }
  list.replaceChildren(...out);
}

export const covers = {"mode": {"comments": ["every state; cursor on a comment"]}, "hint": {"edit": ["every state; cursor on a comment"], "retract": ["every state; cursor on a comment"]}};

export const differences = {"panel-scrolled": ["scrolled to the cursor (the last comment)"], "float-edge": ["every state; cursor on a comment", "no comments yet"], "image-edge": ["every state; cursor on a comment"]};

export const variants = [
  { name: 'every state; cursor on a comment', fixture: 'comments', ui: { panel: true, mode: 'comments', cur: '#l:first' }, responsive: true },
  { name: 'no comments yet', fixture: 'empty', ui: { panel: true, mode: 'comments' } },
  { name: 'scrolled to the cursor (the last comment)', fixture: 'comments', ui: { panel: true, mode: 'comments', cur: '#l:last' }, panelScroll: 'cursor', h: 520 },
];
