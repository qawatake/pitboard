// The gallery (/_gallery): every part of the report and every layer of the
// page, in each of its variants, drawn by the page's own render.js from
// fixtures.json (made by the Go code: internal/web/gallery_test.go).
//
// Each module owns its variants (export const variants) and says which
// values of the data model they cover (export const covers). Axes that are
// not a part's own are applied here to every variant: the width (1280 /
// 390), the theme (light / dark), and the cursor on the part. A few whole
// pages show the spacing between parts.
//
// When it is done, window.__gallery says what it drew and what went wrong
// (a story that threw, drew nothing, has something wider than its box but
// Code, or a module whose variants do not show what it says it covers):
// scripts/web-gallery.sh fails on any.

import { createApp, newMemo, render, renderPart, fit, PARTS, LAYERS } from '../render.js';
import { findThing } from '../parts/row.js';
import * as S from '../state.js';
import { el, plain } from '../dom.js';

const params = new URLSearchParams(location.search);
const W = Number(params.get('w')) || 1280;
const THEME = params.get('theme') === 'dark' ? 'dark' : 'light';
const CUR = params.get('cur') === '1';
const ONLY = params.get('only') || ''; // module, or module/NN
const env = { imageURL: () => '/gallery/sample.svg' };

// PAGES are whole pages: the spacing between parts.
const PAGES = {
  covers: {},
  variants: [
    { name: 'dense, all closed', ui: { cur: 'hook' }, responsive: true },
    { name: 'dense, several open', ui: { cur: 'hook' }, open: ['restart', 'hook', 'unit', 'web', 'olddiff', '決まったこと (3)'], responsive: true },
    { name: 'comments, open', fixture: 'comments', ui: { cur: 'c1' }, open: ['c1', 'c4', 'c6', 'f1'] },
    { name: 'long strings, open', fixture: 'long', ui: { cur: 'url' }, open: ['url', 'long', 'lask'], responsive: true },
    { name: 'hidden lines shown (H)', fixture: 'hidden', ui: { cur: 'v1', showHidden: true } },
  ],
};

const fx = await (await fetch('/gallery/fixtures.json')).json();
const report = { ready: false, stories: [], errors: [] };
window.__gallery = report;

// keyOf is the key of the line named name (an id, a key, a title).
function keyOf(page, name) {
  if (!name) return '';
  if (name === '#l:first') {
    const c = S.panelOrder(page)[0];
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
  const page = sit.Page;
  const ui = { ...S.initial(), ...(v.ui || {}), open: {}, unpinned: {} };
  for (const [k, on] of Object.entries((v.ui && v.ui.open) || {})) ui.open[keyOf(page, k)] = on;
  for (const n of v.open && v.open !== true ? v.open : []) ui.open[keyOf(page, n)] = true;
  for (const n of v.openAlso || []) ui.open[keyOf(page, n)] = true;
  let things = null;
  if (isPart) {
    const t = typeof v.pick === 'function' ? v.pick(page) : findThing(page, v.pick);
    if (!t) throw new Error(`nothing picked (${v.pick})`);
    things = [t];
    if (v.after) things.push(v.after(page));
    if (v.open === true) ui.open[t.Key] = true;
    if (CUR && t.Stop) ui.cur = t.Key;
  } else {
    ui.cur = keyOf(page, ui.cur);
    if (ui.editor) ui.editor = { ...ui.editor, at: keyOf(page, ui.editor.at), key: keyOf(page, ui.editor.key) };
    if (ui.panel && !ui.mode) ui.mode = 'comments';
    const t = S.index(page).things.get(ui.cur);
    if (t) {
      ui.sel = t.Token;
      const o = sit.Offers && sit.Offers[t.Key];
      const offer = v.locked ? fx.Locked : v.placed ? sit.Placed[t.Key] : o && (S.isOpen(ui, t) ? o.Open : o.Closed);
      if (offer) ui.offer = { key: t.Key, sel: t.Token, offer };
    }
  }
  return { page, ui, things };
}

// facts is what a story shows of the data model (for covers).
function facts(st, frame) {
  const f = {};
  const add = (axis, v) => { if (v != null && v !== '') (f[axis] = f[axis] || new Set()).add(v); };
  const settle = { 対応中: 'doing', 保留: 'hold', 対応済: 'done' };
  const comment = (c) => { add('status', c.Word && c.Word.Text); add('settle', settle[c.Word && c.Word.Text]); add('tone', c.Word && c.Word.Tone); };
  const walk = (t) => {
    if (!t) return;
    add('kind', t.Kind);
    if (t.Stop) add('mark', t.Mark || 'none');
    add('state', t.State);
    for (const s of t.Title || []) add('style', s.Style);
    const r = t.Right || {};
    if (r.Input && r.Input.Text) add('tone', r.Input.Tone);
    if (r.Status && r.Status.Text) add('tone', r.Status.Tone);
    for (const c of t.Comments || []) comment(c);
    for (const x of [...(t.Body || []), ...(t.Kids || []), ...(t.Subs || [])]) walk(x);
  };
  if (st.things) st.things.forEach(walk);
  else {
    add('mode', st.ui.mode);
    add('conn', st.ui.conn);
    for (const b of frame.querySelectorAll('.foot [data-act]')) if (!b.hidden) add('hint', b.dataset.act);
    const t = S.index(st.page).things.get(st.ui.cur);
    const a = (t && t.Actions) || {};
    for (const [k, v] of Object.entries(a)) if (v && (!Array.isArray(v) || v.length)) add('action', k);
    if (st.ui.panel) for (const c of st.page.Comments || []) comment(c);
  }
  return f;
}

// overflows is what is wider than its box (and so cut or spilling): any
// element but Code and what is cut on purpose with an ellipsis.
function overflows(frame) {
  const out = [];
  for (const e of frame.querySelectorAll('*')) {
    if (e.closest('pre.code, .sr, .skip, [hidden]') || !e.getClientRects().length) continue;
    const cs = getComputedStyle(e);
    if (cs.textOverflow === 'ellipsis' || cs.display === 'contents') continue;
    if (e.scrollWidth > e.clientWidth + 1 && e.clientWidth > 0) out.push((e.className || e.tagName).toString().slice(0, 40));
  }
  return out;
}

function frameFor(isPart, v) {
  const app = createApp();
  app.classList.add(THEME);
  app.style.width = W + 'px';
  if (!isPart) app.style.height = (v.h || (W < 600 ? 760 : 720)) + 'px';
  return app;
}

function story(mod, name, v, i, isPart, grid) {
  const id = `${mod}-${String(i + 1).padStart(2, '0')}`;
  const box = el('figure', 'g-story');
  box.id = 'story-' + id;
  box.dataset.module = mod;
  box.dataset.flags = [v.responsive ? 'responsive' : '', v.themed ? 'themed' : ''].join(' ').trim();
  box.appendChild(el('figcaption', 'g-name', `${String(i + 1).padStart(2, '0')} ${v.name}`));
  grid.appendChild(box);
  const rec = { id, module: mod, name: v.name, file: `${mod}/${String(i + 1).padStart(2, '0')}-${v.name.replace(/[^\p{L}\p{N}]+/gu, '-').replace(/^-|-$/g, '').slice(0, 48)}.png`, flags: box.dataset.flags };
  report.stories.push(rec);
  try {
    const st = resolve(v, isPart);
    const app = frameFor(isPart, v);
    box.appendChild(app);
    const memo = newMemo();
    if (isPart) {
      renderPart(app, st.page, st.ui, memo, env, st.things, { heading: !!v.heading });
      fit(app, st.page, st.ui);
    } else {
      render(app, st.page, st.ui, memo, env);
      app.style.setProperty('--foot-h', app.querySelector('.foot').offsetHeight + 'px');
      const un = fit(app, st.page, st.ui);
      if (un) { st.ui = { ...st.ui, unpinned: un }; render(app, st.page, st.ui, memo, env); fit(app, st.page, st.ui); }
      app.style.setProperty('--foot-h', app.querySelector('.foot').offsetHeight + 'px');
      if (st.ui.mode === 'help') app.querySelector('.help .sheet').scrollTop = 0;
    }
    const r = app.getBoundingClientRect();
    if (!r.width || !r.height || (isPart && !app.querySelector('.board').childElementCount)) throw new Error('drew nothing');
    const over = overflows(app);
    if (over.length) report.errors.push({ id, what: 'wider than its box: ' + [...new Set(over)].join(', ') });
    return facts(st, app);
  } catch (e) {
    box.classList.add('error');
    box.appendChild(el('pre', '', String(e && e.stack || e)));
    report.errors.push({ id, what: 'threw: ' + (e && e.message || e) });
    return {};
  }
}

function moduleSection(mod, m, isPart) {
  if (ONLY && ONLY.split('/')[0] !== mod) return;
  const sec = el('section', 'g-module');
  sec.id = 'module-' + mod;
  sec.appendChild(el('h2', '', (isPart ? 'parts/' : mod === 'pages' ? '' : 'layers/') + mod + (mod === 'pages' ? ' (whole pages)' : '.js')));
  sec.appendChild(el('p', 'g-covers', 'covers ' + Object.entries(m.covers || {}).map(([k, v]) => `${k}: ${v.join(' ')}`).join(' · ')));
  const grid = el('div', 'g-grid');
  sec.appendChild(grid);
  document.querySelector('.g-main').appendChild(sec);
  const seen = {};
  (m.variants || []).forEach((v, i) => {
    if (ONLY.includes('/') && ONLY !== `${mod}/${String(i + 1).padStart(2, '0')}`) return;
    const f = story(mod, null, v, i, isPart, grid);
    for (const [axis, vs] of Object.entries(f)) for (const x of vs) (seen[axis] = seen[axis] || new Set()).add(x);
  });
  // Checked at the default width: narrower, keys are left out by design.
  if (ONLY.includes('/') || W !== 1280) return;
  // What the module says it covers, its variants must show.
  for (const [axis, vs] of Object.entries(m.covers || {})) {
    for (const x of vs) {
      if (!(seen[axis] && seen[axis].has(x))) report.errors.push({ id: mod, what: `covers ${axis} ${x}, but no variant shows it` });
    }
  }
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
for (const [mod, m] of Object.entries(PARTS)) moduleSection(mod, m, true);
for (const [mod, m] of Object.entries(LAYERS)) moduleSection(mod, m, false);
moduleSection('pages', PAGES, false);
await document.fonts.ready;
const out = document.querySelector('.g-report');
out.textContent = `${report.stories.length} stories, ${report.errors.length} problems` + report.errors.map((e) => `\n${e.id}: ${e.what}`).join('');
out.classList.toggle('bad', report.errors.length > 0);
report.ready = true;
void plain;
