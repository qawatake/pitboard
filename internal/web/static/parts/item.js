// Item: a line of work. Closed, its title and right end; opened (when it
// has something to open), its note, body and the comments on it, one step
// in from the title's text edge.

import { el, spans } from '../dom.js';
import { row } from './row.js';
import { block } from './text.js';
import * as comment from './comment.js';

// contents is what an opened Item (or notice, or Ask) shows under its
// title: the note (why its status), the body, the comments on it.
export function contents(cx, t) {
  const box = el('div', 'body');
  if (t.Note) {
    const p = el('div', 'para');
    p.appendChild(el('span', 'faint', '理由: '));
    spans(p, [{ Text: t.Note, Style: 'plain' }]);
    box.appendChild(p);
  }
  for (const b of t.Body || []) box.appendChild(block(cx, b));
  box.appendChild(comment.on(cx, t));
  return box;
}

export function draw(cx, t, opts) {
  return row(cx, t, opts, (box, open) => {
    if (open) box.appendChild(contents(cx, t));
  });
}

export const covers = {"kind": ["item"], "mark": ["none", "new", "waiting"], "state": ["todo", "doing", "done"], "tone": ["quiet", "need", "move"]};

export const variants = [
  { name: 'openable, closed; waiting ●; 未送信', pick: 'hook' },
  { name: 'open', pick: 'hook', open: true },
  { name: 'nothing to open; doing (作業中)', pick: 'impl' },
  { name: 'doing with a status, open: lists, heading', pick: 'web', open: true },
  { name: 'todo with a note, open', pick: 'next1', open: true },
  { name: 'done', pick: 'old' },
  { name: 'done, open: diff', pick: 'olddiff', open: true },
  { name: 'unread ● (a reply: title starts "> ")', pick: 'reply' },
  { name: 'waits on the person (yellow status)', pick: 'parts' },
  { name: 'reacted y', pick: 'merge' },
  { name: '既読 (quiet)', fixture: 'comments', pick: 'c3' },
  { name: 'y sent, taken back: 取消 未送信', fixture: 'comments', pick: 'u1' },
  { name: '2 comments (count)', fixture: 'comments', pick: 'c1' },
  { name: 'hidden (H)', fixture: 'hidden', pick: 'h1' },
  { name: 'long title, long right word', fixture: 'long', pick: 'url' },
  { name: 'long, open: URL, letters, list, code', fixture: 'long', pick: 'url', open: true },
];
