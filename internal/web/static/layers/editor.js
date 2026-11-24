// The editor: where a comment is written, right under what it is about
// (the line, its link's target, or the comment being edited), starting
// where the comment will sit. A comment on no line (m) floats where the
// comment panel does, headed 宛先なしのコメント: it is about no line, so
// it is never under one. The text area grows with what is
// written, up to 40% of the screen, then scrolls. Its keys are in the bar
// (only the editor's). The text area is the same element from one drawing
// to the next, so what is being typed (and the input method) is never
// disturbed; its value is set only when the state says otherwise.

import { el } from '../dom.js';
import { thumbs } from '../parts/comment.js';

// box is the editor's element for the drawing (made once per memo).
export function box(cx) {
  const m = cx.memo;
  if (!m.editor) {
    const b = el('div', 'editor');
    const lab = el('label');
    lab.htmlFor = 'editor-text';
    const area = el('textarea');
    area.id = 'editor-text';
    // No note under it: the bar shows the keys, and ? says the rest.
    // Under it, the images the text refers to, at a size to read.
    b.append(lab, area, el('div', 'thumbs'));
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

// loose says whether the editor writes a comment on no line.
export const loose = (ed) => !!ed && ed.kind === 'comment' && !ed.key;

// place puts the editor where it belongs (or takes it away): under its
// line, or floating over the page (app) for a comment on no line.
export function place(cx, app) {
  const ed = cx.ui.editor;
  if (!ed || cx.ui.mode !== 'editor') {
    if (cx.memo.editor) cx.memo.editor.remove();
    return;
  }
  const b = box(cx);
  const r = !loose(ed) && ed.at && cx.rows.get(ed.at);
  b.classList.toggle('loose', !r);
  if (r) (r.querySelector(':scope > .target') || r.querySelector(':scope > .rowbtn')).after(b);
  else if (b.parentNode !== app) app.querySelector('.foot').before(b);
}

const long = Array.from({ length: 8 }, (_, i) => `${i + 1} 行目`).join('\n');

export const covers = {"mode": {"editor": ["on a line"]}, "hint": {"place": ["on a line"], "stop": ["on a line"], "newline": ["on a line"]}};

export const differences = {"loose-editor": ["m: a comment on no line, floating", "m: long text grows upward"], "editor-empty": ["empty, on a line"], "edit-comment": ["editing a comment in the panel"], "content-above-float": ["m: long text and an image over a tall page, scrolled to its end"]};

export const variants = [
  { name: 'on a line', ui: { cur: 'hook', mode: 'editor', editor: { kind: 'comment', key: 'hook', at: 'hook', label: '「hook を入れるか決める」にコメント', text: '入れてよい' } }, responsive: true },
  { name: 'empty, on a line', ui: { cur: 'merge', mode: 'editor', editor: { kind: 'comment', key: 'merge', at: 'merge', label: '「WEB4 をこのまま merge してよいか」にコメント', text: '' } } },
  { name: '8 lines: grows, then scrolls', ui: { cur: 'hook', mode: 'editor', editor: { kind: 'comment', key: 'hook', at: 'hook', label: '「hook を入れるか決める」にコメント', text: long } } },
  { name: 'with an image', fixture: 'comments', ui: { cur: 'c1', mode: 'editor', editor: { kind: 'comment', key: 'c1', at: 'c1', label: '「未送信のコメントのある行」にコメント', text: 'これを見て [Image #1]' } } },
  { name: 'm: a comment on no line, floating', ui: { cur: 'hook', mode: 'editor', editor: { kind: 'comment', key: '', at: 'hook', label: '宛先なしのコメント', text: '' } }, responsive: true },
  { name: 'm: long text grows upward', ui: { cur: 'hook', mode: 'editor', editor: { kind: 'comment', key: '', at: 'hook', label: '宛先なしのコメント', text: long } }, responsive: true },
  { name: 'one large image', fixture: 'comments', ui: { cur: 'c1', mode: 'editor', editor: { kind: 'comment', key: 'c1', at: 'c1', label: '「未送信のコメントのある行」にコメント', text: 'この画面 [Image #2]' } }, responsive: true },
  { name: 'one tall image', fixture: 'comments', ui: { cur: 'c1', mode: 'editor', editor: { kind: 'comment', key: 'c1', at: 'c1', label: '「未送信のコメントのある行」にコメント', text: '縦長 [Image #3]' } }, responsive: true },
  { name: 'three images', fixture: 'comments', ui: { cur: 'c1', mode: 'editor', editor: { kind: 'comment', key: 'c1', at: 'c1', label: '「未送信のコメントのある行」にコメント', text: '前 [Image #1] 後 [Image #3] 全体 [Image #2]' } }, responsive: true },
  { name: 'editing a comment in the panel', fixture: 'comments', ui: { panel: true, cur: '#l:first', mode: 'editor', editor: { kind: 'edit', at: '#l:first', label: 'コメントを直す', text: '同じ行への 2 つ目' } } },
  { name: 'm: long text and an image over a tall page, scrolled to its end', ui: { cur: 'hook', mode: 'editor', editor: { kind: 'comment', key: '', at: 'hook', label: '宛先なしのコメント', text: long + '\n[Image #2]' } }, scroll: 'bottom', responsive: true },
];
