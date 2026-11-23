// The index (/): the boards this server shows, one row each (title,
// where, the same counts as each board's top right; a board whose
// directory is missing says so, faint). Drawn from (index JSON, cursor,
// offline) only: home.js (the live index) and the gallery both draw with
// it. Text is data: textContent only.

import { el } from '../dom.js';

// createHome makes the index's frame (empty).
export function createHome() {
  const app = el('div', 'app home');
  const scroller = el('div', 'scroller');
  const banner = el('div', 'banner');
  banner.setAttribute('role', 'alert');
  banner.hidden = true;
  const main = el('main', 'board');
  main.tabIndex = -1;
  main.setAttribute('aria-labelledby', 'home-title');
  const h = el('h1', '', 'ボード');
  h.id = 'home-title';
  main.append(h, el('div', 'boards'));
  scroller.append(banner, main);
  app.append(scroller);
  return app;
}

// draw draws index data with the cursor on board cur ('' the first).
export function draw(app, data, cur, offline) {
  const banner = app.querySelector('.banner');
  banner.hidden = !offline;
  banner.textContent = offline ? ((data && data.Words.offline) || 'つながっていません') : '';
  app.classList.toggle('offline', !!offline);
  const rows = [];
  if (!data.Boards.length) rows.push(el('p', 'msg', data.Words.none));
  for (const b of data.Boards) {
    const row = el('div', 'row boardrow' + (b.ID === cur ? ' cur' : '') + (b.Gone ? ' gone' : ''));
    const a = el('a', 'rowbtn');
    a.href = '/s/' + encodeURIComponent(b.ID) + '/';
    a.dataset.nav = b.ID;
    if (b.ID === cur) a.setAttribute('aria-current', 'true');
    a.appendChild(el('span', 'gut'));
    a.appendChild(el('span', 'dot'));
    a.appendChild(el('span', 'sign'));
    const t = el('span', 'title');
    t.appendChild(el('span', 'btitle', b.Title || b.Message || b.Session));
    t.appendChild(el('span', 'quiet bdir', b.Where || b.Dir));
    if (b.Gone && b.Title) t.appendChild(el('span', 'quiet bnote', b.Message));
    a.appendChild(t);
    const right = el('span', 'right');
    for (const w of b.Summary || []) right.appendChild(el('span', 'word ' + w.Tone, w.Text));
    a.appendChild(right);
    row.appendChild(a);
    rows.push(row);
  }
  app.querySelector('.boards').replaceChildren(...rows);
}

export const covers = {};

export const differences = {"index-none": ["no boards"], "index-several": ["several boards"], "index-gone": ["one board gone"]};

export const variants = [
  { name: 'no boards', index: 'none' },
  { name: 'several boards', index: 'several', responsive: true },
  { name: 'one board gone', index: 'gone' },
];
