// An image shown large, over everything (from a comment's thumbnail).

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

export const covers = {"mode": ["image"]};

export const variants = [
  { name: 'an image, large', fixture: 'comments', ui: { cur: 'c1', open: { c1: true }, mode: 'image', image: 1 } },
];
