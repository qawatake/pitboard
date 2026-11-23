// The index, live: layers/home.js draws it; this file fetches it, moves
// the cursor (j k), opens a board (enter) and redraws it whenever any
// board changes.

import { createHome, draw } from './layers/home.js';

const app = createHome();
document.body.appendChild(app);
let data = null;
let cur = '';
let offline = false;
let offlineTimer = 0;

function setOffline(on) {
  if (on === offline) return;
  offline = on;
  clearInterval(offlineTimer);
  if (on) offlineTimer = setInterval(refresh, 3000);
  if (data) render();
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
  const had = document.activeElement && document.activeElement.dataset && document.activeElement.dataset.nav;
  if (!data.Boards.some((b) => b.ID === cur)) cur = data.Boards.length ? data.Boards[0].ID : '';
  draw(app, data, cur, offline);
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
