# WEB4 の結果

vet / test (`-count=1`、e2e 込み) / build 通過。git には触っていない。サーバーは `serve --stop` で止め、自分の playwright-cli のブラウザは閉じた (`playwright-cli list` に残る 1 つは別セッション d1c37de6 のもので触っていない)。

- 1〜3・7 (`app.css`): 左端は 1 本。`#top #board #pane #status #bar` の左の余白に `--gw` (= `>` `●` 印の列) を取り、行の箱は `--hang` だけ左に広げてガターを含める (`content-visibility` の切り取りで印が消えないように)。入れ子は `.body .option .cdetail .kids` が 1 段 (`--step` 1.5em) 右。行に `--ind` (= `Depth`、`app.js`) を付け、`>` `●` は外側の列のまま、印だけ自分の左端の左。390px では列を細くする
- 4: 行の枠・点線を外し (`.rowbtn` / `#board` の focus)、`>` は `--focus` 色の太字。点線はバーのボタン・入力欄・ダイアログのボタン・リンクだけ
- 5: 節の見出し・最上位 Fold の上は同じ間 (1.6rem、margin の相殺)、行は `margin-block .25rem`、題の line-height 1.4
- 6: `page.EmptySection` / `Thing.None` (`MsgNone` = 「なし」)。web は見出しの下に薄く、端末 (`viewer/rows.go`) も薄く 1 行。SPEC-format・SPEC (ブラウザ) を更新。テスト: `TestEmptySectionNone`、`TestEmptySectionSaysNone`

計測 (最初の字の `getBoundingClientRect().left`、px。Notice 開・Ask 開・Fold 開 (段落・箇条・Item・入れ子 Fold 開)・Fold 閉):
- 1280: 最上位 (h1・節見出し 2・なし・段落・Item 2・Ask・Fold 2・Notice) すべて 316 / 1 段 (Fold の段落・箇条・Item・Fold、Notice の中身、Ask の選択肢) すべて 340 / 2 段 364。`>` `●` は全深さ 260・276、印 294・318・342。右端の語 全深さ 1020。2 行の題の高さ 45 (行の高さ 22.4)
- 390: 最上位すべて 62.4 / 1 段 86.4 / 2 段 110.4。右端 374。横スクロール無し
- 見出しの上の間 (前の要素の下 → 見出しの字): 4 つとも 27。キーボードで動かした行の outline は none、`>` は rgb(26,95,180)
- バー・エディタ・コメント欄の見出しと行の題も 316

スクリーンショット: `/private/tmp/claude-501/-Users-qawatake-dev-github-com-qawatake-pitboard/6a486f6d-0dd6-467b-8c16-0da65c1a57b4/scratchpad/w4/shots/` の `web4-{1280,390}-{light,dark}.png`、`web4-1280-light-editor.png`

推測で決めたこと: 箇条の行は記号 (`·`) をガターの印として扱い、文を左端に。390px でも右端の語は題の右に置く (前の「題の下に回す」をやめた)。中身の無い判定は Notice 以外の子が無い Section。
