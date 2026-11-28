// The keys (?): a dialog over everything (the pinned band and the bar
// too), its backdrop over the whole screen. It opens at its top, its
// heading in view and focused; keys in a column of their own (mono), what
// they do in the normal color (only notes faint). At most 85% of the
// screen high, scrolling inside; a full sheet on a phone.

import { el } from '../dom.js';

const HELP = [
  ['j k', '上下の行へ (日本語入力のままでも全角の ｊ ｋ が効く)。Tab やクリックで行に移っても同じ'],
  ['↑ ↓', 'ページだけを動かす (カーソルは動かない。端末と同じ。ホイール・トラックパッド・pgup pgdn も)'],
  ['enter', '開く・閉じる (行をクリックしても同じ)'],
  ['space', 'カーソルより下で、いちばん近い印のある行 ({need}・人待ちの項目・{move}) へ。下に無ければ上から。折りたたみは開く'],
  ['g', '節へ飛ぶ: お知らせ・節・いちばん外の折りたたみに a b c … が付く。その字でそこへ。esc でやめる'],
  ['g 1 … 9', 'カーソルの行の中のリンク (別の行を指すもの) に、出てくる順に数字が付く。その数字で指す行へ (折りたたみは開く)。リンクはクリックや enter でも行ける'],
  ['g -', '戻る: 飛ぶ前の行へ (飛んだあとはバーの「← 戻る」でも)'],
  ['a b …', '問いに答える (問いを開いているときだけ。閉じていれば開く)'],
  ['r', '読んだ: {move} (未読) を消す'],
  ['A', 'いま報告にある行をすべて読んだことにする'],
  ['c', 'この行にコメントを書く'],
  ['m', '宛先なしでコメントを書く'],
  ['e', 'コメントを直す (コメントの欄で)'],
  ['x', 'この行を隠す (隠した行の上では表示に戻す)。コメントの欄では、そのコメントを取り消す'],
  ['H', '隠した行を表示する / しない'],
  ['u', '元に戻す: 直前の操作を取り消す (このタブでした操作を、新しい順に 1 つずつ)'],
  ['s', 'AI に送る: 未送信のものをまとめて送る'],
  ['l', 'コメントの欄を開く / 閉じる (下のバーの「コメント N」と同じ)'],
  ['?', 'このキーの一覧'],
  ['', '{note}端末の l (コメントの画面) は、ブラウザでは浮いた欄。AI がまだ済ませていないもの、済んだものの順 (それぞれ新しい順) に出る。enter かクリックで、そのコメントの行へ。esc か欄の外のクリックで閉じる。tab キーはボタンと行の間を動く'],
  ['書いているとき', null],
  ['enter', 'pitboard に置く (s で送るまで AI には届かない)'],
  ['shift+enter', '改行'],
  ['esc', 'やめる (書いたものはこのタブに残り、次に開くと戻る)'],
  ['alt+↑ ↓', '書きながらページを半画面ずつ上 / 下へ (入力欄から出ずに、欄の下に隠れた行を読む)'],
  ['画像', '貼り付け・ドロップで [Image #N] が入る。文の下の見本の × で文から外す'],
];

// desc is a description, its ● samples in their own colors.
function desc(text) {
  const dd = el('dd');
  let s = text;
  if (s.startsWith('{note}')) { dd.className = 'quiet'; s = s.slice(6); }
  for (const part of s.split(/(\{need\}|\{move\})/)) {
    if (part === '{need}') dd.append(el('span', 'need', '黄の ●'));
    else if (part === '{move}') dd.append(el('span', 'move', '緑の ●'));
    else dd.append(part);
  }
  return dd;
}

export function draw(cx, help) {
  help.hidden = cx.ui.mode !== 'help';
  const dl = help.querySelector('dl');
  if (!dl.firstChild) {
    for (const [k, d] of HELP) {
      if (d === null) { dl.appendChild(el('h3', '', k)); continue; }
      dl.append(el('dt', '', k), desc(d));
    }
    // The colors, each with its one meaning (page.Palette), each in its
    // own color: the same words as SPEC and the terminal's list.
    const pal = (cx.page && cx.page.Palette) || [];
    if (pal.length) {
      dl.appendChild(el('h3', '', '色'));
      for (const c of pal) {
        const [hue, meaning] = c.Label.split(': ');
        const dt = el('dt', c.Name, '● ' + hue);
        dt.dataset.color = c.Name;
        dl.append(dt, el('dd', '', meaning));
      }
    }
  }
}

export const covers = {"mode": {"help": ["the keys, over the pinned band and the bar"]}};

export const differences = {"help-scrolled": ["scrolled to its end"], "float-edge": ["the keys, over the pinned band and the bar"]};

export const variants = [
  { name: 'the keys, over the pinned band and the bar', ui: { cur: 'restart', mode: 'help' }, responsive: true },
  { name: 'scrolled to its end', ui: { cur: 'restart', mode: 'help' }, sheetScroll: 'bottom' },
  { name: 'a tall screen: all of it', ui: { cur: 'restart', mode: 'help' }, h: 1500 },
];
