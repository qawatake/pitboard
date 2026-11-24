// Comment: one of the person's comments. Under an opened line (on) its
// text keeps its line breaks, the state word sits at the right end, and
// what belongs to the text (its images, the title it was written on)
// comes right under it; the AI's reply follows, all on the text's column.
// In the comment panel (panelRow), the same with its number and the line
// it is on.

import { el, word } from '../dom.js';

// text is a comment's text, "[Image #N]" faint.
export function text(s) {
  const span = el('span', 'ctext');
  const re = /\[Image #[0-9]+\]/g;
  let last = 0;
  for (const m of s.matchAll(re)) {
    span.appendChild(document.createTextNode(s.slice(last, m.index)));
    span.appendChild(el('span', 'imgref', m[0]));
    last = m.index + m[0].length;
  }
  span.appendChild(document.createTextNode(s.slice(last)));
  return span;
}

// thumbs is a comment's images, at a size to read (the CSS caps them;
// never larger than they are); each opens whole (data-image). In the
// editor (removable), each also has a × that takes its [Image #N] out of
// the text (data-unimage).
export function thumbs(cx, ns, removable) {
  const box = el('div', 'thumbs');
  for (const n of ns || []) {
    const t = el('span', 'thumb');
    const b = el('button');
    b.type = 'button';
    b.dataset.image = String(n);
    b.setAttribute('aria-label', `画像 ${n} を大きく見る`);
    const img = el('img');
    img.src = cx.env.imageURL(n);
    img.alt = `[Image #${n}]`;
    b.appendChild(img);
    t.appendChild(b);
    if (removable) {
      const x = el('button', 'unimage', '×');
      x.type = 'button';
      x.dataset.unimage = String(n);
      x.setAttribute('aria-label', `画像 ${n} を文から外す`);
      t.appendChild(x);
    }
    box.appendChild(t);
  }
  return box;
}

function reply(c) {
  if (!c.Settled) return null;
  const p = el('div', 'creply');
  p.appendChild(el('span', 'quiet', c.SettledHead));
  p.appendChild(document.createTextNode(c.Settled));
  return p;
}

// on is the comments written on line t, as shown under it opened.
export function on(cx, t, cls) {
  const box = el('div', 'comments' + (cls ? ' ' + cls : ''));
  for (const c of t.Comments || []) {
    const d = el('div', 'comment');
    d.appendChild(el('span', 'cm', '✎'));
    d.appendChild(text(c.Text));
    d.appendChild(word(c.Word, 'cstate') || el('span'));
    const under = el('div', 'cunder');
    if (c.Images && c.Images.length) under.appendChild(thumbs(cx, c.Images));
    if (c.TitleThen) under.appendChild(el('div', 'quiet', '(書いたときは: ' + c.TitleThen + ')'));
    const r = reply(c);
    if (r) under.appendChild(r);
    if (under.firstChild) d.appendChild(under);
    box.appendChild(d);
  }
  return box;
}

// panelRow is comment c in the comment panel, as row key: its number in a
// column of its own (the text hangs past it), its state, the line it is on
// (a link there when the line is on the page), and what the AI wrote.
export function panelRow(cx, c, key, canGo) {
  const box = el('div', 'row crow' + (c.Finished ? ' done' : ''));
  box.dataset.key = key;
  const b = el('button', 'rowbtn');
  b.type = 'button';
  b.dataset.nav = key;
  b.appendChild(el('span', 'gut'));
  b.appendChild(el('span', 'dot'));
  b.appendChild(el('span', 'sign'));
  const title = el('span', 'title ctitle');
  title.appendChild(el('span', 'cnum quiet', String(c.N)));
  title.appendChild(text(c.Text));
  b.appendChild(title);
  const r = el('span', 'right');
  r.appendChild(word(c.Word) || el('span'));
  b.appendChild(r);
  box.appendChild(b);
  const d = el('div', 'cdetail');
  if (c.Images && c.Images.length) d.appendChild(thumbs(cx, c.Images));
  if (c.About) {
    const p = el('div', 'target');
    p.appendChild(el('span', 'quiet', '行: '));
    if (canGo) {
      const go = el('button', 'golink', c.About);
      go.type = 'button';
      go.dataset.go = String(c.N);
      p.appendChild(go);
    } else {
      p.appendChild(document.createTextNode(c.About));
    }
    d.appendChild(p);
  }
  // About already says the title it was written on, when that differs.
  if (c.TitleThen && !c.About) d.appendChild(el('div', 'quiet', '(書いたときは: ' + c.TitleThen + ')'));
  // Not sent: the panel says so once, over every comment (comments.js).
  const rep = reply(c);
  if (rep) d.appendChild(rep);
  else if (c.Note && c.Note !== cx.words('notSent')) d.appendChild(el('div', 'quiet', c.Note));
  box.appendChild(d);
  return box;
}

const onLine = (id) => ({ fixture: 'comments', pick: id, open: true });

export const covers = {"status": {"未送信": ["未送信, 3 lines, an image, a second one with a large and a tall image"], "送信済": ["送信済"], "既読": ["既読"], "対応中": ["対応中 (reply)"], "保留": ["保留 (reply)"], "対応済": ["対応済 (2-line reply)"]}, "settle": {"doing": ["対応中 (reply)"], "hold": ["保留 (reply)"], "done": ["対応済 (2-line reply)"]}};

export const differences = {"rights-below": ["narrow: the state word below, as its list"], "image-edge": ["未送信, 3 lines, an image, a second one with a large and a tall image"]};

export const variants = [
  { name: '未送信, 3 lines, an image, a second one with a large and a tall image', ...onLine('c1'), responsive: true },
  { name: '送信済', ...onLine('c2') },
  { name: '既読', ...onLine('c3') },
  { name: '対応中 (reply)', ...onLine('c4') },
  { name: '保留 (reply)', ...onLine('c5') },
  { name: '対応済 (2-line reply)', ...onLine('c6') },
  { name: 'written on another title', ...onLine('c7') },
  { name: 'on a Fold', ...onLine('f1') },
  { name: 'long unbroken', fixture: 'long', pick: 'url', open: true },
  { name: 'narrow: the state word below, as its list', fixture: 'long', pick: 'url', open: true, w: 390 },
];
