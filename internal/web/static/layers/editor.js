// The editor: where a comment is written, right under what it is about
// (the line, its link's target, or the comment being edited; a comment on
// no line at the end of the page). The text area grows with what is
// written, up to 40% of the screen, then scrolls. Its keys are in the bar
// (only the editor's). The text area is the same element from one drawing
// to the next, so what is being typed (and the input method) is never
// disturbed; its value is set only when the state says otherwise.

import { el } from '../dom.js';
import { thumbs } from '../parts/comment.js';

const NOTE = 's で送るまで AI には届かない · esc でやめても書いたものはこのタブに残る · 画像は貼り付け・ドロップ';

// box is the editor's element for the drawing (made once per memo).
export function box(cx) {
  const m = cx.memo;
  if (!m.editor) {
    const b = el('div', 'editor');
    const lab = el('label');
    lab.htmlFor = 'editor-text';
    const area = el('textarea');
    area.id = 'editor-text';
    area.setAttribute('aria-describedby', 'editor-note');
    const shots = el('div', 'thumbs');
    const note = el('div', 'hint', NOTE);
    note.id = 'editor-note';
    b.append(lab, area, shots, note);
    m.editor = b;
  }
  const ed = cx.ui.editor || {};
  const b = m.editor;
  b.querySelector('label').textContent = ed.label || '';
  const area = b.querySelector('textarea');
  const text = ed.text || '';
  if (area.value !== text) area.value = text;
  // Without field-sizing (CSS), it is as many lines high as the text has.
  area.rows = Math.max(3, Math.min(text.split('\n').length + 1, 12));
  const ns = [...new Set([...text.matchAll(/\[Image #([0-9]+)\]/g)].map((x) => Number(x[1])))];
  b.querySelector('.thumbs').replaceWith(thumbs(cx, ns));
  return b;
}

// place puts the editor where it belongs (or takes it away).
export function place(cx, board) {
  const ed = cx.ui.editor;
  if (!ed || cx.ui.mode !== 'editor') {
    if (cx.memo.editor) cx.memo.editor.remove();
    return;
  }
  const b = box(cx);
  const r = ed.at && cx.rows.get(ed.at);
  if (r) (r.querySelector(':scope > .target') || r.querySelector(':scope > .rowbtn')).after(b);
  else board.appendChild(b);
}

const long = Array.from({ length: 8 }, (_, i) => `${i + 1} 行目`).join('\n');

export const covers = {"mode": ["editor"], "hint": ["place", "stop", "newline"]};

export const variants = [
  { name: 'on a line', ui: { cur: 'hook', mode: 'editor', editor: { kind: 'comment', key: 'hook', at: 'hook', label: '「hook を入れるか決める」にコメント', text: '入れてよい' } }, responsive: true },
  { name: '8 lines: grows, then scrolls', ui: { cur: 'hook', mode: 'editor', editor: { kind: 'comment', key: 'hook', at: 'hook', label: '「hook を入れるか決める」にコメント', text: long } } },
  { name: 'with an image', fixture: 'comments', ui: { cur: 'c1', mode: 'editor', editor: { kind: 'comment', key: 'c1', at: 'c1', label: '「未送信のコメントのある行」にコメント', text: 'これを見て [Image #1]' } } },
  { name: 'a comment on no line', ui: { cur: 'hook', mode: 'editor', editor: { kind: 'comment', key: '', at: '', label: '宛先なしのコメント', text: '' } } },
  { name: 'editing in the panel', fixture: 'comments', ui: { panel: true, cur: '#l:1', mode: 'editor', editor: { kind: 'edit', n: 1, at: '#l:1', label: 'コメント 1 を直す', text: '読んでください' } } },
];
