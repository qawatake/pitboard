// An image shown large, over everything (from a comment's thumbnail): on
// the scrim, with an edge and the page's color behind it, so a white
// screenshot does not melt into the page.

export function draw(cx, viewer) {
  const on = cx.ui.mode === 'image' && cx.ui.image > 0;
  viewer.hidden = !on;
  const img = viewer.querySelector('img');
  if (on) {
    const src = cx.env.imageURL(cx.ui.image);
    if (img.getAttribute('src') !== src) img.src = src;
    img.alt = `[Image #${cx.ui.image}]`;
  } else {
    img.removeAttribute('src');
  }
}

export const covers = {"mode": {"image": ["a small image"]}};

export const differences = {"image-large": ["a large image, scaled to the screen"], "float-edge": ["a small image", "a white image"]};

export const variants = [
  { name: 'a small image', fixture: 'comments', ui: { cur: 'c1', open: { c1: true }, mode: 'image', image: 1 } },
  { name: 'a large image, scaled to the screen', fixture: 'comments', ui: { cur: 'c1', open: { c1: true }, mode: 'image', image: 2 } },
  { name: 'a white image', fixture: 'comments', ui: { cur: 'c1', open: { c1: true }, mode: 'image', image: 4 }, responsive: true },
];
