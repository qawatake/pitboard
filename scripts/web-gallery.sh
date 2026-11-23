#!/bin/sh
# web-gallery.sh [OUT]: shoots every variant of the browser page's gallery
# (/_gallery, docs/spec/web-states.md) to OUT/<module>/NN-name.png, a
# 390-wide dark sheet per module (OUT/<module>/sheet-390-dark.png), the
# other widths and themes of the variants flagged responsive or themed
# (NN-name-<w>-<theme>.png), and a contact sheet OUT/index.html.
#
# It runs a scratch server (its own state directory, --no-open, a free
# port) on a scratch board, and Playwright's own headless Chromium through
# playwright-cli (never the system's Chrome). Nothing is clicked through:
# the gallery draws each state from fixtures. Everything it starts is
# stopped at the end.
#
# Its scratch files go in $GALLERY_TMP (default $TMPDIR, else /tmp).
# Playwright keeps a Unix socket under $TMPDIR, whose path may be at most
# 104 bytes on macOS: a longer $TMPDIR is replaced by /tmp for it alone.
#
# It exits non-zero when a story threw, drew nothing, has something wider
# than its box (Code alone may scroll), or a module's variants do not show
# what it says it covers.
set -eu
TMPDIR=${TMPDIR:-}

root=$(cd "$(dirname "$0")/.." && pwd)
tmp=$(mktemp -d "${GALLERY_TMP:-${TMPDIR:-/tmp}}/pitboard-gallery.XXXXXX")
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
mkdir -p "$tmp/b/.pitboard/g" "$tmp/state"
printf '<Title>gallery</Title>\n' > "$tmp/b/.pitboard/g/report.md"
(cd "$tmp/b" && XDG_STATE_HOME="$tmp/state" PITBOARD_SEND=true exec "$tmp/pitboard" -s g serve --foreground --no-open --port 0 > "$tmp/serve.out" 2>&1) &
pid=$!
url=""
for _ in $(seq 50); do
	url=$(head -1 "$tmp/serve.out" 2>/dev/null || true)
	case $url in http://*) break ;; esac
	sleep 0.1
done
case $url in http://*) ;; *) echo "web-gallery: the server did not start:" >&2; cat "$tmp/serve.out" >&2; exit 2 ;; esac
base=${url%%/s/*}

cat > "$tmp/shoot.js" <<EOF
async (page) => {
  const base = '$base', out = '$out';
  await page.setViewportSize({ width: 1400, height: 900 });
  const load = async (q) => {
    await page.goto(base + '/_gallery' + q);
    await page.waitForFunction(() => window.__gallery && window.__gallery.ready, null, { timeout: 30000 });
    return page.evaluate(() => window.__gallery);
  };
  const problems = [];
  const shots = [];
  const g = await load('?w=1280&theme=light');
  problems.push(...g.errors.map((e) => ({ ...e, at: '1280 light' })));
  for (const s of g.stories) {
    await page.locator('#story-' + s.id).screenshot({ path: out + '/' + s.file });
    shots.push({ module: s.module, name: s.name, file: s.file, at: '1280 light' });
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
  return JSON.stringify({ shots, problems });
}
EOF
for m in $(cd "$root/internal/web/static" && ls parts layers | grep '\.js$' | sed 's/\.js$//') pages; do mkdir -p "$out/$m"; done
cd "$tmp"
pw -s="$sess" open --config .playwright/cli.config.json "$url" > /dev/null
pw -s="$sess" --raw run-code --filename "$tmp/shoot.js" > "$tmp/result.json"

node - "$tmp/result.json" "$out" <<'EOF'
const fs = require('fs');
const [file, out] = process.argv.slice(2);
let raw = fs.readFileSync(file, 'utf8').trim();
let r = JSON.parse(raw);
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
console.log(`${r.shots.length} images in ${out} (contact sheet: ${out}/index.html)`);
for (const p of r.problems) console.log(`PROBLEM ${p.at} ${p.id}: ${p.what}`);
process.exit(r.problems.length ? 1 : 0);
EOF
