#!/bin/sh
# web-gallery.sh [OUT]: shoots every variant of the browser page's gallery
# (/_gallery, docs/spec/web-states.md) to OUT/<module>/NN-name.png, a
# 390-wide dark sheet per module (OUT/<module>/sheet-390-dark.png), the
# other widths and themes of the variants flagged responsive or themed
# (NN-name-<w>-<theme>.png), and a contact sheet OUT/index.html.
#
# It runs a scratch server (its own state directory, --no-open, a free
# port, --foreground --gallery) on the boards the fixtures are made from
# (go test ./internal/web -run TestGalleryFixtures -boards DIR), and
# Playwright's own headless Chromium through playwright-cli (never the
# system's Chrome). Nothing is clicked through: the gallery draws each
# state from fixtures. Everything it starts is stopped at the end.
#
# Fidelity: every whole page (pages/*) and a few variants are also drawn
# LIVE: the same board, served for real, put in the same state by ?ui=
# (only a --gallery server takes it). The DOM of the two (without ids,
# keys, tokens, image sources and the cursor) must be the same: for a
# part, its lines must be found, in order, among the live page's.
#
# Its scratch files go in $GALLERY_TMP (default $TMPDIR, else /tmp).
# Playwright keeps a Unix socket under $TMPDIR, whose path may be at most
# 104 bytes on macOS: a longer $TMPDIR is replaced by /tmp for it alone.
#
# It exits non-zero when a story threw, drew nothing, has something wider
# than its box, a module's variants do not show what it says they show
# (covers, differences), or a story differs from the live page.
#
# OUT/report.json holds every story with what its DOM shows.
set -eu
TMPDIR=${TMPDIR:-}

root=$(cd "$(dirname "$0")/.." && pwd)
tmp=$(mktemp -d "${GALLERY_TMP:-${TMPDIR:-/tmp}}/pitboard-gallery.XXXXXX")
tmp=$(cd "$tmp" && pwd -P) # playwright-cli takes files under its real path only
pw() {
	if [ "${#TMPDIR}" -gt 40 ]; then TMPDIR=/tmp playwright-cli "$@"; else playwright-cli "$@"; fi
}
out=${1:-$tmp/shots}
mkdir -p "$out"
out=$(cd "$out" && pwd)
sess="gallery$$"
pid=""
cleanup() {
	[ -n "$pid" ] && kill "$pid" 2>/dev/null || true
	(cd "$tmp" && pw -s="$sess" close >/dev/null 2>&1) || true
	rm -rf "$tmp/b" "$tmp/state" "$tmp/pitboard"
}
trap cleanup EXIT INT TERM

command -v playwright-cli >/dev/null || { echo "web-gallery: playwright-cli is not on PATH" >&2; exit 2; }
# Playwright's own headless Chromium (the newest installed).
cache=${PLAYWRIGHT_BROWSERS_PATH:-}
for c in "$HOME/Library/Caches/ms-playwright" "$HOME/.cache/ms-playwright"; do
	[ -z "$cache" ] && [ -d "$c" ] && cache=$c
done
shell=$(ls -d "$cache"/chromium_headless_shell-*/chrome-headless-shell-*/chrome-headless-shell 2>/dev/null | sort -V | tail -1 || true)
[ -x "$shell" ] || { echo "web-gallery: no Playwright headless shell under $cache (playwright-cli install-browser)" >&2; exit 2; }
mkdir -p "$tmp/.playwright"
printf '{"browser":{"browserName":"chromium","launchOptions":{"executablePath":"%s","headless":true}}}\n' "$shell" > "$tmp/.playwright/cli.config.json"

(cd "$root" && go build -o "$tmp/pitboard" ./cmd/pitboard)
mkdir -p "$tmp/b" "$tmp/state"
(cd "$root" && go test ./internal/web -run '^TestGalleryFixtures$' -count=1 -boards "$tmp/b" > "$tmp/fixtures.out" 2>&1) || { echo "web-gallery: the fixtures' boards:" >&2; cat "$tmp/fixtures.out" >&2; exit 2; }
(cd "$tmp/b/dense" && TZ=Asia/Tokyo XDG_STATE_HOME="$tmp/state" PITBOARD_SEND=true exec "$tmp/pitboard" -s gallery1 serve --foreground --gallery --no-open --port 0 > "$tmp/serve.out" 2>&1) &
pid=$!
url=""
for _ in $(seq 50); do
	url=$(head -1 "$tmp/serve.out" 2>/dev/null || true)
	case $url in http://*) break ;; esac
	sleep 0.1
done
case $url in http://*) ;; *) echo "web-gallery: the server did not start:" >&2; cat "$tmp/serve.out" >&2; exit 2 ;; esac
base=${url%%/s/*}
# Each situation's board, live: its URL.
boards="{"
for d in "$tmp"/b/*/; do
	n=$(basename "$d")
	u=$(cd "$d" && XDG_STATE_HOME="$tmp/state" "$tmp/pitboard" -s gallery1 serve --no-open 2>/dev/null | head -1)
	case $u in http://*) boards="$boards\"$n\":\"${u%%\?*}\"," ;; esac
done
boards="${boards%,}}"

cat > "$tmp/shoot.js" <<EOF
async (page) => {
  const base = '$base', out = '$out';
  await page.setViewportSize({ width: 1400, height: 900 });
  const said = [];
  page.on('pageerror', (e) => said.push(String(e && e.stack || e)));
  page.on('console', (m) => { if (m.type() === 'error') said.push(m.text()); });
  const load = async (q) => {
    await page.goto(base + '/_gallery' + q);
    await page.waitForFunction(() => window.__gallery && window.__gallery.ready, null, { timeout: 30000 })
      .catch((e) => { throw new Error('the gallery did not finish (' + q + '): ' + said.join(' | ')); });
    return page.evaluate(() => window.__gallery);
  };
  const problems = [];
  const shots = [];
  const fidelity = [];
  const g = await load('?w=1280&theme=light');
  problems.push(...g.errors.map((e) => ({ ...e, at: '1280 light' })));
  for (const s of g.stories) {
    await page.locator('#story-' + s.id).screenshot({ path: out + '/' + s.file });
    shots.push({ module: s.module, name: s.name, file: s.file, at: '1280 light' });
  }
  // A story with a focus is shot with it (the keyboard's).
  for (const s of g.stories.filter((s) => s.focus)) {
    await page.keyboard.press('Shift');
    await page.focus('#story-' + s.id + ' ' + s.focus);
    await page.locator('#story-' + s.id).screenshot({ path: out + '/' + s.file });
  }
  // The cursor on each part's first variant.
  const gc = await load('?w=1280&theme=light&cur=1');
  problems.push(...gc.errors.map((e) => ({ ...e, at: '1280 light cur' })));
  for (const s of gc.stories.filter((s) => s.kind === 'part' && s.first)) {
    const file = s.file.replace(/\.png$/, '-cur.png');
    await page.locator('#story-' + s.id).screenshot({ path: out + '/' + file });
    shots.push({ module: s.module, name: s.name + ' (cursor)', file, at: '1280 light' });
  }
  const modules = [...new Set(g.stories.map((s) => s.module))];
  const g2 = await load('?w=390&theme=dark');
  problems.push(...g2.errors.map((e) => ({ ...e, at: '390 dark' })));
  for (const m of modules) {
    const file = m + '/sheet-390-dark.png';
    await page.locator('#module-' + m).screenshot({ path: out + '/' + file });
    shots.push({ module: m, name: 'all variants', file, at: '390 dark' });
  }
  for (const s of g.stories.filter((s) => s.flags)) {
    for (const [w, theme] of [[1280, 'dark'], [390, 'light'], [390, 'dark']]) {
      const nn = s.id.slice(s.module.length + 1);
      const gx = await load('?w=' + w + '&theme=' + theme + '&only=' + s.module + '/' + nn);
      problems.push(...gx.errors.map((e) => ({ ...e, at: w + ' ' + theme })));
      const file = s.file.replace(/\.png$/, '-' + w + '-' + theme + '.png');
      await page.locator('#story-' + s.id).screenshot({ path: out + '/' + file });
      shots.push({ module: s.module, name: s.name, file, at: w + ' ' + theme });
    }
  }
  // Fidelity: the same state drawn live.
  const boards = $boards;
  const serialize = (sel, part) => {
    const keep = ['hidden', 'aria-expanded', 'data-act', 'data-conn', 'style', 'disabled'];
    const ser = (n) => {
      if (n.nodeType === 3) return n.textContent;
      if (n.nodeType !== 1) return '';
      if (n.classList.contains('sr')) return '';
      const cls = [...n.classList].filter((c) => !['cur', 'light', 'dark', 'part', 'live'].includes(c)).sort().join('.');
      const at = keep.filter((a) => n.hasAttribute(a)).map((a) => a + '=' + n.getAttribute(a).replace(/--(foot-h|jw)[^;]*;?/g, '').trim()).filter((x) => !x.endsWith('=')).join(',');
      return '<' + n.tagName.toLowerCase() + (cls ? '.' + cls : '') + (at ? '[' + at + ']' : '') + '>' + [...n.childNodes].map(ser).join('') + '</>';
    };
    const root = sel();
    if (part) return { lists: [...root.querySelectorAll('.board, .notices, .kids')].map((l) => [...l.children].map(ser)) };
    return ['.scroller', '.foot', '.cpanel', '.help', '.viewer', '.editor.loose'].map((s) => { const e = root.querySelector(s); return s + ':' + (e ? ser(e) : '-'); });
  };
  const FID = new Set(['section-02', 'fold-01', 'fold-02', 'item-02', 'item-04', 'ask-02', 'notice-02', 'comment-04', 'link-02', 'editor-01', 'editor-05', 'editor-06', 'comments-01', 'bar-03', 'banners-01', 'goto-01', 'help-01']);
  const fid = g.stories.filter((s) => s.ui && (s.module === 'pages' || FID.has(s.id)));
  for (const s of fid) {
    await page.goto(base + '/_gallery?w=1280&theme=light&only=' + s.module + '/' + s.nn);
    await page.waitForFunction(() => window.__gallery && window.__gallery.ready, null, { timeout: 30000 });
    const part = s.kind === 'part';
    const want = await page.evaluate(([id, part, f]) => (0, eval)(f)(() => document.querySelector('#story-' + id + ' .app'), part), [s.id, part, '(' + serialize.toString() + ')']);
    if (!boards[s.fixture]) { problems.push({ id: s.id, what: 'fidelity: no live board for ' + s.fixture, at: 'live' }); continue; }
    await page.setViewportSize({ width: s.w, height: s.h });
    await page.goto(boards[s.fixture] + '?ui=' + encodeURIComponent(JSON.stringify(s.ui)));
    await page.waitForFunction(() => document.body.dataset.ui === 'set' && document.querySelector('.board') && document.querySelector('.board').childElementCount > 0, null, { timeout: 15000 }).catch(() => {});
    await page.waitForTimeout(700);
    const got = await page.evaluate(([part, f]) => (0, eval)(f)(() => document.querySelector('.app'), part), [part, '(' + serialize.toString() + ')']);
    let ok;
    if (part) {
      const w = want.lists.find((l) => l.length) || [];
      const flat = got.lists;
      ok = flat.some((l) => { for (let i = 0; i + w.length <= l.length; i++) if (w.every((x, j) => l[i + j] === x)) return true; return false; });
      if (!ok) problems.push({ id: s.id, what: 'differs from the live page: ' + JSON.stringify(w).slice(0, 600), at: 'live' });
    } else {
      for (let i = 0; i < want.length; i++) {
        if (want[i] !== got[i]) {
          let k = 0; while (k < want[i].length && want[i][k] === got[i][k]) k++;
          problems.push({ id: s.id, what: 'differs from the live page at ' + want[i].slice(0, want[i].indexOf(':')) + ': gallery …' + want[i].slice(Math.max(0, k - 120), k + 160) + ' / live …' + (got[i] || '').slice(Math.max(0, k - 120), k + 160), at: 'live' });
        }
      }
    }
    fidelity.push(s.id);
    await page.setViewportSize({ width: 1400, height: 900 });
  }
  return JSON.stringify({ shots, problems, fidelity, stories: g.stories });
}
EOF
for m in $(cd "$root/internal/web/static" && ls parts layers | grep '\.js$' | sed 's/\.js$//') pages; do mkdir -p "$out/$m"; done
cd "$tmp"; cp "$tmp/shoot.js" "${GALLERY_KEEP:-/dev/null}" 2>/dev/null || true
pw -s="$sess" open --config .playwright/cli.config.json "$url" > /dev/null
pw -s="$sess" --raw run-code --filename "$tmp/shoot.js" > "$tmp/result.json"

node - "$tmp/result.json" "$out" <<'EOF'
const fs = require('fs');
const [file, out] = process.argv.slice(2);
let raw = fs.readFileSync(file, 'utf8').trim();
let r;
try { r = JSON.parse(raw); } catch { console.error('web-gallery: the browser said:\n' + raw.slice(0, 4000)); process.exit(2); }
if (typeof r === 'string') r = JSON.parse(r);
const esc = (s) => String(s).replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));
const by = {};
for (const s of r.shots) (by[s.module] = by[s.module] || []).push(s);
let html = '<!doctype html><meta charset="utf-8"><title>pitboard gallery</title>' +
  '<style>body{font:13px system-ui;margin:1rem;background:#eee}figure{display:inline-block;vertical-align:top;margin:.5rem;background:#fff;padding:.4rem}img{max-width:420px;display:block}h2{font-family:monospace}.bad{color:#b3261e}</style>' +
  `<h1>pitboard gallery: ${r.shots.length} images, ${r.problems.length} problems</h1>`;
for (const p of r.problems) html += `<p class="bad">${esc(p.at)} ${esc(p.id)}: ${esc(p.what)}</p>`;
for (const [m, ss] of Object.entries(by)) {
  html += `<h2>${esc(m)}</h2>`;
  for (const s of ss) html += `<figure><a href="${esc(s.file)}"><img src="${esc(s.file)}" loading="lazy"></a><figcaption>${esc(s.at)} · ${esc(s.name)}</figcaption></figure>`;
}
fs.writeFileSync(out + '/index.html', html);
fs.writeFileSync(out + '/report.json', JSON.stringify({ problems: r.problems, fidelity: r.fidelity, stories: r.stories }, null, 1));
console.log(`fidelity: ${r.fidelity.length} stories compared with the live page`);
console.log(`${r.shots.length} images in ${out} (contact sheet: ${out}/index.html)`);
for (const p of r.problems) console.log(`PROBLEM ${p.at} ${p.id}: ${p.what}`);
process.exit(r.problems.length ? 1 : 0);
EOF
