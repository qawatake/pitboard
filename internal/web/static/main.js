// pitboard in a browser: the wiring. The rules (what a line is, its marks,
// its signs, its words, what a key does on it and what to say when it does
// nothing) all come from the server (internal/page). state.js says how the
// page's own state changes, keys.js what a key means, render.js draws,
// net.js talks to the server; this file holds the state and does what
// only a live page does: focus, scrolling, timers, storage.

import { connect, Offline, Gone } from './net.js';
import { createApp, newMemo, render, fit } from './render.js';
import * as S from './state.js';
import { intentOf } from './keys.js';
import { plain } from './dom.js';
import * as editorLayer from './layers/editor.js';
import * as barLayer from './layers/bar.js';

// The board's own path (/s/<id>): its calls are under it. The files are
// shared by every board.
const BASE = (location.pathname.match(/^\/s\/[^/]+/) || [''])[0];

const tabId = (() => {
  try {
    let id = sessionStorage.getItem('pitboard-tab');
    if (!id) {
      id = Array.from(crypto.getRandomValues(new Uint8Array(12)), (b) => b.toString(16).padStart(2, '0')).join('');
      sessionStorage.setItem('pitboard-tab', id);
    }
    return id;
  } catch { return 'tab'; }
})();

const store = {
  get(k) { try { return sessionStorage.getItem(k); } catch { return null; } },
  set(k, v) { try { sessionStorage.setItem(k, v); } catch { /* the draft lives only on screen */ } },
  del(k) { try { sessionStorage.removeItem(k); } catch { /* nothing */ } },
};
// A draft is the board's and its target's: the same line id or comment
// number on another board (another /s/<id>) is another draft.
const draftKey = (ed) => 'pitboard-draft:' + BASE + ':' + ed.kind + ':' + (ed.kind === 'edit' ? ed.n : ed.key);

const net = connect(BASE, tabId);
const env = { imageURL: (n) => BASE + '/api/image/' + n };
const app = createApp();
document.body.appendChild(app);
const $ = (sel) => app.querySelector(sel);
const scroller = $('.scroller');

let page = null;
let etag = '';
let ui = S.initial();
const memo = newMemo();
let composing = false;
let pending = false;
let hintSeq = 0;
let hintTimer = 0;
let retryTimer = 0;
let stream = null; // the event stream (reopen: after the board came back)
let returnTo = null; // where the focus goes back to when a dialog closes
const now = () => Date.now();
const textarea = () => $('.editor textarea');

// ---- drawing ----

// draw draws ui; only 'cursor' when nothing but the cursor moved.
function draw(only) {
  if (!page) return;
  if (only === 'cursor') {
    render(app, page, ui, memo, env, 'cursor');
    barLayer.fit($('.foot'));
    return;
  }
  // Keep what the person is looking at where it is on screen.
  const anchor = memo.rows.get(ui.cur) || $('.editor');
  const before = anchor && anchor.isConnected ? anchor.getBoundingClientRect().top : null;
  const ae = document.activeElement;
  const onRow = !!(ae && ae.closest && ae.closest('.row'));
  const ta = textarea();
  const sel = ta && ta.isConnected && document.activeElement === ta ? [ta.selectionStart, ta.selectionEnd] : null;
  render(app, page, ui, memo, env);
  const un = fit(app, page, ui);
  if (un) {
    ui = { ...ui, unpinned: un };
    render(app, page, ui, memo, env);
    fit(app, page, ui);
  }
  syncPadding();
  const nowEl = memo.rows.get(ui.cur) || $('.editor');
  if (nowEl && before != null) scroller.scrollBy(0, nowEl.getBoundingClientRect().top - before);
  const t2 = textarea();
  if (sel && t2) {
    if (document.activeElement !== t2) t2.focus({ preventScroll: true });
    t2.setSelectionRange(sel[0], sel[1]);
  } else if (onRow) {
    focusCur();
  }
}

// set changes the state and draws it.
function set(next, only) {
  ui = next;
  draw(only);
}

// say puts msg on the bottom line (read out again even when it repeats).
function say(msg, tone) {
  const s = $('.status');
  ui = S.say(ui, msg, tone);
  s.textContent = '';
  requestAnimationFrame(() => { draw('cursor'); syncPadding(); });
}

// clear takes the message off when the rule (page.MsgClears) says ev does.
function clear(ev) {
  const was = ui.status;
  ui = S.clear(ui, page, ev);
  if (was !== ui.status) draw('cursor');
}

const navOf = (key) => { const r = memo.rows.get(key); return r && r.querySelector(':scope > .rowbtn'); };

// focusCur puts the focus on the cursor's row: the focus is the cursor.
function focusCur() {
  if (ui.mode === 'editor' || ui.mode === 'help' || ui.mode === 'image') return;
  if (ui.panel && !ui.cur.startsWith('#')) { $('.cpanel').focus({ preventScroll: true }); return; }
  const b = navOf(ui.cur);
  if (b && document.activeElement !== b) b.focus({ preventScroll: true });
}

// ---- the server ----

function setConn(conn, msg) {
  if (ui.conn === conn && (!msg || ui.connMsg === msg)) return;
  ui = S.setConn(ui, conn, msg);
  clearInterval(retryTimer);
  // Keep trying quietly while offline: the first page that comes back
  // clears it. A board that is gone stays gone.
  if (conn === 'offline') retryTimer = setInterval(() => { etag = ''; refresh(); }, 2000);
  // A board that is gone comes back when it is registered again under the
  // same id (pitboard serve in its directory): ask now and then.
  if (conn === 'gone') retryTimer = setInterval(() => { etag = ''; refresh(true); }, 4000);
  draw();
}

function failed(e) {
  if (e instanceof Gone) { setConn('gone', e.message); return true; }
  if (e instanceof Offline) { setConn('offline'); return true; }
  return false;
}

async function refresh(retry) {
  if (composing) { pending = true; return; }
  if (ui.conn === 'gone' && !retry) return;
  const wasGone = ui.conn === 'gone';
  let d;
  try {
    const r = await net.request('/api/page' + (ui.showHidden ? '?hidden=1' : ''), undefined, etag ? { 'If-None-Match': etag } : {});
    if (r.status === 304) {
      setConn('live');
      requestHint(); // the same page as drawn
      return;
    }
    d = await r.json();
    etag = r.headers.get('ETag') || '';
  } catch (e) {
    if (!failed(e)) console.warn('pitboard: page', e);
    return;
  }
  const first = !page;
  if (wasGone) stream.reopen();
  else if (stream) stream.revive();
  page = d;
  if (first) fromURL();
  ui = S.placed(ui, page, now());
  ui = S.setConn(ui, 'live');
  clearInterval(retryTimer);
  draw();
  if (first) focusCur();
  requestHint();
  if (d.Say) say(d.Say);
}

async function act(action, extra = {}) {
  if (ui.busy) return null;
  try {
    const r = await net.api('/api/act', { Action: action, ShowHidden: ui.showHidden, ...extra });
    // A key that did nothing says why: that needs the person.
    if (r.Message) say(r.Message, r.OK || r.Changed || action === 'readall' ? '' : 'need');
    return r;
  } catch (e) {
    if (!failed(e)) say('できませんでした: ' + e.message);
    return null;
  }
}

// requestHint asks the server what works on the cursor's line now.
async function requestHint() {
  clearTimeout(hintTimer);
  const t = S.index(page).things.get(ui.cur);
  if (!t || ui.panel) return;
  const seq = ++hintSeq;
  const key = ui.cur;
  const sel = ui.sel;
  let r;
  try {
    r = await net.api('/api/act', { Action: 'hint', Target: t.Key, Seen: t.Token, Cur: S.facts(ui, t, now()) });
  } catch { return; }
  if (seq !== hintSeq || !r || !r.Offer) return;
  ui = { ...ui, offer: { key, sel, offer: r.Offer } };
  draw('cursor');
  if (r.Offer.Recheck > 0) hintTimer = setTimeout(requestHint, r.Offer.Recheck + 20);
}

// fromURL puts the page in the state ?ui= says (JSON over S.initial()),
// only when the server serves the gallery's check (page.Gallery: pitboard
// serve --foreground --gallery): scripts/web-gallery.sh compares this live
// page with the gallery's drawing of the same state.
function fromURL() {
  if (!page.Gallery) return;
  const q = new URLSearchParams(location.search).get('ui');
  if (!q) return;
  let v;
  try { v = JSON.parse(q); } catch { return; }
  ui = { ...ui, ...v, conn: 'live' };
  document.body.dataset.ui = 'set';
  if (ui.showHidden) { etag = ''; setTimeout(refresh, 0); }
}

// ---- the cursor ----

function setCursor(key, how, focus = true) {
  ui = S.setCursor(ui, page, key, how, now());
  draw('cursor');
  requestHint();
  if (focus) {
    const b = navOf(key);
    if (b && document.activeElement !== b) b.focus({ preventScroll: true });
    if (b) b.scrollIntoView({ block: 'nearest' });
  }
}

function toggle(key) {
  if (key.startsWith('#')) { goToComment(Number(key.slice(3))); return; }
  set(S.toggle(ui, page, key));
  const b = navOf(key);
  if (b) b.focus({ preventScroll: true });
  requestHint();
}

function jumpNext() {
  const target = S.next(ui, page);
  if (!target) { say(S.words(page, 'noMarked')); return; }
  const fs = (target.Folds || []).filter((f) => !ui.open[f]);
  if (fs.length) set(S.setOpen(ui, fs));
  setCursor(target.Key, 'app');
}

function goTo(t) {
  const i = t.Jump - 1;
  const key = page.JumpTo[i];
  if (key) setCursor(key, 'app');
  const h = app.querySelector(`[data-jump="${i + 1}"]`);
  if (h) h.scrollIntoView({ block: 'start' });
}

// ---- the panel ----

function openPanel() {
  if (ui.panel || !page) return;
  returnTo = document.activeElement;
  clear('panel');
  set(S.openPanel(ui));
  const keys = S.navKeys(page, ui);
  if (keys.length) setCursor(keys[0], 'person', true);
  else $('.cpanel').focus({ preventScroll: true });
  // It opens at its top: the heading in view, the first comment under it.
  $('.cpanel').scrollTop = 0;
}

function closePanel(back = true) {
  if (!ui.panel) return;
  clear('panel');
  set(S.closePanel(ui));
  requestHint();
  if (!back) return;
  const r = returnTo;
  if (r && r.isConnected && r !== document.body) r.focus({ preventScroll: true });
  else focusCur();
}

function goToComment(n) {
  const c = S.commentOf(page, n);
  const key = c && S.index(page).byTarget.get(c.Target);
  if (!key) return;
  closePanel(false);
  const up = (S.index(page).parents.get(key) || []).filter((k) => !S.isOpen(ui, S.index(page).things.get(k)));
  if (up.length) set(S.setOpen(ui, up));
  setCursor(key, 'person');
  const r = memo.rows.get(key);
  if (r) r.scrollIntoView({ block: 'center' });
}

// ---- actions ----

async function press(k) {
  const t = S.index(page).things.get(ui.cur);
  if (!t) return;
  const r = await act('key', { Key: k, Target: t.Key, Seen: t.Token, Cur: S.facts(ui, t, now()) });
  if (!r) return;
  switch (r.Do) {
    case 'open': set(S.setOpen(ui, [t.Key])); requestHint(); return;
    case 'comment': openEditor({ kind: 'comment', key: t.Key, token: t.Token, at: t.Key, label: '「' + plain(t.Title) + '」にコメント' }); return;
    case 'hide': if (r.OK) ui = { ...ui, going: t.Key }; break;
  }
  if (r.OK || r.Changed) await refresh();
  else requestHint();
  focusCur();
}

async function retract(n) {
  const r = await act('retract', { N: n, Cur: { How: ui.how, Ago: Math.round(now() - ui.at) } });
  if (r && r.OK) { ui = { ...ui, going: ui.cur }; await refresh(); }
  focusCur();
}

async function undo() {
  ui = { ...ui, going: ui.cur };
  const r = await act('undo');
  if (r && r.OK) await refresh();
  ui = { ...ui, going: '' };
  focusCur();
}

async function readAll() {
  if (!page.CanWrite) return;
  const r = await act('readall');
  if (r && r.OK) await refresh();
}

async function send() {
  if (ui.busy || !page.CanWrite || ui.conn !== 'live') return;
  ui = { ...ui, busy: true };
  document.body.setAttribute('aria-busy', 'true');
  say('送っています…');
  try {
    const r = await net.api('/api/send', {});
    say(r.Message);
  } catch (e) {
    if (!failed(e)) say('送れませんでした: ' + e.message);
  } finally {
    ui = { ...ui, busy: false };
    document.body.removeAttribute('aria-busy');
    await refresh();
    draw('cursor');
  }
}

function toggleHidden() {
  ui = { ...ui, showHidden: !ui.showHidden, going: ui.cur };
  etag = '';
  refresh();
}

// ---- the editor ----

function openEditor(ed) {
  if (!page.CanWrite || ui.conn !== 'live') return;
  if (ui.mode !== 'editor') returnTo = document.activeElement;
  clear('editor');
  const draft = store.get(draftKey(ed));
  set(S.openEditor(ui, { ...ed, text: draft ?? (ed.text || '') }));
  const ta = textarea();
  if (!ta) return;
  ta.focus();
  ta.setSelectionRange(ta.value.length, ta.value.length);
  if (!$('.editor').classList.contains('loose')) $('.editor').scrollIntoView({ block: 'nearest' });
}

function writeLoose() {
  if (ui.panel) closePanel(false);
  openEditor({ kind: 'comment', key: '', at: ui.cur, label: '宛先なしのコメント' });
}

function editComment(n) {
  const c = S.commentOf(page, n);
  if (c) openEditor({ kind: 'edit', n: c.N, at: ui.cur, text: c.Text, label: `コメント ${c.N} を直す` });
}

function closeEditor() {
  const at = ui.editor && ui.editor.at;
  set(S.closeEditor(ui));
  const b = (at && navOf(at)) || navOf(ui.cur);
  if (b) b.focus({ preventScroll: true });
  else (ui.panel ? $('.cpanel') : $('.board')).focus({ preventScroll: true });
}

async function place() {
  const ed = ui.editor;
  const text = textarea().value;
  let r;
  try {
    if (ed.kind === 'edit') r = await net.api('/api/act', { Action: 'edit', N: ed.n, Text: text });
    else r = await net.api('/api/act', { Action: 'comment', Target: ed.key || '', Seen: ed.token || '', Text: text });
  } catch (e) {
    if (!failed(e)) say('置けませんでした: ' + e.message);
    return;
  }
  say(r.Message);
  if (!r.OK) return;
  store.del(draftKey(ed));
  closeEditor();
  await refresh();
}

function edited() {
  const ta = textarea();
  if (!ta || !ui.editor) return;
  ui = S.editText(ui, ta.value);
  store.set(draftKey(ui.editor), ta.value);
  editorLayer.box({ page, ui, memo, env, words: (k) => S.words(page, k) });
}

function upload(file) {
  if (file.size > page.ImageMax) { say(S.words(page, 'imageTooLarge')); return; }
  const rd = new FileReader();
  rd.onload = async () => {
    const data = String(rd.result).replace(/^data:[^,]*,/, '');
    let r;
    try {
      r = await net.api('/api/image', { Data: data });
    } catch (e) {
      if (!failed(e)) say('貼れませんでした: ' + e.message);
      return;
    }
    say(r.Message);
    if (!r.OK) return;
    const ta = textarea();
    ta.setRangeText(`[Image #${r.Image}]`, ta.selectionStart, ta.selectionEnd, 'end');
    edited();
    ta.focus();
  };
  rd.readAsDataURL(file);
}

// ---- dialogs ----

// trap keeps the focus inside dialog while it is open.
function trap(e, dialog) {
  const fs = [...dialog.querySelectorAll('button, a[href], textarea, [tabindex]:not([tabindex="-1"])')]
    .filter((x) => !x.disabled && x.getClientRects().length);
  e.preventDefault();
  if (!fs.length) return;
  const i = fs.indexOf(document.activeElement);
  fs[(i + (e.shiftKey ? -1 : 1) + fs.length) % fs.length].focus();
}

function openHelp() {
  returnTo = document.activeElement;
  clear('panel');
  set(S.openHelp(ui));
  const sheet = $('.help .sheet');
  sheet.scrollTop = 0;
  sheet.querySelector('h2').focus({ preventScroll: true });
}
function closeHelp() {
  set(S.closeHelp(ui));
  if (returnTo && returnTo.isConnected) returnTo.focus({ preventScroll: true });
}
function openImage(n, from) {
  returnTo = from || document.activeElement;
  clear('panel');
  set(S.openImage(ui, n));
  $('.viewer-close').focus();
}
function closeImage() {
  set(S.closeImage(ui));
  if (returnTo && returnTo.isConnected) returnTo.focus({ preventScroll: true });
}

// ---- keys and clicks ----

// doAct is what a button of the bar (data-act) does.
function doAct(what, key) {
  switch (what) {
    case 'open': case 'close': toggle(ui.cur); break;
    case 'undo': undo(); break;
    case 'next': jumpNext(); break;
    case 'jump': set(S.startGoto(ui, page)); break;
    case 'send': send(); break;
    case 'write': writeLoose(); break;
    case 'keys': openHelp(); break;
    case 'edit': { const c = S.cursorComment(page, ui); if (c) editComment(c.N); break; }
    case 'retract': { const c = S.cursorComment(page, ui); if (c) retract(c.N); break; }
    case 'go': { const c = S.cursorComment(page, ui); if (c) goToComment(c.N); break; }
    case 'closePanel': closePanel(); break;
    case 'place': place(); break;
    case 'stop': closeEditor(); break;
    case 'endGoto': set(S.endGoto(ui)); focusCur(); break;
    case 'jumpTo': { const t = S.jumpOf(page, key); set(S.endGoto(ui)); if (t) goTo(t); break; }
    case 'closeHelp': closeHelp(); break;
    case 'closeImage': closeImage(); break;
    default: press(key); // r c x and an option's key
  }
}

function run(it, e) {
  switch (it.do) {
    case 'none': e.preventDefault(); return;
    case 'trap': trap(e, $(it.in)); return;
    case 'closeImage': e.preventDefault(); closeImage(); return;
    case 'closeHelp': e.preventDefault(); closeHelp(); return;
    case 'closePanel': e.preventDefault(); closePanel(); return;
  }
  e.preventDefault();
  clear('key');
  switch (it.do) {
    case 'goto': {
      const t = S.jumpOf(page, it.key);
      set(S.endGoto(ui));
      if (t) goTo(t); else focusCur();
      return;
    }
    case 'move': setCursor(S.move(ui, page, it.d, now()).cur, 'person'); return;
    case 'toggle': toggle(ui.cur); return;
    case 'next': jumpNext(); return;
    case 'help': openHelp(); return;
    case 'undo': undo(); return;
    case 'send': send(); return;
    case 'write': writeLoose(); return;
    case 'showHidden': toggleHidden(); return;
    case 'readAll': readAll(); return;
    case 'startGoto': set(S.startGoto(ui, page)); return;
    case 'editComment': editComment(it.n); return;
    case 'retract': retract(it.n); return;
    case 'press': press(it.key); return;
  }
}

function onKey(e) {
  // Every key the person presses is the next key: the message goes
  // (page.MsgClears), enter on a row and esc too. Not a lone modifier, not
  // what is typed into the editor.
  if (page && !/^(Shift|Control|Alt|Meta|CapsLock)$/.test(e.key) && !(e.target && e.target.tagName === 'TEXTAREA') && !e.isComposing) clear('key');
  if (e.target && e.target.tagName === 'TEXTAREA') { editorKey(e); if (e.defaultPrevented) return; }
  const it = intentOf(e, ui, page);
  if (!it) {
    // Any other key the person presses on the page is still the next key.
    if (page && !e.ctrlKey && !e.metaKey && !e.altKey && e.key.length === 1 && !(e.target && e.target.tagName === 'TEXTAREA')) clear('key');
    return;
  }
  run(it, e);
}

function editorKey(e) {
  if (e.isComposing || e.keyCode === 229) return; // the input method's enter
  if (e.key === 'Escape') { e.preventDefault(); closeEditor(); return; }
  if (e.key === 'Enter' && !e.shiftKey && !e.altKey && !e.ctrlKey && !e.metaKey) { e.preventDefault(); place(); }
  // alt+↑ ↓ (as the terminal's): the page half a screen up / down, the
  // focus staying in the text (what is under a floating editor can be
  // read without leaving it).
  if (e.altKey && !e.ctrlKey && !e.metaKey && (e.key === 'ArrowUp' || e.key === 'ArrowDown')) {
    e.preventDefault();
    scroller.scrollBy(0, (e.key === 'ArrowUp' ? -1 : 1) * Math.round(scroller.clientHeight / 2));
  }
}

// syncPadding keeps the end of the page above the bottom, however tall
// the message above the keys is, and above a floating layer (the m editor,
// the comment panel) while one is open, however tall it grows: the page
// gets that much more room at its end (--float-h), so every line can be
// scrolled to above it. Each layer is watched as it grows.
const floatWatch = window.ResizeObserver ? new ResizeObserver(() => syncPadding()) : null;
const watched = new WeakSet();
function syncPadding() {
  const h = $('.foot').offsetHeight;
  app.style.setProperty('--foot-h', h + 'px');
  let fh = 0;
  for (const f of app.querySelectorAll('.editor.loose, .cpanel:not([hidden])')) {
    if (floatWatch && !watched.has(f)) { floatWatch.observe(f); watched.add(f); }
    fh = Math.max(fh, f.offsetHeight);
  }
  app.style.setProperty('--float-h', fh ? fh + 'px' : '0px');
}

function start() {
  document.addEventListener('keydown', onKey);
  // Focus is the cursor: a row reached by Tab or a click is the cursor's.
  app.addEventListener('focusin', (e) => {
    const b = e.target;
    if (!b || !b.dataset || !b.dataset.nav || b.dataset.nav === ui.cur) return;
    clear('pointer');
    setCursor(b.dataset.nav, 'person', false);
  });
  app.addEventListener('click', (e) => {
    const tg = e.target;
    if (!tg.closest) return;
    const img = tg.closest('[data-image]');
    if (img) { e.stopPropagation(); openImage(Number(img.dataset.image), img); return; }
    const go = tg.closest('[data-go]');
    if (go) { e.stopPropagation(); goToComment(Number(go.dataset.go)); return; }
    const a = tg.closest('[data-act]');
    if (a && a.tagName === 'BUTTON') { e.stopPropagation(); doAct(a.dataset.act, a.dataset.key); return; }
    if (tg.closest('.b-comments')) { e.stopPropagation(); if (ui.panel) closePanel(); else openPanel(); return; }
    if (tg === $('.help')) { closeHelp(); return; }
    if (tg === $('.viewer')) { closeImage(); return; }
    const b = tg.closest('.rowbtn');
    if (!b || tg.closest('a')) return;
    const key = b.dataset.nav;
    clear(e.detail === 0 ? 'key' : 'pointer');
    if (key !== ui.cur) setCursor(key, 'person', false);
    const t = S.index(page).things.get(key);
    // A comment goes to its line by enter (a click from the keyboard has
    // no detail); clicked, it is only chosen (its line's title goes).
    if (key.startsWith('#') ? e.detail === 0 : t && t.CanOpen) toggle(key);
    else b.focus({ preventScroll: true });
  });
  // A click outside the panel closes it (not one in a dialog over it).
  document.addEventListener('pointerdown', (e) => {
    if (!ui.panel || ui.mode === 'help' || ui.mode === 'image') return;
    const tg = e.target;
    if (!tg.closest || tg.closest('.cpanel, .b-comments, .help, .viewer, .foot')) return;
    closePanel();
  });
  app.addEventListener('input', (e) => { if (e.target.tagName === 'TEXTAREA') edited(); });
  app.addEventListener('compositionstart', () => { composing = true; });
  app.addEventListener('compositionend', () => { composing = false; if (pending) { pending = false; refresh(); } });
  app.addEventListener('paste', (e) => {
    if (e.target.tagName !== 'TEXTAREA') return;
    const files = [...(e.clipboardData ? e.clipboardData.files : [])].filter((f) => f.type.startsWith('image/'));
    if (files.length) { e.preventDefault(); files.forEach(upload); }
  });
  app.addEventListener('dragover', (e) => { if (e.target.tagName === 'TEXTAREA') e.preventDefault(); });
  app.addEventListener('drop', (e) => {
    if (e.target.tagName !== 'TEXTAREA') return;
    const files = [...(e.dataTransfer ? e.dataTransfer.files : [])].filter((f) => f.type.startsWith('image/'));
    if (files.length) { e.preventDefault(); files.forEach(upload); }
  });
  if (window.ResizeObserver) {
    new ResizeObserver(() => { syncPadding(); if (page) { const un = fit(app, page, ui); if (un) set({ ...ui, unpinned: un }); } }).observe(app);
    new ResizeObserver(syncPadding).observe($('.foot'));
  }
  syncPadding();
  stream = net.events({
    changed: () => refresh(),
    open: () => refresh(),
    lost: () => { if (ui.conn === 'live') { setConn('offline'); refresh(); } },
    gone: (msg) => setConn('gone', msg),
  });
  refresh();
}

start();
