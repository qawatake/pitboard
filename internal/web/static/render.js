// Drawing: the page's DOM from (page, uiState) only. No fetch, no timer,
// no listener, no global: the live page (main.js) and the gallery
// (gallery.js) both draw with it. memo keeps what was drawn (rows by key,
// the editor's text area) so that drawing again changes only what
// differs; a fresh memo draws the same thing from nothing. env is where
// things are (env.imageURL(n)).
//
// Each part of the report (parts/*.js) and each layer of the page
// (layers/*.js) draws itself and lists its own variants next to its code
// (docs/spec/web-states.md).
//
// fit() is the one step that needs the page laid out: it leaves keys out
// of a bar with no room, puts the right-end words of a narrow list on
// lines of their own, and says which open notices are too tall to pin.

import { el } from './dom.js';
import { patch, rightOf } from './parts/row.js';
import { isOpen, words } from './state.js';
import * as notice from './parts/notice.js';
import * as section from './parts/section.js';
import * as item from './parts/item.js';
import * as ask from './parts/ask.js';
import * as link from './parts/link.js';
import * as fold from './parts/fold.js';
import * as text from './parts/text.js';
import * as code from './parts/code.js';
import * as comment from './parts/comment.js';
import * as editor from './layers/editor.js';
import * as help from './layers/help.js';
import * as comments from './layers/comments.js';
import * as goto from './layers/goto.js';
import * as banners from './layers/banners.js';
import * as bar from './layers/bar.js';
import * as image from './layers/image.js';

// The modules, for the gallery: the parts of the report, the layers of
// the page.
export const PARTS = { notice, section, item, ask, link, fold, text, code, comment };
export const LAYERS = { bar, goto, editor, help, comments, image, banners };

export function newMemo() {
  return { cache: new Map(), rows: new Map(), editor: null, curRow: null };
}

// createApp makes the page's frame (empty): the scrolling page (the
// connection banner, the pinned band, the report), the bottom (message,
// keys), and what floats over them (comments, keys, an image).
export function createApp() {
  const app = el('div', 'app');
  const scroller = el('div', 'scroller');
  const skip = el('a', 'skip', '報告へ');
  skip.href = '#board';
  const banner = el('div', 'banner');
  banner.setAttribute('role', 'alert');
  banner.hidden = true;
  const top = el('header', 'top');
  const notices = el('div', 'notices');
  notices.setAttribute('aria-label', 'お知らせ');
  const summary = el('div', 'summary');
  summary.setAttribute('aria-label', '状況');
  top.append(notices, summary);
  const board = el('main', 'board');
  board.id = 'board';
  board.tabIndex = -1;
  board.setAttribute('aria-label', '報告');
  scroller.append(skip, banner, top, board);

  const foot = el('footer', 'foot');
  const status = el('div', 'status');
  status.setAttribute('role', 'status');
  status.setAttribute('aria-live', 'polite');
  const nav = el('nav', 'bar');
  nav.setAttribute('aria-label', 'キー');
  const hints = el('span', 'hints');
  const cb = el('button', 'b-comments');
  cb.type = 'button';
  cb.setAttribute('aria-haspopup', 'dialog');
  cb.setAttribute('aria-expanded', 'false');
  nav.append(hints, cb);
  foot.append(status, nav);

  const backdrop = el('div', 'backdrop');
  const panel = el('div', 'cpanel');
  panel.tabIndex = -1;
  panel.setAttribute('role', 'dialog');
  panel.setAttribute('aria-modal', 'true');
  panel.setAttribute('aria-label', 'コメント');
  panel.hidden = true;
  panel.append(el('h2', '', 'コメント'), el('div', 'clist'));

  const hl = el('div', 'help overlay');
  hl.setAttribute('role', 'dialog');
  hl.setAttribute('aria-modal', 'true');
  hl.setAttribute('aria-label', 'キー');
  hl.hidden = true;
  const sheet = el('div', 'sheet');
  const h = el('h2', '', 'キー');
  h.tabIndex = -1;
  const close = el('button', 'help-close');
  close.type = 'button';
  close.dataset.act = 'closeHelp';
  close.append(el('kbd', '', 'esc'), ' 閉じる');
  sheet.append(h, el('dl'), close);
  hl.append(sheet);

  const viewer = el('div', 'viewer overlay');
  viewer.setAttribute('role', 'dialog');
  viewer.setAttribute('aria-modal', 'true');
  viewer.setAttribute('aria-label', '画像');
  viewer.hidden = true;
  const vc = el('button', 'viewer-close');
  vc.type = 'button';
  vc.dataset.act = 'closeImage';
  vc.append(el('kbd', '', 'esc'), ' 閉じる');
  viewer.append(el('img'), vc);

  app.append(scroller, foot, backdrop, panel, hl, viewer);
  return app;
}

const q = (root, sel) => root.querySelector(sel);

// context is what every part draws with.
function context(page, ui, memo, env) {
  const cx = {
    page, ui, memo, env, rows: new Map(),
    words: (k) => words(page, k),
    isOpen: (t) => isOpen(ui, t),
    collect: null,
  };
  cx.things = (ts) => things(cx, ts, false);
  cx.line = (t, box) => line(cx, t, box);
  return cx;
}

// line makes box, the drawing of line of text t (page.Thing.Line: the
// title, a heading, a paragraph, code, a list line, a line of an opened
// body), a line one can point at: the cursor's when it is ui.cur, what a
// click or Tab goes to, with what the person wrote on it at its right end.
// A line nobody points at (in a notice's body) stays as it is.
function line(cx, t, box) {
  if (!t.Line) return box;
  box.classList.add('line');
  box.dataset.nav = t.Key;
  box.tabIndex = -1;
  const r = t.Right || {};
  if (r.Input && r.Input.Text && !box.querySelector(':scope > .right')) box.appendChild(rightOf(t, false));
  cx.rows.set(t.Key, box);
  if (cx.collect) cx.collect.push([t, box]);
  return box;
}

// things draws lines ts (top: the report's own, where a Fold is a heading).
function things(cx, ts, top) {
  const out = [];
  for (const t of ts || []) {
    switch (t.Kind) {
      case 'title': out.push(text.title(cx, t)); break;
      case 'section': section.draw(cx, t, out); break;
      default:
        if (t.Stop) out.push(rowOf(cx, t, { heading: top && t.Kind === 'fold' }));
        else out.push(text.block(cx, t));
    }
  }
  return out;
}

function rowOf(cx, t, opts) {
  switch (t.Kind) {
    case 'notice': return notice.draw(cx, t, opts);
    case 'ask': return ask.draw(cx, t, opts);
    case 'link': return link.draw(cx, t, opts);
    case 'fold': return fold.draw(cx, t, opts);
    case 'list': return text.listRow(cx, t, opts);
    default: return item.draw(cx, t, opts);
  }
}

// cursor marks the cursor's row (and only it): the class and aria-current.
function cursor(cx) {
  const { ui, memo } = cx;
  const now = ui.panel && !ui.cur.startsWith('#') ? null : memo.rows.get(ui.cur) || null;
  const prev = memo.curRow;
  // A row's button carries aria-current; a line of text, itself.
  const nav = (r) => r.querySelector(':scope > .rowbtn') || (r.dataset.nav ? r : null);
  if (prev && prev !== now) {
    prev.classList.remove('cur');
    const b = nav(prev);
    if (b) b.removeAttribute('aria-current');
  }
  if (now) {
    now.classList.add('cur');
    const b = nav(now);
    if (b) b.setAttribute('aria-current', 'true');
  }
  memo.curRow = now;
}

// render draws (page, ui) into app (made by createApp). only: 'cursor'
// when nothing but the cursor (and the bar, which follows it) changed:
// the same result, without walking the report.
export function render(app, page, ui, memo, env, only) {
  const cx = context(page, ui, memo, env);
  app.dataset.mode = ui.mode;
  app.classList.toggle('offline', ui.conn !== 'live');
  app.classList.toggle('panel-open', !!ui.panel);
  app.classList.toggle('locked', ui.mode === 'help' || ui.mode === 'image' || !!ui.panel);
  if (only === 'cursor') {
    cx.rows = memo.rows;
    cursor(cx);
    bar.draw(cx, q(app, '.foot'));
    return;
  }
  banners.connection(cx, q(app, '.banner'));

  const sum = q(app, '.summary');
  sum.replaceChildren();
  for (const w of page.Summary || []) sum.appendChild(el('span', 'word ' + w.Tone, w.Text));
  if (page.Hidden > 0) sum.appendChild(el('span', 'quiet word', `隠した行 ${page.Hidden}` + (ui.showHidden ? ' (表示中)' : '')));

  // Notices are pinned in the band at the top, but one opened too tall to
  // pin heads the page instead (as the terminal's).
  const pinned = [];
  const head = banners.head(cx);
  for (const t of page.Notices || []) (ui.unpinned[t.Key] ? head : pinned).push(rowOf(cx, t, {}));
  patch(q(app, '.notices'), pinned);
  patch(q(app, '.board'), [...head, ...things(cx, page.Things, true)]);
  // Rows of lines no longer there are forgotten.
  for (const k of memo.cache.keys()) if (!cx.rows.has(k)) memo.cache.delete(k);

  comments.draw(cx, q(app, '.cpanel'));
  memo.rows = cx.rows;
  cursor(cx);
  editor.place(cx, app);
  help.draw(cx, q(app, '.help'));
  image.draw(cx, q(app, '.viewer'));
  bar.draw(cx, q(app, '.foot'));
}

// renderPart draws lines ts of page alone (a part's variant), with the
// page's frame around them, through the very path render takes: the same
// things() and the same mount point (.board, or the band for a notice),
// a line of the report's own top level drawn as at the top (a Fold there
// is a heading). Nothing is told how to look.
export function renderPart(app, page, ui, memo, env, ts) {
  const cx = context(page, ui, memo, env);
  app.classList.add('part');
  const top = ts.every((t) => (page.Things || []).includes(t));
  const notices = ts.filter((t) => (page.Notices || []).includes(t));
  patch(q(app, '.notices'), notices.map((t) => rowOf(cx, t, {})));
  const rest = ts.filter((t) => !notices.includes(t));
  patch(q(app, '.board'), top ? things(cx, rest, true) : things(cx, rest, false));
  memo.rows = cx.rows;
  cursor(cx);
}

// lines is how many lines box e takes (its line height from CSS).
function lines(e) {
  const lh = parseFloat(getComputedStyle(e).lineHeight) || 20;
  return Math.round(e.getBoundingClientRect().height / lh);
}

// lists lays out lists and head words as page says (lists.go): a head
// word run-in when "head: body" fits on one line, else a definition; a
// list (the list lines of one parent, up to a blank line) loose, a gap
// between all of its lines, when any of them takes more than one line.
function lists(app) {
  for (const e of app.querySelectorAll('.hasHead')) {
    e.classList.remove('def');
    if (e.getClientRects().length && lines(e) > 1) e.classList.add('def');
  }
  for (const parent of new Set([...app.querySelectorAll('.li')].map((x) => x.parentElement))) {
    let group = [];
    const done = () => {
      const loose = group.some((li) => { const t = li.querySelector(':scope > .list > .lt'); return t && lines(t) > 1; });
      group.forEach((li, i) => li.classList.toggle('loose', loose && i > 0));
      group = [];
    };
    for (const c of parent.children) {
      if (!c.classList.contains('li') || c.querySelector(':scope > .list.gap')) done();
      if (c.classList.contains('li')) group.push(c);
    }
    done();
  }
}

// pad gives the page room at its end for what covers it at the bottom:
// the keys (--foot-h) and, while one is open, a floating layer (the m
// editor, the comment panel: --float-h, its height), so every line can be
// scrolled to above them. It returns the floating layers (the live page
// watches them grow).
export function pad(app) {
  app.style.setProperty('--foot-h', app.querySelector('.foot').offsetHeight + 'px');
  const fs = [...app.querySelectorAll('.editor.loose, .cpanel:not([hidden])')];
  const fh = Math.max(0, ...fs.map((f) => f.offsetHeight));
  app.style.setProperty('--float-h', fh + 'px');
  return fs;
}

// fit adjusts what needs the page laid out, and says which open notices
// are too tall to pin (taller than 30% of the screen): a new unpinned set
// when it changed, else null.
export function fit(app, page, ui) {
  bar.fit(q(app, '.foot'));
  // Narrow: a list whose right-end words do not fit beside the title (in
  // 40% of the width) has them on lines of their own, every row alike.
  for (const list of app.querySelectorAll('.notices, .board, .kids')) {
    const rows = [...list.children].filter((x) => x.classList && x.classList.contains('row'));
    let below = false;
    if (app.clientWidth < 600) {
      for (const r of rows) {
        const right = r.querySelector(':scope > .rowbtn > .right');
        const b = r.querySelector(':scope > .rowbtn');
        if (right && b && right.scrollWidth > b.clientWidth * 0.4) { below = true; break; }
      }
    }
    list.classList.toggle('rights-below', below);
  }
  lists(app);
  const h = app.clientHeight;
  let changed = false;
  const unpinned = { ...ui.unpinned };
  for (const r of q(app, '.notices').children) {
    const key = r.dataset && r.dataset.key;
    if (key && r.classList.contains('open') && r.offsetHeight > h * 0.3) { unpinned[key] = true; changed = true; }
  }
  for (const k of Object.keys(unpinned)) {
    const t = (page.Notices || []).find((x) => x.Key === k);
    if (!t || !isOpen(ui, t)) { delete unpinned[k]; changed = true; }
  }
  return changed ? unpinned : null;
}

export { comment, code };
