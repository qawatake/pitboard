# FIX9 結果

## 直した項目

- a1: SKILL.md の description から「板」を消し、`Keeps a pitboard for the current session` に
- a2〜a7: SPEC.md の `(仮称)`・`ita`・`済`→`対応済`・「受け取った」(作法の節ごと消えた)・緑の「新しい」→「読んでほしい」。SPEC-format の例の題を `pitboard を作る` に
- a8〜a10: `-h` の settle の説明、`edit thought` の説明、Thoughts の節から destination をなくした (`settle every thought` / `Never leave one unsettled`)
- a11・b1: hook の出力を `settle N [--doing|--hold|--done] TEXT" (several: --STATE N TEXT --STATE N TEXT ...) once you know what to do with it.` に。SPEC.md の hook の節も
- b2・d4・g1: `viewerKeys` に `u` を足した。`-h` の禁止キーは `viewerKeys` から組み立てる (`usage` は `usageText` の `{viewerKeys}` を置き換えた var)。SPEC-format の一覧を正とし (コードの `viewerKeys` が元)、SPEC.md は参照に
- g (テスト): `fix9_test.go` の `TestViewerKeysMatchTheViewer`。a〜z のうち `viewerKeys` に無い字は Ask の答えとして記録されること、ある字は `check` が弾くことを確かめる。viewer がキーを足すと前者で落ちる
- b3: SPEC.md の外のエディタを `ctrl+g` に
- b4・c4: SPEC.md の置き場所・起動時の文に `<session>/`
- b5・c5: SPEC.md の `^id` の書式と部品の表を消し、SPEC-format.md への参照に。カーソルの決まりは「報告の画面のカーソル」の節に残した
- b6・f2: 「今回やらないこと」を消し、「作り」は仕様の 5 行だけに (module 名・`sketch/`・git・inbox・テストの行を消した)
- b7・c7: SPEC-format の存在しないセッションの文言を `セッション abx の報告はまだありません` に
- b8・c8: 案内に出さないのは `tab` `q` `H`。`s` `u` `space` は使えるときだけ出る、と
- b9・b10・c6: 一覧の案内の行 (158) を消し、255 に `m 書く` を足した
- b11・f1・e1〜e4: README を人向けに書き直した (何であるか、入れる、開く、`?`、届かないとき (`PITBOARD_SEND` / hook)、画像の貼り付け)。AI の仕事の説明は `-h` に任せた
- b12: 置かれたカーソルで効くキーに `r` を足した
- b13: 「作り」の整理で消えた
- b14: `-h` の Fold の `id` の説明
- c1: SPEC.md の wait 節の順番の行を「届く道は 3 つ、`s` が起こすのは wait か打ち込み」に。`-h` の Send の 2. に `(s does not wake you through them; ...)`
- c2: README の skill の説明を「`-h` を読む、と要点数行だけ」に
- c3・d6: SPEC.md の「`-h` と skill にも書く」を「`-h` にも書く」に (2 か所)
- c9: SPEC.md のコメントの状態の壊れた括弧
- d1: `-h` の Marks の消え方を 1 文に。README は「読んだら `r`」だけ
- d2: SPEC.md の「送る」の状態の語と、欄の右端の状態を、「コメント」の状態への参照に
- d5・f3: SPEC.md の作法の節を「正は `-h` の Manners」の 1 行に
- d7: events の `undo` の対象の列挙を「u で取り消す」への参照に
- e6・e7・f5: `?` の一覧: `q` に `ctrl+c`、`r` に「(右上の「未読」)」、`x (コメント)` `e (コメント)` を `tab` の下へ
- e8: SPEC.md のコメントの欄で効くキーに `space` `A` `u` (一覧では `space` `A` は効かない)
- e: `sketch/pv` `BRIEF` への言及 (SPEC.md・SPEC-format.md) と、SPEC-format の「置き換える」「理由」の経緯を消した
- まとめの 2: hook の文面 (上の a11)。コードのコメントの destination も直した (`view.go` `store.go`)
- 2 (作法): `-h` の Manners に「題は 1 文で言い切る、細かいことは中身に」「status は記号で言えないことだけ」「済んだ行は Done の Fold に」を足した。勧める構成の 4. 7. と例 (`state="doing"` から `status="working"` を消し、`status="queued"` の行を足した) も合わせた。SPEC-format の例の `status="作業中"` も消した
- 2 (取り下げ): 題の長さの警告は一度実装したが、指示の書き換えに従ってすべて消した (`check`・`problemNote`・テスト・SPEC)

## 推測で決めた点

- a12 (英語の地の文の `thought`) と a13 (`the board`) は FIX9 に決定が無かったので変えなかった。Thoughts の節の頭を `(The board calls them comments; commands and their output say thought.)` にしただけ
- README の「届かないとき」は、`PITBOARD_SEND` (止まっている AI を起こす) と hook (作業中に届ける、`s` では起こさない) に役割を分けて書いた
- SPEC.md の冒頭の「人が書いたことは、AI が区切りを通るときに届く」を「人が送ったときに AI に届く」に直した (いまの送る仕組みに合わせた)
- `paneOrder` のコメントは、実装に合わせて「対応済でないもの → 対応済」と書き直した

## 未確認の点

- viewer を実際の端末で開いての `?` の一覧の見た目は見ていない (テストと `helpRows` の文字列だけ)
