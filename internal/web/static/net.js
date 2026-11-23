// Talking to the server: the board's calls (under /s/<id>) and its event
// stream. Nothing here draws; the caller is told what happened.

export class Offline extends Error {}
export class Gone extends Error {}

// connect makes the calls of the board at base for browser tab tabId.
export function connect(base, tabId) {
  async function request(path, body, headers = {}) {
    const opt = { headers: { 'X-Pitboard-Tab': tabId, ...headers }, credentials: 'same-origin' };
    if (body !== undefined) {
      opt.method = 'POST';
      opt.headers['Content-Type'] = 'application/json';
      opt.body = JSON.stringify(body);
    }
    let r;
    try {
      r = await fetch(base + path, opt);
    } catch {
      throw new Offline('');
    }
    // The board was dropped (its directory went): the server says so.
    if (r.status === 404 && base) throw new Gone((await r.text()).trim());
    // The server was started again (a new key), or the cookie is gone.
    if (r.status === 403) throw new Offline('');
    if (!r.ok && r.status !== 304) throw new Error((await r.text()).trim() || r.statusText);
    return r;
  }
  const api = async (path, body) => (await request(path, body)).json();

  // events calls on.changed when the board changes, on.open when the
  // stream (re)opens, on.lost while it cannot, on.gone(msg) when the board
  // was dropped.
  function events(on) {
    let es = null;
    let retry = 2000;
    let stopped = false;
    const open = () => {
      es = new EventSource(base + '/api/events');
      es.addEventListener('changed', () => on.changed());
      es.addEventListener('open', () => { retry = 2000; on.open(); });
      es.addEventListener('gone', (e) => { stopped = true; es.close(); on.gone(e.data); });
      es.addEventListener('error', () => {
        if (stopped) return;
        on.lost();
        if (es.readyState === EventSource.CLOSED) {
          es.close();
          setTimeout(open, retry);
          retry = Math.min(retry * 2, 30000);
        }
      });
    };
    open();
    // reopen opens it again (the board came back after gone).
    return { reopen: () => { if (es) es.close(); stopped = false; retry = 2000; open(); } };
  }
  return { request, api, events };
}
