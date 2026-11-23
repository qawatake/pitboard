// What a key means on the page now: key -> intent. Pure: it reads the
// event's key and target, uiState and the page, and says what to do; the
// doing is main.js's. null: the key is not the page's (the browser's own,
// or text being written).

import { cursorComment, index } from './state.js';

// imeFree makes keys work with a Japanese input method left on: full-width
// letters and symbols become their ASCII keys.
export function imeFree(k) {
  if (k.length === 1) {
    const c = k.charCodeAt(0);
    if (c >= 0xff01 && c <= 0xff5e) return String.fromCharCode(c - 0xfee0);
    if (c === 0x3000) return ' ';
  }
  return k;
}

// intentOf is what key event e does. An intent is {do, ...}; trap: Tab
// stays in the dialog that is open (its selector).
export function intentOf(e, ui, page) {
  const tg = e.target || {};
  const k = imeFree(e.key);
  if (ui.mode === 'image') {
    if (k === 'Escape') return { do: 'closeImage' };
    if (k === 'Tab') return { do: 'trap', in: '.viewer' };
    return null;
  }
  if (ui.mode === 'help') {
    if (k === 'Escape' || k === '?' || k === 'q') return { do: 'closeHelp' };
    if (k === 'Tab') return { do: 'trap', in: '.help' };
    return null;
  }
  const writing = tg.tagName === 'TEXTAREA' || tg.tagName === 'INPUT' || tg.isContentEditable;
  if (ui.panel && k === 'Tab') return { do: 'trap', in: '.cpanel' };
  if (ui.panel && k === 'Escape' && !writing) return { do: 'closePanel' };
  if (writing) return null; // the editor's own keys (editor.js)
  if (e.ctrlKey || e.metaKey || e.altKey || e.isComposing) return null;
  if (!page) return null;
  const onButton = tg.tagName === 'BUTTON' && !(tg.dataset && tg.dataset.nav);
  if (ui.mode === 'goto') return { do: 'goto', key: k };
  switch (k) {
    case 'j': case 'ArrowDown': return { do: 'move', d: 1 };
    case 'k': case 'ArrowUp': return { do: 'move', d: -1 };
    case 'Enter':
      if (onButton || (tg.dataset && tg.dataset.nav)) return null; // the button's own
      return ui.cur ? { do: 'toggle' } : null;
    case ' ':
      if (onButton) return null;
      return ui.panel ? { do: 'none' } : { do: 'next' };
    case '?': return { do: 'help' };
    case 'u': return { do: 'undo' };
    case 's': return { do: 'send' };
    case 'm': return { do: 'write' };
    case 'H': return ui.panel ? null : { do: 'showHidden' };
    case 'A': return ui.panel ? null : { do: 'readAll' };
    case 'g': return ui.panel || !(page.JumpTo || []).length ? null : { do: 'startGoto' };
    case 'Escape': return null;
  }
  const c = cursorComment(page, ui);
  if (c) {
    if (k === 'e') return { do: 'editComment', n: c.N };
    if (k === 'x') return { do: 'retract', n: c.N };
    return null;
  }
  if (ui.panel || !index(page).things.has(ui.cur) || k.length !== 1 || !/[a-zA-Z]/.test(k)) return null;
  // Any other letter goes to the server's guard: it says what it does on
  // this line (or why it does nothing).
  return { do: 'press', key: k };
}
