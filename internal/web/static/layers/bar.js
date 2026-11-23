// The bar at the bottom: the message line, then the keys as buttons, then
// the way to the comments. It is the terminal's hint line: the keys of the
// cursor's line (what the server offers for it: a key it would refuse is
// never shown), then the page's own (page.Bar). While writing, only the
// editor's keys (page.EditorHints): no key that would type into the text.
// When there is no room (one line from 900px wide, two below), keys are
// left out in page's order (DropRank), the terminal's: fit() does that
// once the bar is laid out.

import { el, keyLabel, plain } from '../dom.js';
import { cursorComment, index, offerOf } from '../state.js';
import * as goto from './goto.js';

// key is a key's button: data-act says what it does (a hint's Do).
function key(cx, h, extra = {}) {
  const b = el('button', 'key');
  b.type = 'button';
  b.append(keyLabel(extra.label || h.Key, extra.label ? '' : h.Word));
  b.dataset.act = h.Do;
  b.dataset.key = extra.key || h.Key;
  const r = cx.page.Drop && cx.page.Drop[h.Do];
  if (r) b.dataset.rank = String(r);
  b.setAttribute('aria-keyshortcuts', h.Key === 'enter' ? 'Enter' : h.Key === 'space' ? 'Space' : h.Key === 'esc' ? 'Escape' : h.Key);
  if (extra.aria) b.setAttribute('aria-label', extra.aria);
  if (cx.ui.conn !== 'live') b.disabled = true;
  return b;
}

const note = (text, act) => {
  const s = el('span', 'faint barnote', text);
  if (act) s.dataset.act = act;
  return s;
};

// items is what the bar holds now.
export function items(cx) {
  const { page, ui } = cx;
  const out = [];
  if (ui.mode === 'editor') {
    for (const h of page.Editor || []) {
      out.push(h.Do === 'newline' ? note(h.Key + ' ' + h.Word, h.Do) : key(cx, h));
    }
    return out;
  }
  if (ui.mode === 'goto') return goto.items(cx, key);
  const c = cursorComment(page, ui);
  const t = index(page).things.get(ui.cur);
  if (c) {
    if (c.Target && index(page).byTarget.has(c.Target)) out.push(key(cx, { Do: 'go', Key: 'enter', Word: cx.words("go") }));
    for (const h of page.Comment || []) out.push(key(cx, h));
    out.push(key(cx, { Do: "closePanel", Key: "esc", Word: cx.words("close") }));
  } else if (t && !ui.panel) {
    const o = offerOf(ui);
    if (o && o.Message) out.push(note(o.Message));
    for (const h of (o && o.Hints) || []) {
      switch (h.Do) {
        case 'wait': out.push(note(h.Key + ' ' + h.Word, h.Do)); break;
        case 'answer':
          for (const k of o.Options || []) {
            const opt = (t.Kids || []).find((x) => x.OptionKey === k);
            out.push(key(cx, h, { label: k, key: k, aria: h.Word + ': ' + (opt ? plain(opt.Title) : k) }));
          }
          out.push(note(h.Word, h.Do));
          break;
        default: out.push(key(cx, h));
      }
    }
  }
  for (const h of page.More || []) {
    if (ui.panel && h.Do !== 'undo') continue;
    out.push(key(cx, h));
  }
  for (const h of page.Tail || []) {
    const b = key(cx, h);
    if (h.Do === 'send') { b.classList.add('need'); if (ui.busy) b.disabled = true; }
    out.push(b);
  }
  return out;
}

// draw fills the bottom of the page: the message, the keys, the comments.
export function draw(cx, foot) {
  const status = foot.querySelector('.status');
  if (status.textContent !== cx.ui.status) status.textContent = cx.ui.status;
  const hints = foot.querySelector('.hints');
  const focused = hints.contains(document.activeElement) ? document.activeElement.dataset.key : null;
  hints.replaceChildren(...items(cx));
  const b = foot.querySelector('.b-comments');
  const n = ((cx.page && cx.page.Comments) || []).filter((c) => !c.Finished).length;
  const kids = [document.createTextNode('コメント ' + n)];
  if (cx.page && cx.page.Unsent > 0) kids.push(' ', el('span', 'need', '未送信 ' + cx.page.Unsent));
  b.replaceChildren(...kids);
  b.setAttribute('aria-expanded', String(cx.ui.panel));
  b.disabled = cx.ui.conn !== 'live' || cx.ui.mode === 'editor';
  foot.querySelector('.bar').classList.toggle('jumps', cx.ui.mode === 'goto');
  return focused;
}

// fit leaves keys out, lowest rank first (the last of a rank first), until
// the bar takes one line (900px wide and more) or two; g's list may take
// three. It needs the bar laid out (a step after drawing).
export function fit(foot) {
  const bar = foot.querySelector('.bar');
  const keys = [...bar.querySelectorAll('.hints > [data-rank]')];
  for (const k of keys) k.hidden = false;
  const most = bar.classList.contains('jumps') ? 3 : foot.clientWidth >= 900 ? 1 : 2;
  const lines = () => new Set([...bar.children, ...bar.querySelector('.hints').children]
    .filter((x) => !x.hidden && x.getClientRects().length && x.className !== 'hints')
    .map((x) => Math.round(x.getBoundingClientRect().top))).size;
  const order = keys.map((k, i) => ({ k, r: Number(k.dataset.rank), i })).sort((a, b) => a.r - b.r || b.i - a.i);
  for (const { k } of order) {
    if (lines() <= most) break;
    k.hidden = true;
  }
}

const at = (cur, extra) => ({ cur, ...extra });

export const covers = {"mode": ["normal"], "action": ["YN", "Options", "Comment", "Hide", "Read"], "hint": ["open", "close", "read", "answer", "wait", "y", "n", "comment", "hide", "undo", "next", "jump", "send", "write", "keys"]};

export const variants = [
  { name: 'an Item: enter y n c x, u space g, s m ?', ui: at('hook'), responsive: true },
  { name: 'an open Item: enter 閉じる', ui: at('hook'), open: ['hook'] },
  { name: 'an open Ask: its options', ui: at('where'), open: ['where'] },
  { name: 'unread: r 読んだ', ui: at('reply') },
  { name: 'a list line: x, and room for u 元に戻す, space 次へ', ui: at('ただの行') },
  { name: 'placed by the page: j k 動かすと…', ui: at('merge'), placed: true },
  { name: 'rewritten under the cursor', ui: at('hook'), locked: true },
  { name: 'a hidden line: x 表示に戻す', fixture: 'hidden', ui: at('h1') },
  { name: 'a message', ui: at('hook', { status: 'n を付けました: プルリクエスト' }) },
  { name: 'a message in two parts', ui: at('hook', { status: '送りました。ただ AI が待っていないので、チャットで一言伝えてください。\nAI に pitboard wait を動かしてもらうか、PITBOARD_SEND を設定してください (pitboard -h の "Send")' }) },
  { name: 'sending', ui: at('hook', { busy: true, status: '送っています…' }) },
];
