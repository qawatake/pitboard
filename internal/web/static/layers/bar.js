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

// note is a key that is not a button (shift+enter, j k, an Ask's a b):
// written as the buttons are, the key bold, what it does in the same
// color as theirs.
const note = (key, word, act) => {
  const s = el('span', 'barnote');
  s.append(keyLabel(key, word));
  if (act) s.dataset.act = act;
  return s;
};

// items is what the bar holds now.
export function items(cx) {
  const { page, ui } = cx;
  const out = [];
  if (ui.mode === 'editor') {
    for (const h of page.Editor || []) {
      out.push(h.Do === 'newline' ? note(h.Key, h.Word, h.Do) : key(cx, h));
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
    for (const h of (o && o.Hints) || []) {
      switch (h.Do) {
        case 'wait': out.push(note(h.Key, h.Word, h.Do)); break;
        case 'answer':
          for (const k of o.Options || []) {
            const opt = (t.Kids || []).find((x) => x.OptionKey === k);
            out.push(key(cx, h, { label: k, key: k, aria: h.Word + ': ' + (opt ? plain(opt.Title) : k) }));
          }
          { const w = el('span', 'barnote', h.Word); w.dataset.act = h.Do; out.push(w); }
          break;
        default: out.push(key(cx, h));
      }
    }
  }
  // After a jump (g, a link): the way back.
  if ((ui.back || []).length && !ui.panel) out.push(key(cx, { Do: 'back', Key: 'g -', Word: '← ' + cx.words('back') }));
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
  // The message: the person's last act's, else what the line's offer says
  // (a line rewritten under the cursor, one the page put the cursor on).
  // On its own line above the keys, so it never pushes them to a second.
  const status = foot.querySelector('.status');
  const o = !cx.ui.status && cx.ui.mode === 'normal' && !cx.ui.panel ? offerOf(cx.ui) : null;
  const msg = cx.ui.status || (o && o.Message) || '';
  if (status.textContent !== msg) status.textContent = msg;
  status.classList.toggle('need', !!cx.ui.status && cx.ui.statusTone === 'need');
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
// the bar takes one line (900px wide and more) or two. g's list keeps to
// the same: its titles are cut shorter first, then the last places go,
// counted in a "… +N" at its end. It needs the bar laid out (a step after
// drawing).
export function fit(foot) {
  const bar = foot.querySelector('.bar');
  const hints = bar.querySelector('.hints');
  const most = foot.clientWidth >= 900 ? 1 : 2;
  const lines = () => new Set([...bar.children, ...hints.children]
    .filter((x) => !x.hidden && x.getClientRects().length && x.className !== 'hints')
    .map((x) => Math.round(x.getBoundingClientRect().top))).size;
  for (const x of hints.querySelectorAll('.more')) x.remove();
  if (bar.classList.contains('jumps')) {
    const places = [...hints.querySelectorAll('[data-act="jumpTo"]')];
    for (const p of places) { p.hidden = false; p.style.removeProperty('--jw'); }
    for (const w of ['12em', '8em', '5em', '3em']) {
      if (lines() <= most) return;
      for (const p of places) p.style.setProperty('--jw', w);
    }
    const more = el('span', 'barnote more');
    hints.appendChild(more);
    for (let i = places.length - 1; i >= 0 && lines() > most; i--) {
      places[i].hidden = true;
      more.textContent = '… +' + (places.length - i);
    }
    if (!more.textContent) more.remove();
    return;
  }
  const keys = [...bar.querySelectorAll('.hints > [data-rank]')];
  for (const k of keys) k.hidden = false;
  const order = keys.map((k, i) => ({ k, r: Number(k.dataset.rank), i })).sort((a, b) => a.r - b.r || b.i - a.i);
  for (const { k } of order) {
    if (lines() <= most) break;
    k.hidden = true;
  }
}

const at = (cur, extra) => ({ cur, ...extra });

export const covers = {"mode": {"normal": ["an Item: enter c x, space g, s m ?"]}, "action": {"Options": ["an open Ask: its options"], "Comment": ["an Item: enter c x, space g, s m ?"], "Hide": ["a list line: x, and room for u 元に戻す, space 次へ"], "Read": ["unread: r 読んだ"]}, "hint": {"undo": ["after an act in this tab: u 元に戻す"], "open": ["an Item: enter c x, space g, s m ?"], "close": ["an open Item: enter 閉じる"], "read": ["unread: r 読んだ"], "answer": ["an open Ask: its options"], "wait": ["placed by the page: j k 動かすと…"], "comment": ["an Item: enter c x, space g, s m ?"], "hide": ["a list line: x, and room for u 元に戻す, space 次へ"], "next": ["a list line: x, and room for u 元に戻す, space 次へ"], "jump": ["an Item: enter c x, space g, s m ?"], "send": ["an Item: enter c x, space g, s m ?"], "write": ["an Item: enter c x, space g, s m ?"], "keys": ["an Item: enter c x, space g, s m ?"], "link": ["a line with one link: g 1 → where it goes"], "back": ["after a jump: g - ← 戻る"]}, "refusal": {"back": ["refusal: back"], "changed": ["refusal: changed"], "emptyComment": ["refusal: emptyComment"], "goGone": ["refusal: goGone"], "goNoLine": ["refusal: goNoLine"], "linkHidden": ["refusal: linkHidden"], "moved": ["refusal: moved"], "noMarked": ["refusal: noMarked"], "placedAnswer": ["refusal: placedAnswer"], "settlingHide": ["refusal: settlingHide"], "settlingReact": ["refusal: settlingReact"], "settlingShow": ["refusal: settlingShow"], "tooLong": ["refusal: tooLong"]}};

export const differences = {"message": ["a message"], "sent": ["sent"]};

export const variants = [
  { name: 'an Item: enter c x, space g, s m ?', ui: at('hook'), responsive: true },
  { name: 'after an act in this tab: u 元に戻す', ui: at('ただの行'), undo: true },
  { name: 'an open Item: enter 閉じる', ui: at('hook'), open: ['hook'] },
  { name: 'an open Ask: its options', ui: at('where'), open: ['where'] },
  { name: 'unread: r 読んだ', ui: at('reply') },
  { name: 'a line with one link: g 1 → where it goes', ui: at('もう 1 つのただの行。仕様は 最小版の仕様') },
  { name: 'after a jump: g - ← 戻る', ui: at('ただの行', { back: [{ cur: 'reply', y: 0 }] }) },
  { name: 'a list line: x, and room for u 元に戻す, space 次へ', ui: at('ただの行') },
  { name: 'placed by the page: j k 動かすと…', ui: at('merge'), open: ['merge'], placed: true },
  { name: 'rewritten under the cursor', ui: at('hook'), locked: true },
  { name: 'a hidden line: x 表示に戻す', fixture: 'hidden', ui: at('h1') },
  { name: 'a message', ui: at('hook', { status: '答え a を付けました: ページの単位' }) },
  { name: 'a message in two parts', ui: at('hook', { status: '送りました。ただ AI が待っていないので、チャットで一言伝えてください。\nAI に pitboard wait を動かしてもらうか、PITBOARD_SEND を設定してください (pitboard -h の "Send")' }) },
  { name: 'sending', ui: at('hook', { busy: true, status: '送っています…' }) },
  { name: 'sent', ui: at('hook'), sent: true },
  { name: 'refusal: back', ui: at('hook'), refusal: 'back' },
  { name: 'refusal: changed', ui: at('hook'), refusal: 'changed' },
  { name: 'refusal: emptyComment', ui: at('hook'), refusal: 'emptyComment' },
  { name: 'refusal: goGone', ui: at('hook'), refusal: 'goGone' },
  { name: 'refusal: goNoLine', ui: at('hook'), refusal: 'goNoLine' },
  { name: 'refusal: linkHidden', ui: at('hook'), refusal: 'linkHidden' },
  { name: 'refusal: moved', ui: at('hook'), refusal: 'moved' },
  { name: 'refusal: noMarked', ui: at('hook'), refusal: 'noMarked' },
  { name: 'refusal: placedAnswer', ui: at('hook'), refusal: 'placedAnswer' },
  { name: 'refusal: settlingHide', ui: at('hook'), refusal: 'settlingHide' },
  { name: 'refusal: settlingReact', ui: at('hook'), refusal: 'settlingReact' },
  { name: 'refusal: settlingShow', ui: at('hook'), refusal: 'settlingShow' },
  { name: 'refusal: tooLong', ui: at('hook'), refusal: 'tooLong' },
];
