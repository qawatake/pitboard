// pitboard in a browser. The rules (what a line is, its marks, its words,
// what can be done on it) all come from the server (internal/page); this
// file only draws them and sends the person's actions.
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
  open: new Map(),   // key -> open (what the person opened or closed)
  copen: new Map(),  // comment row key -> open
  cur: '',           // the selected row's key
  curTitle: '',      // what it said when selected (to notice a rewrite)
  placed: true,      // the page put the selection there, not the person
  lock: false,       // the selected line was rewritten or went: no reactions until moved
  autoAt: 0,         // when the page moved the selection by itself
  showHidden: false,
  list: false,       // the comment list instead of the report
  editor: null,      // {kind, key, token, n, at, label}
  jumping: false,
  busy: false,
  composing: false,
  pending: false,    // a refresh waiting for the input method
};
const SETTLE = 400;
const draftKey = (ed) => 'pitboard-draft:' + ed.kind + ':' + (ed.kind === 'edit' ? ed.n : ed.key);
const store = {
  get(k) { try { return sessionStorage.getItem(k); } catch { return null; } },
  set(k, v) { try { sessionStorage.setItem(k, v); } catch { /* the draft lives only on screen */ } },
  del(k) { try { sessionStorage.removeItem(k); } catch { /* nothing */ } },
};

// ---- talking to the server ----

async function api(path, body) {
  const opt = { headers: { 'X-Pitboard-Tab': tabId }, credentials: 'same-origin' };
  if (body !== undefined) {
    opt.method = 'POST';
    opt.headers['Content-Type'] = 'application/json';
    opt.body = JSON.stringify(body);
  }
  const r = await fetch(path, opt);
  if (!r.ok) throw new Error((await r.text()).trim() || r.statusText);
  return r.json();
}

async function refresh() {
  if (S.composing) { S.pending = true; return; }
  try {
    S.data = await api('/api/page' + (S.showHidden ? '?hidden=1' : ''));
  } catch (e) {
    say('読めませんでした: ' + e.message);
    return;
  }
  render();
}

function say(msg) {
  const s = $('status');
  s.textContent = '';
  // A new text each time, so it is read out even when it repeats.
  requestAnimationFrame(() => { s.textContent = msg || ''; });
}

async function act(action, extra = {}) {
  if (S.busy) return null;
  try {
    const r = await api('/api/act', { Action: action, ShowHidden: S.showHidden, ...extra });
    say(r.Message);
    await refresh();
    return r;
  } finally {
    S.going = '';
  }
}

// ---- walking the page ----

function allThings() {
  const out = [];
  const walk = (ts) => { for (const t of ts || []) { out.push(t); walk(t.Kids); } };
  if (S.data) { walk(S.data.Notices); walk(S.data.Things); }
  return out;
}
const byKey = (k) => allThings().find((t) => t.Key === k && t.Stop);
const comments = () => (S.data && S.data.Comments) || [];
const commentOf = (n) => comments().find((c) => c.N === n);
const isOpen = (t) => (S.open.has(t.Key) ? S.open.get(t.Key) : t.Open);
const canWrite = () => S.data && S.data.CanWrite;
const navRows = () => Array.from(document.querySelectorAll('[data-nav]'));
const plain = (spans) => (spans || []).map((s) => s.Text).join('');

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

function rightOf(t) {
  const r = t.Right || {};
  const box = el('span', 'right');
  const mine = [];
  if (r.Reaction) {
    const cls = r.Reaction === 'y' ? 'yes' : r.Reaction === 'n' ? 'no' : 'optkey';
    mine.push(el('span', cls, r.Reaction));
    if (t.Kind === 'ask' && r.Option && !isOpen(t)) {
      const head = [...r.Option].length > 12 ? [...r.Option].slice(0, 11).join('') + '…' : r.Option;
      mine.push(el('span', 'faint', head));
    }
  }
  if (r.Input && r.Input.Text) {
    const w = el('span', '', '');
    if (r.Count > 0) w.appendChild(el('span', 'faint', r.Count + ' '));
    w.appendChild(word(r.Input));
    mine.push(w);
  }
  for (const m of mine) box.appendChild(m);
  if (r.AtText) box.appendChild(el('span', 'faint', r.AtText));
  else if (r.Status && r.Status.Text) box.appendChild(word(r.Status));
  return box;
}

function signOf(t, open) {
  if (t.Hidden) return '⌀';
  if (t.Kind === 'link') return '→';
  if (t.Kind === 'item' && t.State === 'done') return '✓';
  if (t.Kind === 'ask') return '?';
  if (t.CanOpen) return open ? '▾' : '▸';
  if (t.Kind === 'notice') return '!';
  if (t.Kind === 'list') return t.Num || (t.Level === 1 ? '-' : '·');
  return '·';
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

// row draws a line the person can point at, and what it shows open.
function row(t, opts = {}) {
  const open = t.CanOpen && isOpen(t);
  const box = el('div', 'row');
  box.dataset.key = t.Key;
  if (t.Kind === 'notice') box.classList.add('notice');
  if (t.Kind === 'ask') box.classList.add('ask');
  if (t.Kind === 'link') box.classList.add('linkrow');
  if (t.State === 'done') box.classList.add('done');
  if (t.Hidden) box.classList.add('hidden');
  if (opts.heading && !t.Hidden) box.classList.add('heading');
  if (open) box.classList.add('open');
  if (t.Key === S.cur && !S.list) box.classList.add('cur');
  const b = el('button', 'rowbtn');
  b.type = 'button';
  b.dataset.nav = t.Key;
  if (t.CanOpen) b.setAttribute('aria-expanded', String(open));
  if (t.Key === S.cur) b.setAttribute('aria-current', 'true');
  b.appendChild(markOf(t));
  b.appendChild(el('span', 'sign', signOf(t, open)));
  const title = el('span', 'title');
  if (S.jumping && opts.jump) title.appendChild(el('span', 'jumpkey', opts.jump));
  if (t.Jump) box.dataset.jump = String(t.Jump);
  spans(title, t.Title, true);
  if (t.Hidden) title.appendChild(el('span', 'sr', ' (隠した行)'));
  b.appendChild(title);
  b.appendChild(rightOf(t));
  b.addEventListener('click', () => clickRow(t.Key));
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
  if (t.Key === S.cur && !S.list) box.appendChild(actionsOf(t));
  if (S.editor && S.editor.at === t.Key && !S.list) box.appendChild(editorBox());
  if (open) {
    if (t.Kind === 'fold') {
      box.appendChild(commentsOn(t, 'body'));
      const kids = el('div', 'kids');
      things(kids, t.Kids);
      box.appendChild(kids);
    } else {
      box.appendChild(contents(t));
      if (t.Kind === 'ask') {
        for (const o of t.Kids || []) {
          const line = el('div', 'option');
          const k = el('span', o.Chosen ? 'chosen' : 'faint', (o.Chosen ? '● ' : '') + o.OptionKey);
          if (o.Chosen) k.appendChild(el('span', 'sr', ' (選んだ答え)'));
          line.appendChild(k);
          line.appendChild(spans(el('span'), o.Title));
          box.appendChild(line);
        }
      }
    }
  }
  return box;
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

function things(parent, ts, top) {
  for (const t of ts || []) {
    const jump = t.Jump ? jumpLetter(t.Jump - 1) : '';
    switch (t.Kind) {
      case 'title':
        parent.appendChild(spans(el('h1'), t.Title));
        break;
      case 'section': {
        const h = el('h2', 'section');
        if (t.Jump) h.dataset.jump = String(t.Jump);
        if (S.jumping && jump) h.appendChild(el('span', 'jumpkey', jump));
        spans(h, t.Title);
        parent.appendChild(h);
        things(parent, t.Kids);
        break;
      }
      default:
        if (t.Stop) {
          const r = row(t, { heading: top && t.Kind === 'fold', jump });
          if (t.Kind === 'list' && t.Subs && t.Subs.length) {
            const s = el('div', 'subs body');
            for (const x of t.Subs) s.appendChild(block(x));
            r.insertBefore(s, r.children[1] || null);
          }
          parent.appendChild(r);
        } else {
          parent.appendChild(block(t));
        }
    }
  }
}

// actionsOf is the buttons of the selected line: the terminal's hint words.
function actionsOf(t) {
  const box = el('div', 'actions');
  box.setAttribute('role', 'group');
  box.setAttribute('aria-label', 'この行でできること');
  const a = t.Actions || {};
  const add = (label, key, fn, primary) => {
    const b = el('button', primary ? 'primary' : '', label);
    b.type = 'button';
    if (key) {
      b.append(' ', el('kbd', '', key));
      b.setAttribute('aria-keyshortcuts', key);
    }
    b.addEventListener('click', (e) => { e.stopPropagation(); S.placed = false; S.lock = false; fn(); });
    box.appendChild(b);
  };
  const askOpen = t.Kind === 'ask' && isOpen(t);
  if (askOpen) for (const k of a.Options || []) {
    const o = (t.Kids || []).find((x) => x.OptionKey === k);
    add((o ? plain(o.Title) : k), k, () => react(t, k));
  }
  if (t.CanOpen) add(isOpen(t) ? '閉じる' : '開く', 'enter', () => toggle(t.Key));
  if (a.Read) add('読んだ', 'r', () => read(t));
  if (a.YN) {
    add('よし', 'y', () => react(t, 'y'));
    add('だめ', 'n', () => react(t, 'n'));
  }
  if (a.Comment) add('行に書く', 'c', () => openEditor({ kind: 'comment', key: t.Key, token: t.Token, at: t.Key, label: '「' + plain(t.Title) + '」にコメント' }));
  if (a.Hide) add(t.Hidden ? '表示に戻す' : '隠す', 'x', () => hide(t));
  return box;
}

function render() {
  const d = S.data;
  if (!d) return;
  // Keep what the person is looking at where it is on screen.
  const anchor = document.querySelector('.row.cur') || document.querySelector('.editor');
  const before = anchor ? anchor.getBoundingClientRect().top : null;
  const focusRow = document.activeElement && document.activeElement.dataset && document.activeElement.dataset.nav;
  const ed = saveEditor();

  checkCursor();

  // top right
  const sum = $('summary');
  sum.replaceChildren();
  for (const w of d.Summary || []) sum.appendChild(word(w));
  if (d.Hidden > 0) sum.appendChild(el('span', 'faint', `隠した行 ${d.Hidden}` + (S.showHidden ? ' (表示中)' : '')));

  const nb = $('notices');
  nb.replaceChildren();
  const board = $('board');
  board.replaceChildren();
  const pane = $('pane');
  pane.replaceChildren();

  if (S.list) {
    renderList(board);
  } else {
    for (const t of d.Notices || []) nb.appendChild(row(t, { jump: t.Jump ? jumpLetter(t.Jump - 1) : '' }));
    if (d.Message) board.appendChild(el('p', 'msg', d.Message));
    if (d.Err) board.appendChild(el('p', 'err', '読めませんでした: ' + d.Err));
    if (d.ReadErr) board.appendChild(el('p', 'err', d.ReadErr));
    if (d.Problems && d.Problems.length) {
      const p = el('div', 'err');
      p.appendChild(el('strong', '', '報告に書き方の誤りがあります' + (d.Partial ? ' (読めた所までを出しています)' : '')));
      for (const l of d.Problems) p.appendChild(el('div', '', l));
      board.appendChild(p);
    }
    things(board, d.Things, true);
    if (S.editor && !S.editor.at) board.appendChild(editorBox());
    renderPane(pane);
  }
  restoreEditor(ed);
  renderBar();

  const now = document.querySelector('.row.cur') || document.querySelector('.editor');
  if (now && before != null) window.scrollBy(0, now.getBoundingClientRect().top - before);
  if (focusRow && !ed.focused) {
    const b = document.querySelector(`[data-nav="${CSS.escape(S.cur)}"]`);
    if (b) b.focus({ preventScroll: true });
  }
}

function renderBar() {
  const d = S.data;
  const send = $('b-send');
  send.replaceChildren(document.createTextNode(d.Unsent > 0 ? `送る (${d.Unsent}) ` : '送る '), el('kbd', '', 's'));
  send.classList.toggle('primary', d.Unsent > 0 && d.CanWrite);
  send.disabled = S.busy || !d.CanWrite;
  $('b-write').disabled = S.busy || !d.CanWrite;
  $('b-undo').disabled = S.busy || !d.CanUndo;
  $('b-list').textContent = S.list ? '報告へ' : 'コメント一覧';
}

// commentRow draws one comment as a line of the comment area or list.
function commentRow(c, key, numbered) {
  const open = !!S.copen.get(key);
  const box = el('div', 'row' + (open ? ' open' : '') + (key === S.cur ? ' cur' : '') + (c.Finished ? ' done' : ''));
  box.dataset.key = key;
  const b = el('button', 'rowbtn');
  b.type = 'button';
  b.dataset.nav = key;
  b.setAttribute('aria-expanded', String(open));
  if (key === S.cur) b.setAttribute('aria-current', 'true');
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
  b.addEventListener('click', () => clickRow(key));
  box.appendChild(b);
  if (key === S.cur && canWrite()) {
    const a = el('div', 'actions');
    a.setAttribute('role', 'group');
    a.setAttribute('aria-label', 'このコメントでできること');
    const add = (label, k, fn) => {
      const x = el('button', '', label);
      x.type = 'button';
      x.append(' ', el('kbd', '', k));
      x.setAttribute('aria-keyshortcuts', k);
      x.addEventListener('click', (e) => { e.stopPropagation(); fn(); });
      a.appendChild(x);
    };
    add(open ? '閉じる' : '開く', 'enter', () => toggle(key));
    add('直す', 'e', () => editComment(c, key));
    add('取り消す', 'x', () => retract(c));
    box.appendChild(a);
  }
  if (S.editor && S.editor.at === key) box.appendChild(editorBox());
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
  if (S.editor && !S.editor.at) board.appendChild(editorBox());
}

// ---- the selection ----

// checkCursor keeps the selection on the same line across a refresh. A line
// gone, or rewritten into another, moves or locks it: nothing is done on it
// until the person moves.
function checkCursor() {
  const keys = navKeysOf();
  if (!keys.length) { S.cur = ''; return; }
  if (!S.cur || !keys.includes(S.cur)) {
    const had = S.cur !== '';
    const i = had ? Math.min(S.lastIndex || 0, keys.length - 1) : 0;
    const expected = had && S.cur === S.going;
    S.cur = keys[Math.max(i, 0)];
    S.curTitle = titleOf(S.cur);
    if (expected) {
      // The person took the line away (hid it, took the comment back): the
      // selection goes to what is next, and waits a moment there.
      S.autoAt = Date.now();
    } else if (had) {
      lockNow();
    } else {
      S.placed = true;
    }
    S.going = '';
    return;
  }
  const t = titleOf(S.cur);
  if (S.curTitle && t !== S.curTitle) lockNow();
  S.curTitle = t;
  S.lastIndex = keys.indexOf(S.cur);
}

function lockNow() {
  if (!S.lock) say('報告が書き換わりました。カーソルの行を確かめてください');
  S.lock = true;
  S.autoAt = 0;
}

function titleOf(key) {
  if (key.startsWith('#')) return 'comment';
  const t = byKey(key);
  return t ? t.Kind + '|' + plain(t.Title) : '';
}

// navKeysOf is the keys of the lines one can point at, in the order shown.
function navKeysOf() {
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

function select(key, how) {
  if (S.lock) say('');
  S.cur = key;
  S.curTitle = titleOf(key);
  S.lastIndex = navKeysOf().indexOf(key);
  S.lock = false;
  S.placed = false;
  S.autoAt = how === 'auto' ? Date.now() : 0;
  render();
  const b = document.querySelector(`[data-nav="${CSS.escape(key)}"]`);
  if (b) {
    b.focus({ preventScroll: true });
    b.scrollIntoView({ block: 'nearest' });
  }
}

function move(d) {
  const keys = navKeysOf();
  if (!keys.length) return;
  let i = keys.indexOf(S.cur);
  i = i < 0 ? 0 : Math.max(0, Math.min(keys.length - 1, i + d));
  select(keys[i]);
}

// settled says whether reactions may go on the selected line: the person
// pointed at it, it was not rewritten, and a line the page moved to has
// been on screen a moment.
function settled() {
  return !S.placed && !S.lock && (!S.autoAt || Date.now() - S.autoAt >= SETTLE);
}

function clickRow(key) {
  S.cur = key;
  S.curTitle = titleOf(key);
  S.lastIndex = navKeysOf().indexOf(key);
  S.lock = false;
  S.placed = false;
  S.autoAt = 0;
  const t = byKey(key);
  if (key.startsWith('#')) toggle(key);
  else if (t && t.CanOpen) toggle(key);
  else render();
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
  const b = document.querySelector(`[data-nav="${CSS.escape(key)}"]`);
  if (b) b.focus({ preventScroll: true });
}

// ---- actions ----

async function guarded(fn) {
  try {
    await fn();
  } catch (e) {
    say('できませんでした: ' + e.message);
  }
}

function react(t, value) {
  // The options of a closed Ask are not on screen: the key only opens it.
  if (t.Kind === 'ask' && !isOpen(t) && value !== 'y' && value !== 'n') { toggle(t.Key); return; }
  guarded(() => act('react', { Target: t.Key, Seen: t.Token, Value: value }));
}
function hide(t) {
  S.going = t.Key;
  guarded(() => act('hide', { Target: t.Key, Seen: t.Token }));
}
function read(t) { guarded(() => act('read', { Target: t.Key, Seen: t.Token })); }
function retract(c) {
  S.going = S.cur;
  guarded(() => act('retract', { N: c.N }));
}
function undo() { S.going = S.cur; guarded(() => act('undo')); }
function readAll() { if (canWrite()) guarded(() => act('readall')); }

async function send() {
  if (S.busy || !canWrite()) return;
  S.busy = true;
  document.body.setAttribute('aria-busy', 'true');
  renderBar();
  say('送っています…');
  try {
    const r = await api('/api/send', {});
    say(r.Message);
  } catch (e) {
    say('送れませんでした: ' + e.message);
  } finally {
    S.busy = false;
    document.body.removeAttribute('aria-busy');
    await refresh();
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
  edArea.focus({ preventScroll: true });
  edArea.setSelectionRange(ed.start, ed.end);
}

function openEditor(ed) {
  if (!canWrite()) return;
  S.editor = ed;
  editorBox();
  edArea.value = store.get(draftKey(ed)) ?? (ed.text || '');
  render();
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
  render();
  const b = at && document.querySelector(`[data-nav="${CSS.escape(at)}"]`);
  if (b) b.focus({ preventScroll: true }); else $('board').focus({ preventScroll: true });
}

async function place() {
  const ed = S.editor;
  const text = edArea.value;
  if (!text.trim()) {
    store.del(draftKey(ed));
    say('空だったので書きませんでした');
    closeEditor();
    return;
  }
  let r;
  try {
    if (ed.kind === 'edit') r = await api('/api/act', { Action: 'edit', N: ed.n, Text: text });
    else r = await api('/api/act', { Action: 'comment', Target: ed.key || '', Seen: ed.token || '', Text: text });
  } catch (e) {
    say('置けませんでした: ' + e.message);
    return;
  }
  if (!r.OK) { say(r.Message); return; }
  store.del(draftKey(ed));
  edArea.value = '';
  say(r.Message || (ed.kind === 'edit' ? 'コメントを直しました' : 'コメントを置きました (s で送る)'));
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
    say(`画像が大きすぎるので貼りませんでした (${S.data.ImageMax >> 20} MiB まで)`);
    return;
  }
  const rd = new FileReader();
  rd.onload = async () => {
    const data = String(rd.result).replace(/^data:[^,]*,/, '');
    let r;
    try {
      r = await api('/api/image', { Data: data });
    } catch (e) {
      say('貼れませんでした: ' + e.message);
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

// ---- images ----

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

// ---- keys ----

const HELP = [
  ['j k ↑↓', '上下の行へ (日本語入力のままでも全角の ｊ ｋ が効く)'],
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

function jumpLetter(i) { return String.fromCharCode(97 + (i >= 6 ? i + 1 : i)); }
function jumpIndex(ch, n) {
  if (ch < 'a' || ch > 'z' || ch === 'g') return -1;
  let i = ch.charCodeAt(0) - 97;
  if (ch > 'g') i--;
  return i < n ? i : -1;
}

// goTo goes to place i of g: its line (a Section's first), its heading at
// the top of the screen.
function goTo(i) {
  const key = S.data.JumpTo[i];
  if (key) select(key, 'auto');
  else render();
  const h = document.querySelector(`[data-jump="${i + 1}"]`);
  if (h) h.scrollIntoView({ block: 'start' });
}

function jumpNext() {
  const d = S.data;
  const order = [];
  const walk = (ts) => { for (const t of ts || []) { if (t.Stop) order.push(t.Key); walk(t.Kids); } };
  walk(d.Notices); walk(d.Things);
  const at = order.indexOf(S.cur);
  const next = d.Next || [];
  if (!next.length) { say('印のある行はありません'); return; }
  const pos = (k) => order.indexOf(k);
  const target = next.find((x) => pos(x.Key) > at) || next[0];
  for (const f of target.Folds || []) S.open.set(f, true);
  select(target.Key, 'auto');
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

function onKey(e) {
  if (!$('viewer').hidden) {
    if (e.key === 'Escape') { e.preventDefault(); closeImage(); }
    return;
  }
  if (!$('help').hidden) {
    if (e.key === 'Escape' || e.key === '?' || e.key === 'q') { e.preventDefault(); closeHelp(); }
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
    S.jumping = false;
    const i = jumpIndex(k, (S.data.JumpTo || []).length);
    if (i >= 0) goTo(i);
    else { say(''); render(); }
    return;
  }
  const t = byKey(S.cur);
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
    case 'H': done(); S.showHidden = !S.showHidden; refresh(); return;
    case 'A': done(); readAll(); return;
    case 'g':
      if (S.list || !(S.data.JumpTo || []).length) return;
      done(); S.jumping = true; say('飛ぶ先の字を押してください (esc やめる)'); render(); return;
    case 'Escape': return;
  }
  if (c) {
    if (k === 'e') { done(); editComment(c, S.cur); }
    if (k === 'x' && settled()) { done(); retract(c); }
    return;
  }
  if (!t || S.list) return;
  const a = t.Actions || {};
  switch (k) {
    case 'r': done(); if (a.Read && !S.lock && settled()) read(t); return;
    case 'c': done(); if (a.Comment && !S.lock) openEditor({ kind: 'comment', key: t.Key, token: t.Token, at: t.Key, label: '「' + plain(t.Title) + '」にコメント' }); return;
    case 'x': done(); if (a.Hide && settled()) hide(t); return;
  }
  if (k.length !== 1) return;
  const ask = t.Kind === 'ask';
  if (ask && !isOpen(t) && (a.Options || []).includes(k)) { done(); toggle(t.Key); return; }
  if (!settled()) {
    if ((a.YN && (k === 'y' || k === 'n')) || (a.Options || []).includes(k)) {
      done();
      say(S.lock ? '報告が書き換わりました。カーソルの行を確かめてください' : '行を動かすと効きます (j k)');
    }
    return;
  }
  if ((a.YN && (k === 'y' || k === 'n')) || (ask && (a.Options || []).includes(k))) { done(); react(t, k); }
}

// ---- start ----

function start() {
  drawHelp();
  document.addEventListener('keydown', onKey);
  $('b-write').addEventListener('click', () => openEditor({ kind: 'comment', key: '', at: S.list ? '' : S.cur, label: '宛先なしのコメント' }));
  $('b-send').addEventListener('click', send);
  $('b-undo').addEventListener('click', undo);
  $('b-keys').addEventListener('click', openHelp);
  $('b-list').addEventListener('click', () => {
    S.list = !S.list;
    S.cur = '';
    S.editor = null;
    if (edBox) edBox.remove();
    render();
    $('board').focus({ preventScroll: true });
  });
  $('help-close').addEventListener('click', closeHelp);
  $('viewer-close').addEventListener('click', closeImage);
  $('viewer').addEventListener('click', (e) => { if (e.target === $('viewer')) closeImage(); });
  $('help').addEventListener('click', (e) => { if (e.target === $('help')) closeHelp(); });
  const es = new EventSource('/api/events');
  es.addEventListener('changed', () => refresh());
  es.addEventListener('open', () => refresh());
  refresh();
}

start();
