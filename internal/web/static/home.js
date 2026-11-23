// The index: the boards this server shows, one row each (title, where,
// the same counts as each board's top right). j k move, enter opens. It is
// redrawn whenever any board changes. Text is data: textContent only.

const $ = (id) => document.getElementById(id);
const el = (tag, cls, text) => {
  const e = document.createElement(tag);
  if (cls) e.className = cls;
  if (text != null) e.textContent = text;
  return e;
};

let data = null;
let cur = '';
let offlineTimer = 0;

function setOffline(on) {
  const b = $('offline');
  if (on === !b.hidden) return;
  b.textContent = on ? ((data && data.Words.offline) || 'つながっていません') : '';
  b.hidden = !on;
  document.body.classList.toggle('offline', on);
  clearInterval(offlineTimer);
  if (on) offlineTimer = setInterval(refresh, 3000);
}

async function refresh() {
  let r;
  try {
    r = await fetch('/api/boards', { credentials: 'same-origin' });
  } catch { setOffline(true); return; }
  if (!r.ok) { setOffline(true); return; }
  data = await r.json();
  setOffline(false);
  render();
}

function render() {
  const box = $('boards');
  const had = document.activeElement && document.activeElement.dataset && document.activeElement.dataset.nav;
  const rows = [];
  if (!data.Boards.length) rows.push(el('p', 'msg', data.Words.none));
  if (!data.Boards.some((b) => b.ID === cur)) cur = data.Boards.length ? data.Boards[0].ID : '';
  for (const b of data.Boards) {
    const row = el('div', 'row boardrow' + (b.ID === cur ? ' cur' : ''));
    const a = el('a', 'rowbtn');
    a.href = '/s/' + encodeURIComponent(b.ID) + '/';
    a.dataset.nav = b.ID;
    if (b.ID === cur) a.setAttribute('aria-current', 'true');
    a.appendChild(el('span', 'gut'));
    a.appendChild(el('span', 'dot'));
    a.appendChild(el('span', 'sign'));
    const t = el('span', 'title');
    t.appendChild(el('span', 'btitle', b.Title || b.Message || b.Session));
    t.appendChild(el('span', 'faint bdir', b.Where || b.Dir));
    a.appendChild(t);
    const right = el('span', 'right');
    for (const w of b.Summary || []) right.appendChild(el('span', w.Tone, w.Text));
    a.appendChild(right);
    row.appendChild(a);
    rows.push(row);
  }
  box.replaceChildren(...rows);
  if (had) focusCur();
}

// mark shows the cursor on its row only.
function mark() {
  for (const a of document.querySelectorAll('#boards .rowbtn')) {
    const on = a.dataset.nav === cur;
    a.parentElement.classList.toggle('cur', on);
    if (on) a.setAttribute('aria-current', 'true'); else a.removeAttribute('aria-current');
  }
}

function focusCur() {
  const a = document.querySelector(`[data-nav="${CSS.escape(cur)}"]`);
  if (a) { a.focus({ preventScroll: true }); a.scrollIntoView({ block: 'nearest' }); }
}

function move(d) {
  if (!data || !data.Boards.length) return;
  const ids = data.Boards.map((b) => b.ID);
  let i = ids.indexOf(cur);
  i = Math.max(0, Math.min(ids.length - 1, i + d));
  cur = ids[i];
  mark();
  focusCur();
}

function onKey(e) {
  if (e.ctrlKey || e.metaKey || e.altKey || e.isComposing) return;
  let k = e.key;
  if (k.length === 1 && k.charCodeAt(0) >= 0xff01 && k.charCodeAt(0) <= 0xff5e) k = String.fromCharCode(k.charCodeAt(0) - 0xfee0);
  switch (k) {
    case 'j': case 'ArrowDown': e.preventDefault(); move(1); return;
    case 'k': case 'ArrowUp': e.preventDefault(); move(-1); return;
    case 'Enter': {
      if (document.activeElement && document.activeElement.dataset && document.activeElement.dataset.nav) return; // the link's own
      const a = document.querySelector(`[data-nav="${CSS.escape(cur)}"]`);
      if (a) { e.preventDefault(); location.href = a.href; }
    }
  }
}

function connect() {
  const es = new EventSource('/api/events');
  es.addEventListener('changed', refresh);
  es.addEventListener('open', refresh);
  es.addEventListener('error', () => {
    setOffline(true);
    if (es.readyState === EventSource.CLOSED) { es.close(); setTimeout(connect, 3000); }
  });
}

document.addEventListener('keydown', onKey);
document.addEventListener('focusin', (e) => {
  const id = e.target && e.target.dataset && e.target.dataset.nav;
  if (id && id !== cur) { cur = id; mark(); }
});
connect();
refresh();
