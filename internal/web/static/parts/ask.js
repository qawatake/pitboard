// Ask: a question and its options. Closed, the question (answered: the
// key and the head of the answer at the right end); opened, its options,
// each key in a column, the chosen one's ● hanging left of it, so the
// texts line up whether chosen or not.

import { el, spans } from '../dom.js';
import { row } from './row.js';
import { contents } from './item.js';

export function draw(cx, t, opts) {
  return row(cx, t, opts, (box, open) => {
    if (!open) return;
    box.appendChild(contents(cx, t));
    const list = el('div', 'options');
    for (const o of t.Kids || []) {
      const line = el('div', 'option' + (o.Chosen ? ' chosen' : ''));
      const dot = el('span', 'odot', o.Chosen ? '●' : '');
      if (o.Chosen) dot.appendChild(el('span', 'sr', '選んだ答え: '));
      line.appendChild(dot);
      line.appendChild(el('span', 'okey', o.OptionKey));
      line.appendChild(spans(el('span', 'otext'), o.Title));
      list.appendChild(line);
    }
    box.appendChild(list);
  });
}

export const covers = {"kind": ["ask", "option"]};

export const variants = [
  { name: 'answered, closed (key and head)', pick: 'unit' },
  { name: 'answered, open (● hangs)', pick: 'unit', open: true },
  { name: 'not answered, closed', pick: 'where' },
  { name: 'not answered, open; long option', pick: 'where', open: true },
  { name: 'long unbroken option', fixture: 'long', pick: 'lask', open: true },
  { name: 'hidden (H)', fixture: 'hidden', pick: 'h3' },
];
