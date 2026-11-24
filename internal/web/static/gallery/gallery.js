// The gallery (/_gallery): every part of the report and every layer of the
// page, in each of its variants, drawn by the page's own render.js (and
// layers/home.js for the index) from fixtures.json, made by the Go code
// (internal/web/gallery_test.go). A variant takes the very path the live
// page takes: render() for a layer or a page, renderPart() (the same
// things() and mount point) for a part, then fit().
//
// Each module owns its variants (export const variants) and declares, as
// static data, which variants show each value of the data model it covers
// (export const covers = {axis: {value: [variant names]}}) and each visual
// difference that is no enum value (export const differences = {name:
// [variant names]}). Go checks the bookkeeping (every value covered, every
// name a variant); this page checks that each named variant really SHOWS
// it, by looking at the drawn DOM (classes, attributes, layout), never at
// the variant's data or ui.
//
// Axes that are not a part's own are applied here to every variant: the
// width (1280 / 390), the theme (light / dark), and the cursor on the
// part. A variant may fix its own width (w) and height (h).
//
// When it is done, window.__gallery says what it drew and what went wrong
// (a story that threw, drew nothing, has something wider than its box, a
// declared value or difference its variant does not show):
// scripts/web-gallery.sh fails on any, and compares a few stories with the
// live page put in the same state.

import { createApp, newMemo, render, renderPart, fit, pad, PARTS, LAYERS } from '../render.js';
import { createHome, draw as drawHome } from '../layers/home.js';
import * as home from '../layers/home.js';
import * as pages from './pages.js';
import { findThing } from '../parts/row.js';
import * as S from '../state.js';
import { el } from '../dom.js';

const params = new URLSearchParams(location.search);
const W = Number(params.get('w')) || 1280;
const THEME = params.get('theme') === 'dark' ? 'dark' : 'light';
const CUR = params.get('cur') === '1';
const ONLY = params.get('only') || ''; // module, or module/NN
// Image 2 is large (2400 × 1600), 3 tall (600 × 1800), 4 white (a
// screenshot of a white page, 1200 × 760), any other small.
const env = { imageURL: (n) => (n === 2 ? '/gallery/large.svg' : n === 3 ? '/gallery/tall.svg' : n === 4 ? '/gallery/white.svg' : '/gallery/sample.svg') };

const fx = await (await fetch('/gallery/fixtures.json')).json();
const report = { ready: false, stories: [], errors: [] };
window.__gallery = report;

// keyOf is the key of the line named name (an id, a key, a title).
function keyOf(page, name) {
  if (!name) return '';
  if (name === '#l:first' || name === '#l:last') {
    const cs = S.panelOrder(page);
    const c = name === '#l:first' ? cs[0] : cs[cs.length - 1];
    return c ? '#l:' + c.N : '';
  }
  if (name.startsWith('#')) return name;
  const t = findThing(page, name);
  if (!t) throw new Error(`no line ${name}`);
  return t.Key;
}

// resolve is a variant's (page, ui) and, for a part, its lines.
function resolve(v, isPart) {
  const sit = fx.Situations[v.fixture || 'dense'];
  if (!sit) throw new Error(`no fixture ${v.fixture}`);
  // undo: as the tab that did the acts sees it (its bar has u).
  const page = v.undo ? { ...sit.Page, CanUndo: true, More: sit.UndoMore } : sit.Page;
  const ui = { ...S.initial(), ...(sit.ShowHidden ? { showHidden: true } : {}), ...(v.ui || {}), open: {}, unpinned: {} };
  for (const [k, on] of Object.entries((v.ui && v.ui.open) || {})) ui.open[keyOf(page, k)] = on;
  for (const n of v.open && v.open !== true ? v.open : []) ui.open[keyOf(page, n)] = true;
  for (const n of v.openAlso || []) ui.open[keyOf(page, n)] = true;
  if (v.refusal) Object.assign(ui, { status: fx.Refusals[v.refusal], statusTone: 'need' });
  if (v.sent) Object.assign(ui, { status: fx.Sent, statusTone: '' });
  let things = null;
  if (isPart) {
    const t = typeof v.pick === 'function' ? v.pick(page) : findThing(page, v.pick);
    if (!t) throw new Error(`nothing picked (${v.pick})`);
    things = [t];
    if (v.after) things.push(v.after(page));
    if (v.open === true) ui.open[t.Key] = true;
    // The lines that hold it open (no change to the part drawn alone): the
    // live page it is compared with shows it so.
    for (const k of S.index(page).parents.get(t.Key) || []) if (!(k in ui.open)) ui.open[k] = true;
    if (CUR && t.Stop) ui.cur = t.Key;
  } else {
    ui.cur = keyOf(page, ui.cur);
    if (ui.editor) ui.editor = { ...ui.editor, at: keyOf(page, ui.editor.at), key: keyOf(page, ui.editor.key) };
    if (ui.editor && ui.editor.kind === 'edit') ui.editor.n = Number(ui.editor.at.slice(3));
    if (ui.panel && !ui.mode) ui.mode = 'comments';
    const t = S.index(page).things.get(ui.cur);
    if (t) {
      ui.sel = t.Token;
      const o = sit.Offers && sit.Offers[t.Key];
      const offer = v.locked ? fx.Locked : v.placed ? (S.isOpen(ui, t) && sit.PlacedOpen ? sit.PlacedOpen[t.Key] : sit.Placed[t.Key]) : o && (S.isOpen(ui, t) ? o.Open : o.Closed);
      if (offer) ui.offer = { key: t.Key, sel: t.Token, offer };
    }
  }
  return { page, ui, things };
}

// ---- what a story shows, read from its DOM ----

const visible = (e) => !!e && !e.closest('[hidden]') && e.getClientRects().length > 0;
const all = (root, sel) => [...root.querySelectorAll(sel)].filter(visible);
const settleOf = { '対応中: ': 'doing', '保留: ': 'hold', '対応: ': 'done' };

// straddles: some line of the report is cut by the y of edge.
function straddles(app, y) {
  return all(app, '.board .rowbtn, .board .para, .board h2, .board .cl').some((e) => {
    const r = e.getBoundingClientRect();
    return r.top < y - 2 && r.bottom > y + 2;
  });
}

// floatsLow: a floating layer sits on the text column's edges, its bottom a
// little above the bar (a sheet, edge to edge, on a narrow screen).
function floatsLow(app, el) {
  if (!visible(el)) return false;
  if (app.clientWidth <= 480) return true;
  const b = app.querySelector('.board'), f = app.querySelector('.foot');
  if (!b || !f) return false;
  const br = b.getBoundingClientRect(), cs = getComputedStyle(b), r = el.getBoundingClientRect();
  const left = br.left + parseFloat(cs.paddingLeft), right = br.right - parseFloat(cs.paddingRight);
  const gap = f.getBoundingClientRect().top - r.bottom;
  return Math.abs(r.left - left) < 1.5 && Math.abs(r.right - right) < 1.5 && gap > 0 && gap < 24;
}

// rgba is a computed color as [r, g, b, a].
function rgba(c) {
  const m = c.match(/rgba?\(([^)]*)\)/);
  if (!m) return null;
  const p = m[1].split(/[\s,/]+/).filter(Boolean).map(Number);
  return [p[0], p[1], p[2], p.length > 3 ? p[3] : 1];
}
// contrast is the WCAG contrast of two opaque colors.
function contrast(x, y) {
  const lum = (c) => { const [r, g, b] = c.slice(0, 3).map((v) => { v /= 255; return v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4; }); return 0.2126 * r + 0.7152 * g + 0.0722 * b; };
  const [h, l] = [lum(x), lum(y)].sort((p, q) => q - p);
  return (h + 0.05) / (l + 0.05);
}
// edged: e has a visible edge: a border ≥ 1px (its top one: a sheet on a
// phone keeps only that) of a color that is not e's background, standing
// off the page (contrast ≥ 1.3 with the page's color).
function edged(app, e) {
  const cs = getComputedStyle(e);
  const bc = rgba(cs.borderTopColor), page = rgba(getComputedStyle(app).backgroundColor);
  return parseFloat(cs.borderTopWidth) >= 1 && cs.borderTopStyle !== 'none' && !!bc && bc[3] === 1 &&
    cs.borderTopColor !== cs.backgroundColor && contrast(bc, page) >= 1.3;
}

// textEdges is the left edges of the text that must start on the one
// text edge: the outermost rows' titles (the band's and the report's, the
// lines of a top-level Fold lifted onto it, the index's boards), and the
// headings, paragraphs and the bar's first key. {titles, others}.
function textEdges(a) {
  const left = (e) => e.getBoundingClientRect().left;
  const titles = all(a, '.notices > .row > .rowbtn > .title, .board > .row > .rowbtn > .title, .board > .row.heading > .kids > .row > .rowbtn > .title, .boards > .row > .rowbtn > .title').map(left);
  const others = all(a, '.board > h1, .board > h2.section, .board > .para').map(left);
  const key = all(a, '.foot .bar > .hints > *')[0];
  if (key) others.push(left(key));
  return { titles, others };
}

// banded: the pinned band has its edge: a hairline under it, of a color
// that stands off the page.
function banded(a) {
  const t = a.querySelector('.top'), cs = getComputedStyle(t);
  const bc = rgba(cs.borderBottomColor), page = rgba(getComputedStyle(a).backgroundColor);
  return parseFloat(cs.borderBottomWidth) >= 1 && cs.borderBottomStyle !== 'none' && !!bc && bc[3] === 1 && contrast(bc, page) >= 1.3;
}

// DIFFS says, for each visual difference a module may declare, whether a
// drawn frame shows it.
export const DIFFS = {
  'rights-below': (a) => all(a, '.rights-below > .row > .rowbtn > .right:not(:empty)').length > 0,
  'unpinned': (a) => all(a, '.board > .row.k-notice').length > 0,
  'under-band': (a) => all(a, '.notices > .row').length > 0 && banded(a) && straddles(a, a.querySelector('.top').getBoundingClientRect().bottom),
  // The band with the counts alone (no notice) over lines scrolled under
  // it: its edge (a hairline) all the same.
  'under-band-bare': (a) => all(a, '.notices > .row').length === 0 && banded(a) && straddles(a, a.querySelector('.top').getBoundingClientRect().bottom),
  // Row titles start on the text edge of headings, paragraphs and the
  // bar (within 1px), at every width: checked at 1280 and at 390.
  'text-edge': (a) => { const { titles, others } = textEdges(a); const xs = [...titles, ...others]; return titles.length > 0 && others.length > 0 && Math.max(...xs) - Math.min(...xs) <= 1; },
  'under-bar': (a) => straddles(a, a.querySelector('.foot').getBoundingClientRect().top),
  'at-bottom': (a) => { const s = a.querySelector('.scroller'); return s.scrollTop > 0 && s.scrollTop + s.clientHeight >= s.scrollHeight - 1; },
  'focus-bar': (a) => { const f = document.activeElement; return !!f && a.contains(f) && !!f.closest('.bar') && f.matches(':focus-visible') && getComputedStyle(f).outlineStyle !== 'none'; },
  // A floating layer open over a page scrolled to its end: the last line
  // of the page is above the layer, not under it.
  // g on a line with links: the digits show on that line only.
  'link-digits': (a) => all(a, '.row.nums .linknum').some(visible) && !all(a, '.linknum').some((n) => visible(n) && !n.closest('.row.nums')),
  'content-above-float': (a) => {
    const f = a.querySelector('.editor.loose'); const s = a.querySelector('.scroller');
    const rows = all(a, '.board .row');
    if (!f || !visible(f) || !rows.length || s.scrollTop <= 0 || s.scrollTop + s.clientHeight < s.scrollHeight - 1) return false;
    return rows[rows.length - 1].getBoundingClientRect().bottom <= f.getBoundingClientRect().top + 1;
  },
  'loose-editor': (a) => all(a, '.editor.loose textarea').length > 0 && floatsLow(a, a.querySelector('.editor.loose')),
  'editor-empty': (a) => all(a, '.editor textarea').some((t) => t.value === ''),
  'edit-comment': (a) => all(a, '.cpanel .crow > .editor textarea').length > 0,
  'panel-scrolled': (a) => { const p = a.querySelector('.cpanel'); const c = p.querySelector('.row.cur'); if (!visible(p) || !c || p.scrollTop <= 0 || !floatsLow(a, p)) return false; const pr = p.getBoundingClientRect(), cr = c.getBoundingClientRect(); return cr.top >= pr.top - 1 && cr.bottom <= pr.bottom + 1; },
  'help-scrolled': (a) => { const s = a.querySelector('.help .sheet'); return visible(s) && s.scrollTop > 0 && s.scrollTop + s.clientHeight >= s.scrollHeight - 1; },
  // What floats (the m editor, the comment panel, ?, an image shown
  // whole): every one open has a visible edge.
  'float-edge': (a) => { const fs = ['.editor.loose', '.cpanel', '.help .sheet', '.viewer img'].map((s) => a.querySelector(s)).filter(visible); return fs.length > 0 && fs.every((f) => edged(a, f)); },
  // Images under a text: each with an edge, rounded, 12px or more from
  // the next on its line; in the editor, each with its ×.
  'image-edge': (a) => {
    const ts = all(a, '.thumbs .thumb');
    if (!ts.length) return false;
    const gapOK = ts.every((t, i) => { const n = ts[i + 1]; if (!n || n.parentNode !== t.parentNode) return true; const r = t.getBoundingClientRect(), q = n.getBoundingClientRect(); return Math.abs(r.top - q.top) > 1 || q.left - r.right >= 11.5; });
    return gapOK && ts.every((t) => { const i = t.querySelector('img'); return edged(a, i) && parseFloat(getComputedStyle(i).borderTopLeftRadius) >= 8 && (!t.closest('.editor') || visible(t.querySelector('.unimage'))); });
  },
  'image-large': (a) => { const i = a.querySelector('.viewer img'); return visible(i) && i.naturalWidth > i.clientWidth + 1; },
  'message': (a) => all(a, '.status:not(.need)').some((s) => s.textContent !== ''),
  'sent': (a) => all(a, '.status').some((s) => s.textContent.startsWith(fx.Sent)),
  'index-none': (a) => all(a, '.boards > .msg').length > 0,
  'index-several': (a) => all(a, '.boards > .boardrow').length >= 3,
  'index-gone': (a) => all(a, '.boardrow.gone').length > 0,
  'list-tight': (a) => all(a, '.li').length >= 2 && all(a, '.li.loose').length === 0,
  'list-loose': (a) => all(a, '.li.loose').length > 0,
  'head-runin': (a) => all(a, '.hasHead:not(.def) > .hw').length > 0,
  'head-def': (a) => all(a, '.hasHead.def > .hw').length > 0,
};

// facts is what a drawn frame shows of the data model, by axis.
function facts(app) {
  const f = {};
  const add = (axis, v) => { if (v != null && v !== '') (f[axis] = f[axis] || new Set()).add(v); };
  const words = fx.Situations.dense.Page.Words;
  for (const r of all(app, '.row[class*="k-"]')) {
    add('kind', [...r.classList].find((c) => c.startsWith('k-')).slice(2));
    for (const c of r.classList) if (c.startsWith('st-')) add('state', c.slice(3));
    const d = r.querySelector(':scope > .rowbtn > .dot');
    if (d) add('mark', d.classList.contains('waiting') ? 'waiting' : d.classList.contains('new') ? 'new' : 'none');
  }
  if (all(app, 'h1.ptitle').length) add('kind', 'title');
  if (all(app, 'h2.section').length) add('kind', 'section');
  if (all(app, '.para').length) add('kind', 'para');
  if (all(app, '.list').length) add('kind', 'list');
  if (all(app, 'pre.code').length) add('kind', 'code');
  if (all(app, '.option').length) add('kind', 'option');
  for (const e of all(app, '.board strong, .notices strong')) if (e.textContent) add('style', 'bold');
  for (const e of all(app, '.board code, .notices code')) if (!e.closest('pre')) add('style', 'code');
  for (const e of all(app, '.board .link, .notices .link')) if (e.textContent) add('style', 'link');
  for (const e of all(app, '.board .title, .board .para, .notices .title')) {
    if ([...e.childNodes].some((n) => n.nodeType === 3 && n.textContent.trim())) add('style', 'plain');
  }
  for (const w of all(app, '.board .word, .notices .word, .cpanel .word, .summary .word, .boardrow .right .word')) for (const t of ['quiet', 'need', 'move', 'normal']) if (w.classList.contains(t)) add('tone', t);
  for (const w of all(app, '.cstate, .crow .right .word')) add('status', w.textContent);
  for (const h of all(app, '.creply > .quiet:first-child')) add('settle', settleOf[h.textContent]);
  const bar = all(app, '.foot .bar [data-act]').map((b) => b.dataset.act);
  for (const a of bar) add('hint', a);
  if (bar.includes('answer')) add('action', 'Options');
  if (bar.includes('comment')) add('action', 'Comment');
  if (bar.includes('hide')) add('action', 'Hide');
  if (bar.includes('read')) add('action', 'Read');
  if (app.querySelector('.foot')) {
    if (all(app, '.bar.jumps').length && all(app, '.jumpkey').length) add('mode', 'goto');
    if (all(app, '.editor textarea').length) add('mode', 'editor');
    if (all(app, '.help .sheet').length) add('mode', 'help');
    if (all(app, '.cpanel').length) add('mode', 'comments');
    if (all(app, '.viewer img[src]').length) add('mode', 'image');
    if (!f.mode && all(app, '.bar .key').length) add('mode', 'normal');
    const b = app.querySelector('.banner');
    add('conn', visible(b) ? b.dataset.conn : 'live');
  }
  for (const s of all(app, '.status.need')) {
    const k = Object.keys(fx.Refusals).find((x) => fx.Refusals[x] === s.textContent);
    add('refusal', k);
  }
  void words;
  for (const [name, has] of Object.entries(DIFFS)) {
    let yes = false;
    try { yes = has(app); } catch { yes = false; } // a frame without that part
    if (yes) add('difference', name);
  }
  return f;
}

// overflows is what is wider than its box (and so cut or spilling): any
// element but what is cut on purpose with an ellipsis. Code too: it wraps.
function overflows(frame) {
  const out = [];
  for (const e of frame.querySelectorAll('*')) {
    if (e.closest('.sr, .skip, [hidden]') || !e.getClientRects().length) continue;
    const cs = getComputedStyle(e);
    if (cs.textOverflow === 'ellipsis' || cs.display === 'contents') continue;
    if (e.scrollWidth > e.clientWidth + 1 && e.clientWidth > 0) out.push((e.className || e.tagName).toString().slice(0, 40));
  }
  return out;
}

// ---- drawing the stories ----

function frameFor(kind, v) {
  const app = kind === 'home' ? createHome() : createApp();
  app.classList.add(THEME);
  const w = v.w || W;
  app.style.width = w + 'px';
  if (kind !== 'part') app.style.height = (v.h || (w < 600 ? 760 : 720)) + 'px';
  return app;
}

// scrollTo scrolls the frame's page as the variant says.
function scrollTo(app, how) {
  const s = app.querySelector('.scroller');
  if (how === 'bottom') { s.scrollTop = s.scrollHeight; return; }
  const want = DIFFS[how];
  for (let y = 0; y < s.scrollHeight; y += 6) {
    s.scrollTop = y;
    if (want(app)) return;
  }
}

function draw(kind, v, app, st) {
  const memo = newMemo();
  if (kind === 'home') {
    drawHome(app, fx.Index[v.index], v.cur || '', false);
    return;
  }
  if (kind === 'part') {
    renderPart(app, st.page, st.ui, memo, env, st.things);
    fit(app, st.page, st.ui);
    return;
  }
  render(app, st.page, st.ui, memo, env);
  pad(app);
  const un = fit(app, st.page, st.ui);
  if (un) { st.ui = { ...st.ui, unpinned: un }; render(app, st.page, st.ui, memo, env); fit(app, st.page, st.ui); }
  pad(app);
  scrolls(app, v, st.ui);
}

// scrolls puts what scrolls where the variant says (again once fonts and
// images are in: they move things).
function scrolls(app, v, ui) {
  if (!ui) return;
  if (ui.mode === 'help') {
    const sh = app.querySelector('.help .sheet');
    sh.scrollTop = v.sheetScroll === 'bottom' ? sh.scrollHeight : 0;
  }
  if (ui.panel) {
    const p = app.querySelector('.cpanel');
    p.scrollTop = 0;
    const c = p.querySelector('.row.cur');
    if (v.panelScroll === 'cursor' && c) p.scrollTop = c.getBoundingClientRect().bottom - p.getBoundingClientRect().bottom + 8;
  }
  if (v.scroll) scrollTo(app, v.scroll);
}

const stories = []; // {rec, app, v, kind}

function story(mod, v, i, kind, grid) {
  const nn = String(i + 1).padStart(2, '0');
  const id = `${mod}-${nn}`;
  const box = el('figure', 'g-story');
  box.id = 'story-' + id;
  box.dataset.module = mod;
  box.dataset.flags = [v.responsive ? 'responsive' : '', v.themed ? 'themed' : ''].join(' ').trim();
  if (v.focus) box.dataset.focus = v.focus;
  box.appendChild(el('figcaption', 'g-name', `${nn} ${v.name}`));
  grid.appendChild(box);
  const rec = { id, module: mod, name: v.name, nn, kind, file: `${mod}/${nn}-${v.name.replace(/[^\p{L}\p{N}]+/gu, '-').replace(/^-|-$/g, '').slice(0, 48)}.png`, flags: box.dataset.flags, first: i === 0, w: v.w || W, h: v.h || ((v.w || W) < 600 ? 760 : 720), focus: v.focus || '' };
  report.stories.push(rec);
  try {
    const st = kind === 'home' ? null : resolve(v, kind === 'part');
    const app = frameFor(kind, v);
    box.appendChild(app);
    draw(kind, v, app, st);
    const r = app.getBoundingClientRect();
    if (!r.width || !r.height || (kind === 'part' && !app.querySelector('.board').childElementCount && !app.querySelector('.notices').childElementCount)) throw new Error('drew nothing');
    if (st) {
      rec.fixture = v.fixture || 'dense';
      const u = st.ui;
      rec.ui = { cur: u.cur, open: u.open, mode: u.mode, panel: u.panel, editor: u.editor, image: u.image, status: u.status, statusTone: u.statusTone, showHidden: u.showHidden, back: u.back, busy: u.busy, how: 'person', start: false, at: 0 };
    }
    if (kind === 'home') rec.index = v.index;
    stories.push({ rec, app, v, kind, ui: st && st.ui });
  } catch (e) {
    box.classList.add('error');
    box.appendChild(el('pre', '', String(e && e.stack || e)));
    report.errors.push({ id, what: 'threw: ' + (e && e.message || e) });
  }
}

const declared = []; // {mod, axis, value, names}

function moduleSection(mod, m, kind) {
  if (ONLY && ONLY.split('/')[0] !== mod) return;
  const sec = el('section', 'g-module');
  sec.id = 'module-' + mod;
  const label = kind === 'part' ? 'parts/' + mod + '.js' : mod === 'pages' ? 'pages (whole pages)' : 'layers/' + mod + '.js';
  sec.appendChild(el('h2', '', label));
  const cov = Object.entries(m.covers || {}).map(([k, v]) => `${k}: ${Object.keys(v).join(' ')}`);
  const dif = Object.keys(m.differences || {});
  sec.appendChild(el('p', 'g-covers', 'covers ' + cov.join(' · ') + (dif.length ? ' · differences: ' + dif.join(' ') : '')));
  const grid = el('div', 'g-grid');
  sec.appendChild(grid);
  document.querySelector('.g-main').appendChild(sec);
  (m.variants || []).forEach((v, i) => {
    if (ONLY.includes('/') && ONLY !== `${mod}/${String(i + 1).padStart(2, '0')}`) return;
    story(mod, v, i, kind, grid);
  });
  for (const [axis, vals] of Object.entries(m.covers || {})) for (const [value, names] of Object.entries(vals)) declared.push({ mod, axis, value, names });
  for (const [value, names] of Object.entries(m.differences || {})) declared.push({ mod, axis: 'difference', value, names });
}

function controls() {
  for (const a of document.querySelectorAll('.g-controls a')) {
    const [k, v] = a.dataset.set.split('=');
    const p = new URLSearchParams(location.search);
    p.set(k, v);
    a.href = '?' + p.toString();
    if ((params.get(k) || { w: '1280', theme: 'light', cur: '0' }[k]) === v) a.classList.add('on');
  }
}

controls();
for (const [mod, m] of Object.entries(PARTS)) moduleSection(mod, m, 'part');
for (const [mod, m] of Object.entries(LAYERS)) moduleSection(mod, m, 'layer');
moduleSection('home', home, 'home');
moduleSection('pages', pages, 'layer');
await document.fonts.ready;
await Promise.all([...document.images].map((i) => i.decode().catch(() => {})));

// Each story's facts, from its DOM (one at a time: a story with a focus
// takes it, is read, and lets it go).
const shown = new Map();
for (const s of stories) {
  if (s.kind !== 'part' && s.kind !== 'home') pad(s.app); // images in: a layer may be taller now
  scrolls(s.app, s.v, s.ui);
  if (s.v.focus) { const f = s.app.querySelector(s.v.focus); if (f) f.focus({ preventScroll: true }); }
  shown.set(s.rec.name + '\u0000' + s.rec.module, facts(s.app));
  if (s.v.focus && document.activeElement) document.activeElement.blur();
  const over = overflows(s.app);
  if (over.length) report.errors.push({ id: s.rec.id, what: 'wider than its box: ' + [...new Set(over)].join(', ') });
}
// What a module declares, the variant it names must show. At the default
// width (a variant with its own width at that one); narrower, keys are
// left out by design. What holds at every width (EVERY_WIDTH) is checked
// at each.
const EVERY_WIDTH = new Set(['text-edge']);
if (!ONLY.includes('/')) {
  for (const d of declared) {
    if (W !== 1280 && !(d.axis === 'difference' && EVERY_WIDTH.has(d.value))) continue;
    for (const name of d.names) {
      const f = shown.get(name + '\u0000' + d.mod);
      if (!f) { report.errors.push({ id: d.mod, what: `${d.axis} ${d.value}: no variant "${name}"` }); continue; }
      if (!(f[d.axis] && f[d.axis].has(d.value))) report.errors.push({ id: d.mod, what: `${d.axis} ${d.value}: variant "${name}" does not show it` });
    }
  }
}
// For the script: every story's facts (to make or check the covers).
for (const r of report.stories) {
  const f = shown.get(r.name + '\u0000' + r.module) || {};
  r.facts = Object.fromEntries(Object.entries(f).map(([k, v]) => [k, [...v]]));
}
const out = document.querySelector('.g-report');
out.textContent = `${report.stories.length} stories, ${report.errors.length} problems` + report.errors.map((e) => `\n${e.id}: ${e.what}`).join('');
out.classList.toggle('bad', report.errors.length > 0);
report.ready = true;
