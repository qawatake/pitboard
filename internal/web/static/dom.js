// Small helpers every part draws with. Report and comment text is data: it
// is only ever put in the page with textContent and created elements,
// never as HTML.

export const el = (tag, cls, text) => {
  const e = document.createElement(tag);
  if (cls) e.className = cls;
  if (text != null) e.textContent = text;
  return e;
};

export const isWeb = (u) => /^https?:\/\//i.test(u || '');

// spans draws inline text (page.Span): bold, code, links (a link only
// outside a row's button, and only to http(s)); "\n" breaks the line.
export function spans(parent, ss, inButton) {
  for (const s of ss || []) {
    let e;
    switch (s.Style) {
      case 'bold': e = el('strong'); break;
      case 'code': e = el('code'); break;
      case 'link':
        if (!inButton && isWeb(s.URL)) {
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

export const plain = (ss) => (ss || []).map((s) => s.Text).join('');

// word is a word in its tone's color (page.Word); state words never wrap.
export function word(w, extra) {
  if (!w || !w.Text) return null;
  return el('span', 'word ' + w.Tone + (extra ? ' ' + extra : ''), w.Text);
}

// keyLabel is a key and what it does, as the bar and the keys list write
// it: the key bold, then the word.
export function keyLabel(key, label) {
  const f = document.createDocumentFragment();
  f.append(el('kbd', '', key));
  if (label) f.append(' ' + label);
  return f;
}
