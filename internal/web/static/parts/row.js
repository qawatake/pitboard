// The frame of a line one can point at (a Stop): the gutter (cursor, mark,
// sign) hanging left of the text edge, the title on it, the words at the
// right end. Each kind's module draws what it adds (body, options, target,
// kids) through row(). Not a part of its own: it has no variants (the
// marks, signs and right words are shown by the parts that have them).

import { el, spans, word } from '../dom.js';

// markOf is the mark column: ● waiting (yellow) or unread (green).
export function markOf(t) {
  const m = el('span', 'dot');
  if (t.Mark === 'waiting' || t.Mark === 'new') {
    m.classList.add(t.Mark);
    m.textContent = '●';
    m.appendChild(el('span', 'sr', t.Mark === 'waiting' ? '反応待ち' : '未読'));
  }
  return m;
}

// signOf draws a line's sign, in one box for every sign (same width,
// centered). The signs of a line that opens (page's SignClosed, SignOpen)
// are drawn as triangles, the same size in every font; the others are
// their glyph.
export function signOf(cx, sign) {
  const s = el('span', 'sign');
  if (sign && (sign === cx.words('signClosed') || sign === cx.words('signOpen'))) {
    s.classList.add('tri', sign === cx.words('signOpen') ? 'is-open' : 'is-closed');
    s.appendChild(el('i'));
  } else {
    s.appendChild(el('span', 'glyph', sign || ''));
  }
  return s;
}

// rightOf is the right end (page.Right): the answer, the state of the
// latest input, then the AI's status (or, on a notice, when it went up).
export function rightOf(t, open) {
  const r = t.Right || {};
  const box = el('span', 'right');
  if (r.Reaction) {
    box.appendChild(el('span', 'optkey', r.Reaction));
    if (t.Kind === 'ask' && r.Option && !open) box.appendChild(el('span', 'quiet ohead', r.Option));
  }
  if (r.Input && r.Input.Text) {
    const w = el('span', 'word');
    if (r.Count > 0) w.appendChild(el('span', 'quiet', r.Count + ' '));
    w.appendChild(word(r.Input));
    box.appendChild(w);
  }
  if (r.AtText) box.appendChild(el('span', 'quiet word', r.AtText));
  else if (r.Status && r.Status.Text) box.appendChild(word(r.Status));
  return box;
}

// patch makes parent's children kids, moving only what is not already in
// place: rows kept from the last drawing stay where they are.
export function patch(parent, kids) {
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

// row draws line t: its frame, then add(box, open) for what its kind adds
// (add returns the element its kids go in, if any). A row whose line says
// the same as last time is not drawn again (cx.memo keeps it by key).
export function row(cx, t, opts, add) {
  const open = !!(t.CanOpen && cx.isOpen(t));
  const jump = cx.ui.mode === 'goto' && t.JumpKey ? t.JumpKey : '';
  const { Kids, ...own } = t;
  const sig = JSON.stringify(own) + '|' + open + '|' + jump + '|' + (opts.heading ? 'h' : '');
  let c = cx.memo.cache.get(t.Key);
  if (!c || c.sig !== sig) {
    const box = frame(cx, t, open, opts, jump);
    // The lines of text drawn in it (its body, its nested lines) are
    // kept with it: drawn again from the cache, they are still lines one
    // points at.
    const outer = cx.collect;
    cx.collect = [];
    const kids = add ? add(box, open) || null : null;
    c = { sig, box, kids, lines: cx.collect };
    cx.collect = outer;
    cx.memo.cache.set(t.Key, c);
  } else {
    for (const [lt, lb] of c.lines || []) cx.rows.set(lt.Key, lb);
  }
  if (cx.collect) cx.collect.push(...(c.lines || []));
  if (c.kids) patch(c.kids, cx.things(t.Kids));
  // While g waits, the cursor line's links show their digits.
  c.box.classList.toggle('nums', cx.ui.mode === 'goto' && cx.ui.cur === t.Key && !!(t.Links || []).length);
  cx.rows.set(t.Key, c.box);
  return c.box;
}

function frame(cx, t, open, opts, jump) {
  const box = el('div', 'row k-' + t.Kind);
  box.dataset.key = t.Key;
  if (t.State) box.classList.add('st-' + t.State);
  if (t.State === 'done') box.classList.add('done');
  if (t.Hidden) box.classList.add('hidden');
  if (opts.heading && !t.Hidden) box.classList.add('heading');
  if (open) box.classList.add('open');
  // How deep in Folds: the row's cursor and mark stay in the outermost
  // gutter, its sign hangs left of its own text edge.
  if (t.Depth) box.style.setProperty('--ind', String(t.Depth));
  if (t.Jump) box.dataset.jump = String(t.Jump);
  const b = el('button', 'rowbtn');
  b.type = 'button';
  b.dataset.nav = t.Key;
  if (t.CanOpen) b.setAttribute('aria-expanded', String(open));
  // g's letter goes in the cursor's column: nothing moves.
  b.appendChild(jump ? el('span', 'gut', jump) : el('span', 'gut'));
  if (jump) b.firstChild.classList.add('jumpkey');
  b.appendChild(markOf(t));
  b.appendChild(signOf(cx, open ? t.OpenSign : t.Sign));
  const title = el('span', 'title');
  // A head word (page.HeadWord): run-in, or a definition when it does not
  // fit on one line (render.fit), as a line of text is.
  if (t.HeadWord) {
    title.classList.add('hasHead');
    title.append(el('strong', 'hw', t.HeadWord), el('span', 'hsep', ': '));
    title.appendChild(spans(el('span', 'hb'), t.Title, true));
  } else spans(title, t.Title, true);
  if (t.Hidden) title.appendChild(el('span', 'sr', ' (隠した行)'));
  b.appendChild(title);
  b.appendChild(rightOf(t, open));
  box.appendChild(b);
  return box;
}

// findThing is the line of page p whose id (Target), key or title is name
// (for the gallery's variants).
export function findThing(p, name) {
  let found = null;
  const walk = (ts) => {
    for (const t of ts || []) {
      if (found) return;
      const title = (t.Title || []).map((s) => s.Text).join('');
      if (t.Target === name || t.Key === name || title === name) { found = t; return; }
      walk(t.Kids);
    }
  };
  walk(p.Notices);
  walk(p.Things);
  return found;
}
