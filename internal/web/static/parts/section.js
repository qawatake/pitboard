// Section: a heading and what it holds. One that holds nothing says なし,
// faint, under its heading. The heading has g's letter hanging in the
// gutter while g waits.

import { el, spans } from '../dom.js';

export function draw(cx, t, out) {
  const h = el('h2', 'section');
  if (t.Jump) h.dataset.jump = String(t.Jump);
  if (cx.ui.mode === 'goto' && t.JumpKey) h.appendChild(el('span', 'jumpkey hang', t.JumpKey));
  spans(h, t.Title);
  out.push(h);
  if (t.None) out.push(el('p', 'none quiet', t.None));
  out.push(...cx.things(t.Kids));
}

export const covers = {"kind": {"section": ["with lines"]}};

export const variants = [
  { name: 'with lines', pick: 'あなたへ' },
  { name: 'empty (なし), then a top-level Fold', pick: 'あとで', after: (p) => p.Things.find((t) => t.Kind === 'fold') },
  { name: 'long heading', fixture: 'long', pick: (p) => p.Things.find((t) => t.Kind === 'section') },
];
