// What the page says above the report instead of (or before) it: the
// connection (offline: what is shown may be old and nothing reaches the
// board; gone: the board was dropped), and the board's own messages (no
// session or report yet, an empty report, a report that could not be read,
// its format errors). The banner starts on the text edge, like the page.

import { el } from '../dom.js';

export function connection(cx, banner) {
  const on = cx.ui.conn !== 'live';
  banner.hidden = !on;
  banner.dataset.conn = cx.ui.conn;
  banner.textContent = on ? (cx.ui.connMsg || cx.words(cx.ui.conn === 'gone' ? 'gone' : 'offline')) : '';
}

// head is what goes before the report on the page.
export function head(cx) {
  const d = cx.page;
  const out = [];
  if (d.Message) out.push(el('p', 'msg', d.Message));
  if (d.Err) out.push(el('p', 'err', '読めませんでした: ' + d.Err));
  if (d.ReadErr) out.push(el('p', 'err', d.ReadErr));
  if (d.Problems && d.Problems.length) {
    const p = el('div', 'err problems');
    p.appendChild(el('strong', '', d.ProblemHead));
    for (const l of d.Problems) p.appendChild(el('div', '', l));
    out.push(p);
  }
  return out;
}

export const covers = {"conn": {"live": ["live"], "offline": ["offline: banner, page and bar dimmed"], "gone": ["gone: the board was dropped"]}};

export const variants = [
  { name: 'live', ui: { cur: 'hook' } },
  { name: 'offline: banner, page and bar dimmed', ui: { cur: 'hook', conn: 'offline' }, responsive: true },
  { name: 'gone: the board was dropped', ui: { cur: 'hook', conn: 'gone' } },
  { name: 'empty report', fixture: 'empty' },
  { name: 'no report yet', fixture: 'noreport' },
  { name: 'no session yet', fixture: 'nosession' },
  { name: 'format errors', fixture: 'problems' },
];
