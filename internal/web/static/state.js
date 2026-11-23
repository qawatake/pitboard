// The page's own state (uiState) and how it changes: pure functions of
// (uiState, page, now) that return a new uiState. Nothing here touches the
// DOM, the network or a timer. What a key does on a line, and what to say,
// is the server's (internal/page); this is only what is open, where the
// cursor is and how it came there, which mode the page is in.
//
// uiState:
//   cur        the cursor's line key ("#l:N": comment N in the panel)
//   start, how, at, sel   how the cursor came there (the server's guard
//              reads them): start (not moved since the page put it there),
//              how (person / app / start), at (ms), sel (the line's token then)
//   lastIndex  where the cursor was among the lines (to stay near it)
//   open       {key: bool}: what the person opened or closed
//   mode       one of MODES (below); panel: the comment panel is open
//              (under an editor, the keys or an image opened from it)
//   editor     {kind: comment|edit, key, token, n, at, label, text}
//   image      the image shown large (mode image)
//   status, statusSeq   the message on the bottom line (seq: said again)
//   conn, connMsg   one of CONNS, and what the banner says
//   showHidden H shows hidden lines
//   offer      the server's offer for the cursor's line {key, sel, offer}
//   busy       sending
//   unpinned   {key: true}: notices opened too tall to pin
//   reportCur  the report's cursor while the panel has it
//   going      the line the person's own act is taking away

// MODES is web.Modes (a Go test checks): each has a variant in the layer
// that owns it.
export const MODES = ["normal", "goto", "editor", "help", "comments", "image"];
// CONNS is web.Conns.
export const CONNS = ["live", "offline", "gone"];

export function initial() {
  return {
    cur: '', start: true, how: 'start', at: 0, sel: '', lastIndex: 0, open: {},
    mode: 'normal', panel: false, editor: null, image: 0,
    status: '', statusSeq: 0, conn: 'live', connMsg: '', showHidden: false,
    offer: null, busy: false, unpinned: {}, reportCur: null, going: '',
  };
}

const own = (o, k) => Object.prototype.hasOwnProperty.call(o, k);
export const isOpen = (ui, t) => (own(ui.open, t.Key) ? ui.open[t.Key] : !!t.Open);
export const words = (page, k) => (page && page.Words && page.Words[k]) || '';
const base = (ui) => (ui.panel ? 'comments' : 'normal');

// index is the lines of page one can point at (by key), the lines one can
// open that hold each (outermost first), and the line of each Target.
export function index(page) {
  const things = new Map();
  const parents = new Map();
  const byTarget = new Map();
  const walk = (ts, up) => {
    for (const t of ts || []) {
      if (t.Stop) {
        things.set(t.Key, t);
        parents.set(t.Key, up);
        if (t.Target && !byTarget.has(t.Target)) byTarget.set(t.Target, t.Key);
      }
      walk(t.Kids, t.Stop && t.CanOpen ? [...up, t.Key] : up);
    }
  };
  if (page) { walk(page.Notices, []); walk(page.Things, []); }
  return { things, parents, byTarget };
}

export const comments = (page) => (page && page.Comments) || [];
export const commentOf = (page, n) => comments(page).find((c) => c.N === n);

// panelOrder is the comments as the panel lists them: those the AI has not
// settled first, then the settled; newest first in each.
export function panelOrder(page) {
  const cs = [...comments(page)].sort((a, b) => b.N - a.N);
  return [...cs.filter((c) => !c.Finished), ...cs.filter((c) => c.Finished)];
}

// cursorComment is the comment the cursor is on in the panel (null none).
export function cursorComment(page, ui) {
  return ui.panel && ui.cur.startsWith('#l:') ? commentOf(page, Number(ui.cur.slice(3))) : null;
}

// navKeys is the keys of the lines one can point at, in the order shown.
export function navKeys(page, ui) {
  if (ui.panel) return panelOrder(page).map((c) => '#l:' + c.N);
  const out = [];
  const walk = (ts) => {
    for (const t of ts || []) {
      if (t.Stop) out.push(t.Key);
      if (t.Kind === 'section' || ((t.Kind === 'fold' || t.Kind === 'ask') && isOpen(ui, t))) walk(t.Kids);
    }
  };
  if (page) { walk(page.Notices); walk(page.Things); }
  return out;
}

// placed keeps the cursor on the same line across a new page. When its
// line went, it goes to the nearest: as the page's own move if the
// person's act took the line away (going), else keeping what it came to
// (the server then finds the line rewritten, and refuses keys until the
// person moves).
export function placed(ui, page, now) {
  const keys = navKeys(page, ui);
  const going = ui.going;
  ui = { ...ui, going: '' };
  if (!keys.length) return { ...ui, cur: '' };
  if (ui.cur && keys.includes(ui.cur)) return { ...ui, lastIndex: keys.indexOf(ui.cur) };
  const had = ui.cur !== '';
  const i = had ? Math.min(ui.lastIndex || 0, keys.length - 1) : 0;
  const cur = keys[Math.max(i, 0)];
  const out = { ...ui, cur, lastIndex: i };
  if ((had && ui.cur === going) || !had) {
    // The person took the line away (hid it, took the comment back): the
    // page's own move. Or the page put the cursor on its first line.
    const t = index(page).things.get(cur);
    Object.assign(out, { how: had ? 'app' : 'start', at: now, sel: t ? t.Token : '' });
    if (!had) out.start = true;
  }
  return out;
}

// setCursor moves the cursor to key: by the person (keys, Tab, a click)
// or by the page (space, g).
export function setCursor(ui, page, key, how, now) {
  const t = index(page).things.get(key);
  return { ...ui, cur: key, how, at: now, sel: t ? t.Token : '', start: false, lastIndex: navKeys(page, ui).indexOf(key) };
}

export function move(ui, page, d, now) {
  const keys = navKeys(page, ui);
  if (!keys.length) return ui;
  let i = keys.indexOf(ui.cur);
  i = i < 0 ? 0 : Math.max(0, Math.min(keys.length - 1, i + d));
  return setCursor(ui, page, keys[i], 'person', now);
}

export function setOpen(ui, keys, open = true) {
  const o = { ...ui.open };
  for (const k of keys) o[k] = open;
  return { ...ui, open: o };
}

// toggle opens or closes line key (one that opens).
export function toggle(ui, page, key) {
  const t = index(page).things.get(key);
  if (!t || !t.CanOpen) return ui;
  return setOpen(ui, [key], !isOpen(ui, t));
}

// next is where space goes: the nearest marked line below the cursor
// (else the first), and the Folds to open on the way; null when none.
export function next(ui, page) {
  const order = [...index(page).things.keys()];
  const at = order.indexOf(ui.cur);
  const nx = (page && page.Next) || [];
  if (!nx.length) return null;
  return nx.find((x) => order.indexOf(x.Key) > at) || nx[0];
}

// The message on the bottom line. clear takes it off when page's rule
// (page.MsgClears, as Clears) says event ev does.
export const say = (ui, msg) => ({ ...ui, status: msg || '', statusSeq: ui.statusSeq + 1 });
export function clear(ui, page, ev) {
  const cl = (page && page.Clears) || ['key', 'editor', 'panel', 'pointer'];
  return cl.includes(ev) && ui.status ? { ...ui, status: '' } : ui;
}

export function openPanel(ui) {
  if (ui.panel) return ui;
  return { ...ui, panel: true, mode: 'comments', editor: null,
    reportCur: { cur: ui.cur, how: ui.how, at: ui.at, sel: ui.sel, start: ui.start, lastIndex: ui.lastIndex }, cur: '' };
}

export function closePanel(ui) {
  if (!ui.panel) return ui;
  const out = { ...ui, panel: false, mode: 'normal' };
  if (ui.editor && ui.editor.at && ui.editor.at.startsWith('#')) out.editor = null;
  if (ui.reportCur) Object.assign(out, ui.reportCur);
  out.reportCur = null;
  return out;
}

export function startGoto(ui, page) {
  if (ui.panel || !((page && page.JumpTo) || []).length) return ui;
  return { ...ui, mode: 'goto' };
}
export const endGoto = (ui) => ({ ...ui, mode: base(ui) });

// jumpOf is the place g goes to with letter k (null none).
export function jumpOf(page, k) {
  let found = null;
  const walk = (ts) => { for (const t of ts || []) { if (!found && t.JumpKey === k) found = t; walk(t.Kids); } };
  walk(page.Notices);
  walk(page.Things);
  return found;
}

export const openHelp = (ui) => ({ ...ui, mode: 'help', helpFrom: ui.mode });
export const closeHelp = (ui) => ({ ...ui, mode: ui.helpFrom === 'editor' && ui.editor ? 'editor' : base(ui) });
export const openImage = (ui, n) => ({ ...ui, mode: 'image', image: n, imageFrom: ui.mode });
export const closeImage = (ui) => ({ ...ui, mode: ui.imageFrom === 'editor' && ui.editor ? 'editor' : base(ui), image: 0 });

export const openEditor = (ui, ed) => ({ ...ui, mode: 'editor', editor: { text: '', ...ed } });
export const closeEditor = (ui) => ({ ...ui, mode: base(ui), editor: null });
export const editText = (ui, text) => (ui.editor ? { ...ui, editor: { ...ui.editor, text } } : ui);

export const setConn = (ui, conn, msg) => ({ ...ui, conn, connMsg: msg || '' });

// facts is how the cursor came to line t, for the server's guard.
export function facts(ui, t, now) {
  return { Start: ui.start, How: ui.how, Ago: Math.round(now - ui.at), Sel: ui.sel || t.Token, Open: isOpen(ui, t) };
}

// offerOf is the server's offer for the cursor's line, if it is for the
// line as it is now (null otherwise).
export function offerOf(ui) {
  const o = ui.offer;
  return o && o.key === ui.cur && o.sel === ui.sel ? o.offer : null;
}
