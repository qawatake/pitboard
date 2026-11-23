// Whole pages: the spacing between parts, and what only a whole page
// shows (where the bands meet what scrolls under them, a notice too tall
// to pin, the focus on the bar). Not a module of the page: the gallery's
// own, declared like one (docs/spec/web-states.md).

export const covers = {};

export const differences = {"unpinned": ["long notice open: unpinned (1280)", "long notice open: unpinned (390)"], "under-band": ["scrolled: lines under the pinned band"], "under-bar": ["scrolled: lines under the bar"], "at-bottom": ["scrolled to the bottom"], "focus-bar": ["Tab on a bar button: its focus ring"]};

export const variants = [
  { name: 'dense, all closed', ui: { cur: 'hook' }, responsive: true },
  { name: 'dense, several open', ui: { cur: 'hook' }, open: ['restart', 'hook', 'unit', 'web', 'olddiff', '決まったこと (3)'], responsive: true },
  { name: 'comments, open', fixture: 'comments', ui: { cur: 'c1' }, open: ['c1', 'c4', 'c6', 'f1'] },
  { name: 'long strings, open', fixture: 'long', ui: { cur: 'url' }, open: ['url', 'lask'], responsive: true },
  { name: 'hidden lines shown (H)', fixture: 'hidden', ui: { cur: 'v1', showHidden: true } },
  { name: 'long notice open: unpinned (1280)', fixture: 'long', ui: { cur: 'long' }, open: ['long'], w: 1280, h: 300 },
  { name: 'long notice open: unpinned (390)', fixture: 'long', ui: { cur: 'long' }, open: ['long'], w: 390, h: 560 },
  { name: 'scrolled: lines under the pinned band', ui: { cur: 'web' }, open: ['restart', 'web'], scroll: 'under-band' },
  { name: 'scrolled: lines under the bar', ui: { cur: 'hook' }, open: ['hook', 'web', 'olddiff'], scroll: 'under-bar' },
  { name: 'scrolled to the bottom', ui: { cur: 'deep' }, open: ['web', 'olddiff', '決まったこと (3)', '中の折りたたみ'], scroll: 'bottom' },
  { name: 'Tab on a bar button: its focus ring', ui: { cur: 'hook' }, focus: '.bar [data-act="jump"]' },
];
