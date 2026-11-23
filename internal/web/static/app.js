// pitboard in a browser. The rules (what a line is, its marks, its signs,
// its words, what a key does on it and what to say when it does nothing)
// all come from the server (internal/page); this file only draws them,
// reports the facts of the cursor (how it came to its line, and when), and
// sends the person's keys.
// Report and comment text is data: it is only ever put in the page with
// textContent and created elements, never as HTML.

const $ = (id) => document.getElementById(id);
const el = (tag, cls, text) => {
  const e = document.createElement(tag);
  if (cls) e.className = cls;
  if (text != null) e.textContent = text;
  return e;
};

// ---- state ----

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

const S = {
  data: null,        // the page (GET /api/page)
  etag: '',
  open: new Map(),   // key -> open (what the person opened or closed)
  copen: new Map(),  // comment row key -> open
  // The cursor, and the facts of how it came there (the server decides
  // what they mean): start (the person has not moved it since the page put
  // it there), how (person / app), at (when), sel (the line's token then).
  cur: '',
  start: true,
  how: 'start',
  at: 0,
  sel: '',
  lastIndex: 0,
  going: '',         // the line the person's own act is taking away
  reportCur: null,   // the cursor on the report while the comment list is shown
  offer: null,       // the server's hint for the cursor's line: {key, sel, offer}
  hintSeq: 0,
  hintTimer: 0,
  showHidden: false,
  list: false,       // the comment list instead of the report
  editor: null,      // {kind, key, token, n, at, label}
  jumping: false,
  busy: false,
  composing: false,
  pending: false,    // a refresh waiting for the input method
  offline: false,
};
const words = (k) => (S.data && S.data.Words && S.data.Words[k]) || '';
const draftKey = (ed) => 'pitboard-draft:' + ed.kind + ':' + (ed.kind === 'edit' ? ed.n : ed.key);
const store = {
  get(k) { try { return sessionStorage.getItem(k); } catch { return null; } },
  set(k, v) { try { sessionStorage.setItem(k, v); } catch { /* the draft lives only on screen */ } },
  del(k) { try { sessionStorage.removeItem(k); } catch { /* nothing */ } },
};

// ---- talking to the server ----

class Offline extends Error {}

// request sends a request; it says the page is offline when the server
// cannot be reached or no longer takes it.
async function request(path, body, headers = {}) {
  const opt = { headers: { 'X-Pitboard-Tab': tabId, ...headers }, credentials: 'same-origin' };
  if (body !== undefined) {
    opt.method = 'POST';
    opt.headers['Content-Type'] = 'application/json';
    opt.body = JSON.stringify(body);
  }
  let r;
  try {
    r = await fetch(path, opt);
  } catch {
    setOffline(true);
    throw new Offline(words('offline'));
  }
  if (r.status === 403) {
    // The server was started again (a new key), or the cookie is gone.
    setOffline(true);
    throw new Offline(words('offline'));
  }
  if (!r.ok && r.status !== 304) throw new Error((await r.text()).trim() || r.statusText);
  return r;
}

async function api(path, body) {
  return (await request(path, body)).json();
}

// setOffline shows (or clears) that the page is not live: what it shows
// may be old, and nothing done on it reaches the board.
function setOffline(on) {
  if (S.offline === on) return;
  S.offline = on;
  const b = $('offline');
  b.textContent = on ? words('offline') : '';
  b.hidden = !on;
  document.body.classList.toggle('offline', on);
  syncPadding();
  // Keep trying quietly: the first page that comes back clears it.
  clearInterval(S.retryTimer);
  if (on) S.retryTimer = setInterval(() => { S.etag = ''; refresh(); }, 3000);
}

async function refresh() {
  if (S.composing) { S.pending = true; return; }
  let d;
  try {
    const r = await request('/api/page' + (S.showHidden ? '?hidden=1' : ''), undefined, S.etag ? { 'If-None-Match': S.etag } : {});
    if (r.status === 304) {
      setOffline(false);
      requestHint(); // the same page as drawn
      return;
    }
    d = await r.json();
    S.etag = r.headers.get('ETag') || '';
  } catch (e) {
    // The banner says it (Offline); a message on the status line stays.
    if (!(e instanceof Offline)) console.warn('pitboard: page', e);
    return;
  }
  setOffline(false);
  const first = !S.data;
  S.data = d;
  render();
  if (first) focusCur();
  if (d.Say) say(d.Say);
}

function say(msg) {
  const s = $('status');
  s.textContent = '';
  // A new text each time, so it is read out even when it repeats.
  requestAnimationFrame(() => { s.textContent = msg || ''; syncPadding(); });
}

// facts is how the cursor came to line t, for the server's guard.
function facts(t) {
  return { Start: S.start, How: S.how, Ago: Math.round(Date.now() - S.at), Sel: S.sel || t.Token, Open: isOpen(t) };
}

async function act(action, extra = {}) {
  if (S.busy) return null;
  let r;
  try {
    r = await api('/api/act', { Action: action, ShowHidden: S.showHidden, ...extra });
  } catch (e) {
    if (!(e instanceof Offline)) say('できませんでした: ' + e.message);
    return null;
  }
  if (r.Message) say(r.Message);
  return r;
}

// ---- walking the page ----

let things = new Map(); // key -> the line one can point at (Stop)
function index() {
  things = new Map();
  const walk = (ts) => { for (const t of ts || []) { if (t.Stop) things.set(t.Key, t); walk(t.Kids); } };
  if (S.data) { walk(S.data.Notices); walk(S.data.Things); }
}
const byKey = (k) => things.get(k);
const comments = () => (S.data && S.data.Comments) || [];
const commentOf = (n) => comments().find((c) => c.N === n);
const isOpen = (t) => (S.open.has(t.Key) ? S.open.get(t.Key) : !!t.Open);
const canWrite = () => S.data && S.data.CanWrite;
const plain = (spans) => (spans || []).map((s) => s.Text).join('');
const rowOf = (key) => rows.get(key);
const navOf = (key) => { const r = rowOf(key); return r && r.querySelector(':scope > .rowbtn'); };

// ---- drawing ----

function spans(parent, ss, inButton) {
  for (const s of ss || []) {
    let e;
    switch (s.Style) {
      case 'bold': e = el('strong'); break;
      case 'code': e = el('code'); break;
      case 'link':
        if (!inButton && /^https?:\/\//i.test(s.URL)) {
          e = el('a');
          e.href = s.URL;
          e.rel = 'noopener noreferrer';
          e.target = '_blank';
        } else {
          e = el('span', 'faint');
        }
        break;
      default: e = document.createDocumentFragment();
    }
    s.Text.split('\n').forEach((part, i) => {
      if (i > 0) e.appendChild(el('br'));
      e.appendChild(document.createTextNode(part));
    });
    parent.appendChild(e);
  }
  return parent;
}

function word(w, extra) {
  if (!w || !w.Text) return null;
  return el('span', w.Tone + (extra ? ' ' + extra : ''), w.Text);
}

function rightOf(t, open) {
  const r = t.Right || {};
  const box = el('span', 'right');
  if (r.Reaction) {
    const cls = r.Reaction === 'y' ? 'yes' : r.Reaction === 'n' ? 'no' : 'optkey';
    box.appendChild(el('span', cls, r.Reaction));
    if (t.Kind === 'ask' && r.OptionHead && !open) box.appendChild(el('span', 'faint ohead', r.OptionHead));
  }
  if (r.Input && r.Input.Text) {
    const w = el('span', '', '');
    if (r.Count > 0) w.appendChild(el('span', 'faint', r.Count + ' '));
    w.appendChild(word(r.Input));
    box.appendChild(w);
  }
  if (r.AtText) box.appendChild(el('span', 'faint', r.AtText));
  else if (r.Status && r.Status.Text) box.appendChild(word(r.Status));
  return box;
}

function markOf(t) {
  const m = el('span', 'dot');
  if (t.Mark === 'waiting' || t.Mark === 'new') {
    m.classList.add(t.Mark);
    m.textContent = '●';
    m.appendChild(el('span', 'sr', t.Mark === 'waiting' ? '反応待ち' : '未読'));
  }
  return m;
}

// Rows are kept by key between drawings: a row whose line says the same is
// not drawn again (only what it holds is put back in).
let rows = new Map();     // key -> row element now on the page
let cache = new Map();    // key -> {sig, box, kids}

// patch makes parent's children kids, moving only what is not already in
// place: rows kept from the last drawing stay where they are.
function patch(parent, kids) {
  const keep = new Set(kids);
  let cur = parent.firstChild;
  for (const k of kids) {
    while (cur && !keep.has(cur)) {
      const next = cur.nextSibling;
      parent.removeChild(cur);
      cur = next;
    }
    if (cur === k) { cur = cur.nextSibling; continue; }
    parent.insertBefore(k, cur);
  }
  while (cur) {
    const next = cur.nextSibling;
    parent.removeChild(cur);
    cur = next;
  }
}

function rowSig(t, open, extra) {
  const { Kids, ...own } = t;
  return JSON.stringify(own) + '|' + open + '|' + (S.jumping && t.JumpKey ? 'j' : '') + '|' + (extra || '');
}

// row draws a line the person can point at, and what it shows open.
function row(t, opts = {}) {
  const open = t.CanOpen && isOpen(t);
  const sig = rowSig(t, open, opts.heading ? 'h' : '');
  let c = cache.get(t.Key);
  if (!c || c.sig !== sig) {
    c = { sig, ...drawRow(t, open, opts) };
    cache.set(t.Key, c);
  }
  if (c.kids) {
    const kids = [];
    things2({ appendChild: (x) => { kids.push(x); return x; } }, t.Kids);
    patch(c.kids, kids);
  }
  rows.set(t.Key, c.box);
  return c.box;
}

function drawRow(t, open, opts) {
  const box = el('div', 'row');
  box.dataset.key = t.Key;
  if (t.Kind === 'notice') box.classList.add('notice');
  if (t.Kind === 'ask') box.classList.add('ask');
  if (t.Kind === 'link') box.classList.add('linkrow');
  if (t.State === 'done') box.classList.add('done');
  if (t.Hidden) box.classList.add('hidden');
  if (opts.heading && !t.Hidden) box.classList.add('heading');
  if (open) box.classList.add('open');
  // How deep in Folds: the row's cursor and mark stay in the outermost
  // gutter, its sign hangs left of its own text edge.
  if (t.Depth) box.style.setProperty('--ind', String(t.Depth));
  const b = el('button', 'rowbtn');
  b.type = 'button';
  b.dataset.nav = t.Key;
  if (t.CanOpen) b.setAttribute('aria-expanded', String(open));
  b.appendChild(el('span', 'gut'));
  b.appendChild(markOf(t));
  b.appendChild(el('span', 'sign', open ? t.OpenSign : t.Sign));
  const title = el('span', 'title');
  if (S.jumping && t.JumpKey) title.appendChild(el('span', 'jumpkey', t.JumpKey));
  if (t.Jump) box.dataset.jump = String(t.Jump);
  spans(title, t.Title, true);
  if (t.Hidden) title.appendChild(el('span', 'sr', ' (隠した行)'));
  b.appendChild(title);
  b.appendChild(rightOf(t, open));
  box.appendChild(b);
  if (t.Kind === 'link') {
    const tg = el('div', 'target');
    if (t.URL && /^https?:\/\//i.test(t.URL)) {
      const a = el('a', '', t.URL);
      a.href = t.URL;
      a.rel = 'noopener noreferrer';
      a.target = '_blank';
      tg.appendChild(a);
    } else {
      tg.textContent = t.URL || t.Path;
    }
    box.appendChild(tg);
  }
  if (t.Kind === 'list' && t.Subs && t.Subs.length) {
    const s = el('div', 'subs body');
    for (const x of t.Subs) s.appendChild(block(x));
    box.appendChild(s);
  }
  let kids = null;
  if (open) {
    if (t.Kind === 'fold') {
      box.appendChild(commentsOn(t, 'body'));
      kids = el('div', 'kids');
      box.appendChild(kids);
    } else {
      box.appendChild(contents(t));
      if (t.Kind === 'ask') {
        for (const o of t.Kids || []) {
          const line = el('div', 'option');
          const k = el('span', 'okey ' + (o.Chosen ? 'chosen' : 'faint'), (o.Chosen ? '● ' : '') + o.OptionKey);
          if (o.Chosen) k.appendChild(el('span', 'sr', ' (選んだ答え)'));
          line.appendChild(k);
          line.appendChild(spans(el('span', 'otext'), o.Title));
          box.appendChild(line);
        }
      }
    }
  }
  return { box, kids };
}

function contents(t) {
  const box = el('div', 'body');
  if (t.Note) {
    const p = el('div', 'para');
    p.appendChild(el('span', 'faint', '理由: '));
    spans(p, [{ Text: t.Note, Style: 'plain' }]);
    box.appendChild(p);
  }
  for (const b of t.Body || []) box.appendChild(block(b));
  box.appendChild(commentsOn(t));
  return box;
}

function commentsOn(t, cls) {
  const box = el('div', cls || '');
  for (const c of t.Comments || []) {
    const d = el('div', 'comment');
    const head = el('div', 'list');
    head.appendChild(el('span', 'lm', '✎'));
    head.appendChild(commentText(c.Text));
    head.appendChild(word(c.Word, 'end'));
    d.appendChild(head);
    if (c.TitleThen) d.appendChild(el('div', 'faint', '(書いたときは: ' + c.TitleThen + ')'));
    if (c.Settled) {
      const p = el('div');
      p.appendChild(el('span', 'faint', c.SettledHead));
      p.appendChild(document.createTextNode(c.Settled));
      d.appendChild(p);
    }
    d.appendChild(thumbs(c.Images));
    box.appendChild(d);
  }
  return box;
}

function commentText(text) {
  const span = el('span', 'ctext');
  const re = /\[Image #[0-9]+\]/g;
  let last = 0;
  for (const m of text.matchAll(re)) {
    span.appendChild(document.createTextNode(text.slice(last, m.index)));
    span.appendChild(el('span', 'imgref', m[0]));
    last = m.index + m[0].length;
  }
  span.appendChild(document.createTextNode(text.slice(last)));
  return span;
}

function thumbs(ns) {
  const box = el('div', 'thumbs');
  for (const n of ns || []) {
    const b = el('button');
    b.type = 'button';
    b.setAttribute('aria-label', `画像 ${n} を大きく見る`);
    const img = el('img');
    img.src = '/api/image/' + n;
    img.alt = `[Image #${n}]`;
    b.appendChild(img);
    b.addEventListener('click', (e) => { e.stopPropagation(); showImage(n, b); });
    box.appendChild(b);
  }
  return box;
}

// block draws a line nobody points at (text, list lines, code).
function block(t) {
  switch (t.Kind) {
    case 'code': {
      const pre = el('pre');
      const code = el('code');
      for (const l of t.Code || []) {
        let cls = '';
        if (t.Lang === 'diff') cls = l.startsWith('+') ? 'move' : l.startsWith('-') ? 'no' : l.startsWith('@@') ? 'optkey' : '';
        code.appendChild(el('span', cls, l + '\n'));
      }
      pre.appendChild(code);
      return pre;
    }
    case 'list': {
      const d = el('div');
      const l = el('div', 'list' + (t.Gap ? ' gap' : ''));
      l.appendChild(el('span', 'lm', t.Num || (t.Level === 1 ? '-' : '·')));
      l.appendChild(spans(el('span'), t.Title));
      d.appendChild(l);
      if (t.Subs && t.Subs.length) {
        const s = el('div', 'subs');
        for (const x of t.Subs) s.appendChild(block(x));
        d.appendChild(s);
      }
      return d;
    }
    default:
      return spans(el('div', 'para' + (t.Head ? ' head' : '') + (t.Gap ? ' gap' : '')), t.Title);
  }
}

function things2(parent, ts, top) {
  for (const t of ts || []) {
    switch (t.Kind) {
      case 'title':
        parent.appendChild(spans(el('h1'), t.Title));
        break;
      case 'section': {
        const h = el('h2', 'section');
        if (t.Jump) h.dataset.jump = String(t.Jump);
        if (S.jumping && t.JumpKey) h.appendChild(el('span', 'jumpkey', t.JumpKey));
        spans(h, t.Title);
        parent.appendChild(h);
        if (t.None) parent.appendChild(el('p', 'none faint', t.None));
        things2(parent, t.Kids);
        break;
      }
      default:
        if (t.Stop) parent.appendChild(row(t, { heading: top && t.Kind === 'fold' }));
        else parent.appendChild(block(t));
    }
  }
}

// ---- the bar at the bottom ----

// The bar is the terminal's hint line, each key a button: the keys of the
// cursor's line (what the server offers for it: a key it would refuse is
// never shown), then the page's own (Bar).

function keyButton(key, label, fn, aria) {
  const b = el('button');
  b.type = 'button';
  b.append(el('kbd', '', key));
  if (label) b.append(' ' + label);
  b.dataset.key = key;
  b.setAttribute('aria-keyshortcuts', key === 'enter' ? 'Enter' : key === 'space' ? 'Space' : key);
  if (aria) b.setAttribute('aria-label', aria);
  b.addEventListener('click', (e) => { e.stopPropagation(); fn(); });
  return b;
}

// drawBar draws the bar for the cursor's line as it is now.
function drawBar() {
  const box = $('hints');
  const had = box.contains(document.activeElement) ? document.activeElement.dataset.key : null;
  const items = [];
  const d = S.data;
  if (!d) return;
  const c = S.cur.startsWith('#') ? commentOf(Number(S.cur.slice(3))) : null;
  const t = byKey(S.cur);
  if (c) {
    const open = !!S.copen.get(S.cur);
    items.push(keyButton('enter', open ? '閉じる' : '開く', () => toggle(S.cur)));
    if (canWrite()) {
      items.push(keyButton('e', '直す', () => editComment(c, S.cur)));
      items.push(keyButton('x', '取り消す', () => retract(c)));
    }
  } else if (t && !S.list) {
    const o = S.offer && S.offer.key === S.cur && S.offer.sel === S.sel ? S.offer.offer : null;
    if (o && o.Message) items.push(el('span', 'faint', o.Message));
    for (const h of (o && o.Hints) || []) {
      switch (h.Do) {
        case 'open': case 'close': items.push(keyButton('enter', h.Word, () => toggle(t.Key))); break;
        case 'wait': items.push(el('span', 'faint', h.Key + ' ' + h.Word)); break;
        case 'answer':
          for (const k of o.Options || []) {
            const opt = (t.Kids || []).find((x) => x.OptionKey === k);
            items.push(keyButton(k, '', () => press(k), h.Word + ': ' + (opt ? plain(opt.Title) : k)));
          }
          items.push(el('span', '', h.Word));
          break;
        default: items.push(keyButton(h.Key, h.Word, () => press(h.Key)));
      }
    }
  }
  for (const h of d.More || []) {
    if (S.list && h.Do !== 'undo') continue;
    items.push(keyButton(h.Key, h.Word, () => barAct(h.Do)));
  }
  for (const h of d.Tail || []) {
    const b = keyButton(h.Key, h.Word, () => barAct(h.Do));
    if (h.Do === 'send') { b.classList.add('need'); b.disabled = S.busy; }
    items.push(b);
  }
  box.replaceChildren(...items);
  $('b-list').textContent = S.list ? '報告へ' : 'コメント一覧';
  if (had != null) {
    const b = box.querySelector(`[data-key="${CSS.escape(had)}"]`) || navOf(S.cur);
    if (b) b.focus({ preventScroll: true });
  }
}

// barAct is what a key of the bar that is not the line's own does.
function barAct(what) {
  switch (what) {
    case 'undo': undo(); break;
    case 'next': jumpNext(); break;
    case 'jump': startJumping(); break;
    case 'send': send(); break;
    case 'write': openEditor({ kind: 'comment', key: '', at: S.list ? '' : S.cur, label: '宛先なしのコメント' }); break;
    case 'keys': openHelp(); break;
  }
}

function startJumping() {
  if (S.list || !(S.data.JumpTo || []).length) return;
  S.jumping = true;
  say(words('jumping'));
  render();
}

// requestHint asks the server what works on the cursor's line now.
async function requestHint() {
  clearTimeout(S.hintTimer);
  const t = byKey(S.cur);
  if (!t || S.list) { drawBar(); return; }
  const seq = ++S.hintSeq;
  const key = S.cur;
  const sel = S.sel;
  let r;
  try {
    r = await api('/api/act', { Action: 'hint', Target: t.Key, Seen: t.Token, Cur: facts(t) });
  } catch { return; }
  if (seq !== S.hintSeq || !r || !r.Offer) return;
  S.offer = { key, sel, offer: r.Offer };
  drawBar();
  if (r.Offer.Recheck > 0) S.hintTimer = setTimeout(requestHint, r.Offer.Recheck + 20);
}

// ---- drawing the page ----

function render() {
  const d = S.data;
  if (!d) return;
  navKeys = null;
  index();
  // Keep what the person is looking at where it is on screen.
  const anchor = rowOf(S.cur) || document.querySelector('.editor');
  const before = anchor && anchor.isConnected ? anchor.getBoundingClientRect().top : null;
  const ae = document.activeElement;
  const focusRow = ae && ae !== document.body && ae.closest && !!ae.closest('.row');
  const ed = saveEditor();

  // top right
  const sum = $('summary');
  sum.replaceChildren();
  for (const w of d.Summary || []) sum.appendChild(word(w));
  if (d.Hidden > 0) sum.appendChild(el('span', 'faint', `隠した行 ${d.Hidden}` + (S.showHidden ? ' (表示中)' : '')));

  const nb = $('notices');
  const board = $('board');
  const pane = $('pane');
  rows = new Map();
  const nbKids = [];
  const boardKids = [];
  const paneKids = [];
  const sink = (arr) => ({ appendChild: (x) => { arr.push(x); return x; } });
  if (S.list) {
    renderList(sink(boardKids));
  } else {
    for (const t of d.Notices || []) nbKids.push(row(t));
    if (d.Message) boardKids.push(el('p', 'msg', d.Message));
    if (d.Err) boardKids.push(el('p', 'err', '読めませんでした: ' + d.Err));
    if (d.ReadErr) boardKids.push(el('p', 'err', d.ReadErr));
    if (d.Problems && d.Problems.length) {
      const p = el('div', 'err');
      p.appendChild(el('strong', '', d.ProblemHead));
      for (const l of d.Problems) p.appendChild(el('div', '', l));
      boardKids.push(p);
    }
    things2(sink(boardKids), d.Things, true);
    renderPane(sink(paneKids));
  }
  patch(nb, nbKids);
  patch(board, boardKids);
  patch(pane, paneKids);
  // Rows of lines no longer there are forgotten.
  for (const k of cache.keys()) if (!rows.has(k)) cache.delete(k);

  checkCursor();
  markCursor(null);
  placeEditor();
  restoreEditor(ed);
  requestHint();
  drawBar();

  const now = rowOf(S.cur) || document.querySelector('.editor');
  if (now && before != null) window.scrollBy(0, now.getBoundingClientRect().top - before);
  if (focusRow && !ed.focused) {
    const b = navOf(S.cur);
    if (b) b.focus({ preventScroll: true });
  }
}

function placeEditor() {
  if (!S.editor) return;
  const box = editorBox();
  const r = S.editor.at && rowOf(S.editor.at);
  if (r) {
    (r.querySelector(':scope > .target') || r.querySelector(':scope > .rowbtn')).after(box);
  } else {
    $('board').appendChild(box);
  }
}

// commentRow draws one comment as a line of the comment area or list.
function commentRow(c, key, numbered) {
  const open = !!S.copen.get(key);
  const box = el('div', 'row' + (open ? ' open' : '') + (c.Finished ? ' done' : ''));
  box.dataset.key = key;
  const b = el('button', 'rowbtn');
  b.type = 'button';
  b.dataset.nav = key;
  b.setAttribute('aria-expanded', String(open));
  b.appendChild(el('span', 'gut'));
  b.appendChild(el('span', 'dot'));
  b.appendChild(el('span', 'sign', open ? '▾' : '▸'));
  const title = el('span', 'title');
  let text = c.Text;
  if (numbered) {
    title.appendChild(el('span', 'faint', c.N + '  '));
    text = text.split('\n')[0];
  } else {
    text = text.split(/\s+/).join(' ');
  }
  title.appendChild(commentText(text));
  b.appendChild(title);
  const r = el('span', 'right');
  r.appendChild(word(c.Word));
  b.appendChild(r);
  box.appendChild(b);
  if (open) {
    const d = el('div', 'cdetail');
    if (c.About) {
      const p = el('div');
      p.appendChild(el('span', 'faint', '行: '));
      p.appendChild(document.createTextNode(c.About));
      d.appendChild(p);
    }
    d.appendChild(commentText(c.Text));
    const s = el('div');
    if (c.Settled) {
      s.appendChild(el('span', 'faint', c.SettledHead));
      s.appendChild(document.createTextNode(c.Settled));
    } else {
      s.className = 'faint';
      s.textContent = c.Note;
    }
    d.appendChild(s);
    d.appendChild(thumbs(c.Images));
    box.appendChild(d);
  }
  rows.set(key, box);
  return box;
}

function renderPane(pane) {
  const open = comments().filter((c) => !c.Finished);
  if (!open.length) return;
  pane.appendChild(el('h2', '', 'コメント'));
  for (const c of open) pane.appendChild(commentRow(c, '#c:' + c.N, false));
}

function renderList(board) {
  board.appendChild(el('h2', 'listhead', 'コメント'));
  const cs = [...comments()].sort((a, b) => b.N - a.N);
  if (!cs.length) board.appendChild(el('p', 'msg', 'コメントはまだありません。m で書けます'));
  for (const c of cs) board.appendChild(commentRow(c, '#l:' + c.N, true));
}

// ---- the cursor ----

// navKeysOf is the keys of the lines one can point at, in the order shown
// (kept until the page is drawn again).
let navKeys = null;
function navKeysOf() {
  if (!navKeys) navKeys = walkNav();
  return navKeys;
}
function walkNav() {
  const out = [];
  if (S.list) {
    for (const c of [...comments()].sort((a, b) => b.N - a.N)) out.push('#l:' + c.N);
    return out;
  }
  const walk = (ts) => {
    for (const t of ts || []) {
      if (t.Stop) out.push(t.Key);
      if (t.Kind === 'section' || ((t.Kind === 'fold' || t.Kind === 'ask') && isOpen(t))) walk(t.Kids);
    }
  };
  if (S.data) { walk(S.data.Notices); walk(S.data.Things); }
  for (const c of comments()) if (!c.Finished) out.push('#c:' + c.N);
  return out;
}

// checkCursor keeps the cursor on the same line across a drawing. When its
// line went, it goes to the nearest: as the page's own move if the
// person's act took the line away, else keeping what it came to (the
// server then finds the line rewritten, and refuses keys until the person
// moves).
function checkCursor() {
  const keys = navKeysOf();
  const going = S.going;
  S.going = '';
  if (!keys.length) { S.cur = ''; return; }
  if (S.cur && keys.includes(S.cur)) {
    S.lastIndex = keys.indexOf(S.cur);
    return;
  }
  const had = S.cur !== '';
  const i = had ? Math.min(S.lastIndex || 0, keys.length - 1) : 0;
  const expected = had && S.cur === going;
  S.cur = keys[Math.max(i, 0)];
  S.lastIndex = i;
  if (expected || !had) {
    // The person took the line away (hid it, took the comment back): the
    // page's own move. Or the page put the cursor on its first line.
    S.how = had ? 'app' : 'start';
    S.at = Date.now();
    const t = byKey(S.cur);
    S.sel = t ? t.Token : '';
    if (!had) S.start = true;
  }
}

let curRow = null;
// markCursor shows the cursor on its row (and only there): two rows change.
function markCursor(prev) {
  const now = S.list && !S.cur.startsWith('#') ? null : rowOf(S.cur);
  for (const r of [curRow, prev]) {
    if (r && r !== now) {
      r.classList.remove('cur');
      const b = r.querySelector(':scope > .rowbtn');
      if (b) b.removeAttribute('aria-current');
    }
  }
  if (now) {
    now.classList.add('cur');
    const b = now.querySelector(':scope > .rowbtn');
    if (b) b.setAttribute('aria-current', 'true');
  }
  curRow = now;
}

// setCursor moves the cursor to key: by the person (keys, Tab, click) or
// by the page (space, g). Nothing else is drawn again.
function setCursor(key, how, focus = true) {
  const prev = rowOf(S.cur);
  const t = byKey(key);
  S.cur = key;
  S.how = how;
  S.at = Date.now();
  S.sel = t ? t.Token : '';
  S.start = false;
  S.lastIndex = navKeysOf().indexOf(key);
  markCursor(prev);
  drawBar();
  requestHint();
  if (focus) {
    const b = navOf(key);
    if (b && document.activeElement !== b) b.focus({ preventScroll: true });
    if (b) b.scrollIntoView({ block: 'nearest' });
  }
}

function move(d) {
  const keys = navKeysOf();
  if (!keys.length) return;
  let i = keys.indexOf(S.cur);
  i = i < 0 ? 0 : Math.max(0, Math.min(keys.length - 1, i + d));
  setCursor(keys[i], 'person');
}

function toggle(key) {
  if (key.startsWith('#')) {
    S.copen.set(key, !S.copen.get(key));
  } else {
    const t = byKey(key);
    if (!t || !t.CanOpen) return;
    S.open.set(key, !isOpen(t));
  }
  render();
  const b = navOf(key);
  if (b) b.focus({ preventScroll: true });
}

// ---- actions ----

// focusCur puts the focus on the cursor's row after an action (or a button
// of it): the focus is the cursor.
function focusCur() {
  if (S.editor || !$('help').hidden || !$('viewer').hidden) return;
  const b = navOf(S.cur);
  if (b && document.activeElement !== b) b.focus({ preventScroll: true });
}

// press sends a key pressed on the cursor's line (or its button): the
// server says what it does there, or why not.
async function press(k) {
  const t = byKey(S.cur);
  if (!t) return;
  const r = await act('key', { Key: k, Target: t.Key, Seen: t.Token, Cur: facts(t) });
  if (!r) return;
  switch (r.Do) {
    case 'open': S.open.set(t.Key, true); render(); return;
    case 'comment': openEditor({ kind: 'comment', key: t.Key, token: t.Token, at: t.Key, label: '「' + plain(t.Title) + '」にコメント' }); return;
    case 'hide': if (r.OK) S.going = t.Key; break;
  }
  if (r.OK || r.Changed) await refresh();
  else requestHint();
  focusCur();
}

async function retract(c) {
  const r = await act('retract', { N: c.N, Cur: { How: S.how, Ago: Math.round(Date.now() - S.at) } });
  if (r && r.OK) { S.going = S.cur; await refresh(); }
  focusCur();
}
async function undo() {
  S.going = S.cur;
  const r = await act('undo');
  if (r && r.OK) await refresh();
  S.going = '';
  focusCur();
}
async function readAll() {
  if (!canWrite()) return;
  const r = await act('readall');
  if (r && r.OK) await refresh();
}

async function send() {
  if (S.busy || !canWrite()) return;
  S.busy = true;
  document.body.setAttribute('aria-busy', 'true');
  drawBar();
  say('送っています…');
  try {
    const r = await api('/api/send', {});
    say(r.Message);
  } catch (e) {
    if (!(e instanceof Offline)) say('送れませんでした: ' + e.message);
  } finally {
    S.busy = false;
    document.body.removeAttribute('aria-busy');
    await refresh();
    drawBar();
  }
}

// ---- the editor ----

let edBox = null;
let edArea = null;
let edThumbs = null;

function editorBox() {
  if (!edBox) {
    edBox = el('div', 'editor');
    const lab = el('label');
    lab.htmlFor = 'editor-text';
    edArea = el('textarea');
    edArea.id = 'editor-text';
    edArea.rows = 3;
    edThumbs = el('div', 'thumbs');
    const hint = el('div', 'hint', 'enter 置く (s で送るまで AI には届かない) · shift+enter 改行 · esc やめる (書いたものは残る) · 画像は貼り付け・ドロップ');
    hint.id = 'editor-hint';
    edArea.setAttribute('aria-describedby', 'editor-hint');
    edBox.append(lab, edArea, edThumbs, hint);
    edArea.addEventListener('keydown', editorKey);
    edArea.addEventListener('input', () => { if (S.editor) store.set(draftKey(S.editor), edArea.value); drawThumbs(); });
    edArea.addEventListener('compositionstart', () => { S.composing = true; });
    edArea.addEventListener('compositionend', () => {
      S.composing = false;
      if (S.pending) { S.pending = false; refresh(); }
    });
    edArea.addEventListener('paste', (e) => {
      const files = [...(e.clipboardData ? e.clipboardData.files : [])].filter((f) => f.type.startsWith('image/'));
      if (files.length) { e.preventDefault(); files.forEach(upload); }
    });
    edArea.addEventListener('dragover', (e) => { e.preventDefault(); });
    edArea.addEventListener('drop', (e) => {
      const files = [...(e.dataTransfer ? e.dataTransfer.files : [])].filter((f) => f.type.startsWith('image/'));
      if (files.length) { e.preventDefault(); files.forEach(upload); }
    });
  }
  edBox.querySelector('label').textContent = S.editor ? S.editor.label : '';
  return edBox;
}

function saveEditor() {
  if (!edArea || !edBox.isConnected) return { focused: false };
  return { focused: document.activeElement === edArea, start: edArea.selectionStart, end: edArea.selectionEnd };
}
function restoreEditor(ed) {
  if (!edArea || !edBox.isConnected || !ed.focused) return;
  if (document.activeElement !== edArea) edArea.focus({ preventScroll: true });
  edArea.setSelectionRange(ed.start, ed.end);
}

function openEditor(ed) {
  if (!canWrite()) return;
  S.editor = ed;
  editorBox();
  edArea.value = store.get(draftKey(ed)) ?? (ed.text || '');
  placeEditor();
  drawThumbs();
  edArea.focus();
  edArea.setSelectionRange(edArea.value.length, edArea.value.length);
  edBox.scrollIntoView({ block: 'nearest' });
}

function editComment(c, key) {
  openEditor({ kind: 'edit', n: c.N, at: key, text: c.Text, label: `コメント ${c.N} を直す` });
}

function closeEditor() {
  const at = S.editor && S.editor.at;
  S.editor = null;
  if (edBox) edBox.remove();
  const b = (at && navOf(at)) || navOf(S.cur);
  if (b) b.focus({ preventScroll: true }); else $('board').focus({ preventScroll: true });
}

async function place() {
  const ed = S.editor;
  const text = edArea.value;
  let r;
  try {
    if (ed.kind === 'edit') r = await api('/api/act', { Action: 'edit', N: ed.n, Text: text });
    else r = await api('/api/act', { Action: 'comment', Target: ed.key || '', Seen: ed.token || '', Text: text });
  } catch (e) {
    if (!(e instanceof Offline)) say('置けませんでした: ' + e.message);
    return;
  }
  say(r.Message);
  if (!r.OK) return;
  store.del(draftKey(ed));
  edArea.value = '';
  closeEditor();
  await refresh();
}

function editorKey(e) {
  if (e.isComposing || e.keyCode === 229) return; // the input method's enter
  if (e.key === 'Escape') { e.preventDefault(); closeEditor(); return; }
  if (e.key === 'Enter' && !e.shiftKey && !e.altKey && !e.ctrlKey && !e.metaKey) {
    e.preventDefault();
    place();
  }
}

function upload(file) {
  if (file.size > S.data.ImageMax) {
    say(words('imageTooLarge'));
    return;
  }
  const rd = new FileReader();
  rd.onload = async () => {
    const data = String(rd.result).replace(/^data:[^,]*,/, '');
    let r;
    try {
      r = await api('/api/image', { Data: data });
    } catch (e) {
      if (!(e instanceof Offline)) say('貼れませんでした: ' + e.message);
      return;
    }
    say(r.Message);
    if (!r.OK) return;
    const ref = `[Image #${r.Image}]`;
    const s = edArea.selectionStart;
    edArea.setRangeText(ref, s, edArea.selectionEnd, 'end');
    edArea.dispatchEvent(new Event('input'));
    edArea.focus();
  };
  rd.readAsDataURL(file);
}

function drawThumbs() {
  if (!edThumbs) return;
  const ns = [...edArea.value.matchAll(/\[Image #([0-9]+)\]/g)].map((m) => Number(m[1]));
  edThumbs.replaceWith(edThumbs = thumbs([...new Set(ns)]));
}

// ---- dialogs ----

// A dialog keeps the focus inside it while open, and gives it back to
// where it was when it closes.
function trapFocus(e, dialog) {
  if (e.key !== 'Tab') return;
  const fs = [...dialog.querySelectorAll('button, a[href], textarea, [tabindex]:not([tabindex="-1"])')].filter((x) => !x.disabled);
  if (!fs.length) { e.preventDefault(); return; }
  const i = fs.indexOf(document.activeElement);
  e.preventDefault();
  fs[(i + (e.shiftKey ? -1 : 1) + fs.length) % fs.length].focus();
}

let viewerReturn = null;
function showImage(n, from) {
  viewerReturn = from;
  $('viewer-img').src = '/api/image/' + n;
  $('viewer-img').alt = `[Image #${n}]`;
  $('viewer').hidden = false;
  $('viewer-close').focus();
}
function closeImage() {
  $('viewer').hidden = true;
  $('viewer-img').removeAttribute('src');
  if (viewerReturn && viewerReturn.isConnected) viewerReturn.focus();
}

const HELP = [
  ['j k ↑↓', '上下の行へ (日本語入力のままでも全角の ｊ ｋ が効く)。Tab やクリックで行に移っても同じ'],
  ['enter', '開く・閉じる (行をクリックしても同じ)'],
  ['space', 'カーソルより下で、いちばん近い印のある行 (黄の ●・人待ちの項目・緑の ●) へ。下に無ければ上から。折りたたみは開く'],
  ['g', '節へ飛ぶ: お知らせ・節・いちばん外の折りたたみに a b c … が付く。その字でそこへ。esc でやめる'],
  ['y', 'よし (項目・リンク・お知らせ)'],
  ['n', 'だめ (項目・リンク・お知らせ)'],
  ['a b …', '問いに答える (問いを開いているときだけ。閉じていれば開く)'],
  ['r', '読んだ: 緑の ● (未読) を消す'],
  ['A', 'いま報告にある行をすべて読んだことにする'],
  ['c', 'この行にコメントを書く'],
  ['m', '宛先なしでコメントを書く'],
  ['e', 'コメントを直す (コメントの欄と一覧で)'],
  ['x', 'この行を隠す (隠した行の上では表示に戻す)。コメントの上では、そのコメントを取り消す'],
  ['H', '隠した行を表示する / しない'],
  ['u', '直前の操作を取り消す (このタブでした操作を、新しい順に 1 つずつ)'],
  ['s', 'AI に送る: 未送信のものをまとめて送る'],
  ['?', 'このキーの一覧'],
  ['', '端末の tab (コメントの一覧) は、ブラウザでは「コメント一覧」のボタン。tab キーはボタンと行の間を動く'],
  ['書いているとき', null],
  ['enter', 'pitboard に置く (s で送るまで AI には届かない)'],
  ['shift+enter', '改行'],
  ['esc', 'やめる (書いたものはこのタブに残り、次に開くと戻る)'],
  ['画像', '貼り付け・ドロップで [Image #N] が入る'],
];

function drawHelp() {
  const dl = $('help-keys');
  dl.replaceChildren();
  for (const [k, d] of HELP) {
    if (d === null) { dl.appendChild(el('h3', '', k)); continue; }
    dl.append(el('dt', '', k), el('dd', '', d));
  }
}
let helpReturn = null;
function openHelp() { helpReturn = document.activeElement; $('help').hidden = false; $('help-close').focus(); }
function closeHelp() { $('help').hidden = true; if (helpReturn && helpReturn.isConnected) helpReturn.focus(); }

// ---- going places ----

// goTo goes to place i of g: its line (a Section's first), its heading at
// the top of the screen.
function goTo(i) {
  const key = S.data.JumpTo[i];
  if (key) setCursor(key, 'app');
  const h = document.querySelector(`[data-jump="${i + 1}"]`);
  if (h) h.scrollIntoView({ block: 'start' });
}

function jumpNext() {
  const d = S.data;
  const order = [...things.keys()];
  const at = order.indexOf(S.cur);
  const next = d.Next || [];
  if (!next.length) { say(words('noMarked')); return; }
  const pos = (k) => order.indexOf(k);
  const target = next.find((x) => pos(x.Key) > at) || next[0];
  let opened = false;
  for (const f of target.Folds || []) if (!S.open.get(f)) { S.open.set(f, true); opened = true; }
  if (opened) render();
  setCursor(target.Key, 'app');
}

// imeFree makes keys work with a Japanese input method left on: full-width
// letters and symbols become their ASCII keys.
function imeFree(k) {
  if (k.length === 1) {
    const c = k.charCodeAt(0);
    if (c >= 0xff01 && c <= 0xff5e) return String.fromCharCode(c - 0xfee0);
    if (c === 0x3000) return ' ';
  }
  return k;
}

function toggleList() {
  S.list = !S.list;
  if (S.list) {
    S.reportCur = { cur: S.cur, how: S.how, at: S.at, sel: S.sel, start: S.start };
    S.cur = '';
  } else if (S.reportCur) {
    // Back to the report: the cursor is where it was, as it was.
    Object.assign(S, S.reportCur);
  }
  S.editor = null;
  if (edBox) edBox.remove();
  render();
  const b = navOf(S.cur);
  if (b) b.focus({ preventScroll: true }); else $('board').focus({ preventScroll: true });
}

function onKey(e) {
  if (!$('viewer').hidden) {
    if (e.key === 'Escape') { e.preventDefault(); closeImage(); }
    trapFocus(e, $('viewer'));
    return;
  }
  if (!$('help').hidden) {
    if (e.key === 'Escape' || e.key === '?' || e.key === 'q') { e.preventDefault(); closeHelp(); }
    trapFocus(e, $('help'));
    return;
  }
  const tg = e.target;
  if (tg && (tg.tagName === 'TEXTAREA' || tg.tagName === 'INPUT' || tg.isContentEditable)) return;
  if (e.ctrlKey || e.metaKey || e.altKey || e.isComposing) return;
  if (!S.data) return;
  const k = imeFree(e.key);
  const onButton = tg && tg.tagName === 'BUTTON' && !tg.dataset.nav;
  if (S.jumping) {
    e.preventDefault();
    const t = findJump(k);
    S.jumping = false;
    say('');
    render();
    if (t) goTo(t.Jump - 1);
    return;
  }
  const c = S.cur.startsWith('#') ? commentOf(Number(S.cur.slice(3))) : null;
  const done = () => e.preventDefault();
  switch (k) {
    case 'j': case 'ArrowDown': done(); move(1); return;
    case 'k': case 'ArrowUp': done(); move(-1); return;
    case 'Enter':
      if (onButton || (tg && tg.dataset && tg.dataset.nav)) return; // the button's own
      done(); if (S.cur) toggle(S.cur); return;
    case ' ':
      if (onButton) return;
      done(); if (!S.list) jumpNext(); return;
    case '?': done(); openHelp(); return;
    case 'u': done(); undo(); return;
    case 's': done(); send(); return;
    case 'm': done(); openEditor({ kind: 'comment', key: '', at: S.list ? '' : S.cur, label: '宛先なしのコメント' }); return;
    case 'H': done(); S.showHidden = !S.showHidden; S.going = S.cur; S.etag = ''; refresh(); return;
    case 'A': done(); readAll(); return;
    case 'g':
      if (S.list || !(S.data.JumpTo || []).length) return;
      done(); startJumping(); return;
    case 'Escape': return;
  }
  if (c) {
    if (k === 'e') { done(); editComment(c, S.cur); }
    if (k === 'x') { done(); retract(c); }
    return;
  }
  if (S.list || !byKey(S.cur) || k.length !== 1 || !/[a-zA-Z]/.test(k)) return;
  // Any other letter goes to the server's guard: it says what it does on
  // this line (or why it does nothing).
  done();
  press(k);
}

// findJump is the place g goes to with letter k (null none).
function findJump(k) {
  let found = null;
  const walk = (ts) => { for (const t of ts || []) { if (!found && t.JumpKey === k) found = t; walk(t.Kids); } };
  walk(S.data.Notices);
  walk(S.data.Things);
  return found;
}

// ---- start ----

// syncPadding keeps the end of the page above the bar at the bottom,
// however tall the status above it is.
function syncPadding() {
  const h = $('foot').offsetHeight;
  document.body.style.paddingBottom = (h + 24) + 'px';
  document.documentElement.style.scrollPaddingBottom = (h + 8) + 'px';
}

let es = null;
let retry = 2000;
function connect() {
  es = new EventSource('/api/events');
  es.addEventListener('changed', () => refresh());
  es.addEventListener('open', () => { retry = 2000; refresh(); });
  es.addEventListener('error', () => {
    // The server is gone, or no longer takes this page (started again):
    // say so until it does, and keep trying quietly.
    setOffline(true);
    if (es.readyState === EventSource.CLOSED) {
      es.close();
      setTimeout(connect, retry);
      retry = Math.min(retry * 2, 30000);
    }
  });
}

function start() {
  drawHelp();
  document.addEventListener('keydown', onKey);
  // Focus is the cursor: a row reached by Tab or a click is the cursor's.
  document.addEventListener('focusin', (e) => {
    const b = e.target;
    if (!b || !b.dataset || !b.dataset.nav || b.dataset.nav === S.cur) return;
    setCursor(b.dataset.nav, 'person', false);
  });
  for (const id of ['board', 'notices', 'pane']) {
    $(id).addEventListener('click', (e) => {
      const b = e.target.closest && e.target.closest('.rowbtn');
      if (!b || e.target.closest('a')) return;
      const key = b.dataset.nav;
      if (key !== S.cur) setCursor(key, 'person', false);
      const t = byKey(key);
      if (key.startsWith('#') || (t && t.CanOpen)) toggle(key);
      else b.focus({ preventScroll: true });
    });
  }
  $('b-list').addEventListener('click', toggleList);
  $('help-close').addEventListener('click', closeHelp);
  $('viewer-close').addEventListener('click', closeImage);
  $('viewer').addEventListener('click', (e) => { if (e.target === $('viewer')) closeImage(); });
  $('help').addEventListener('click', (e) => { if (e.target === $('help')) closeHelp(); });
  if (window.ResizeObserver) new ResizeObserver(syncPadding).observe($('foot'));
  syncPadding();
  connect();
  refresh();
}

start();
