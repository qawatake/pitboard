# FIX19 の結果

`scripts/check.sh` 通過 (ギャラリー 190 枚、問題 0、生きたページとの比較 28 件): `/private/tmp/claude-501/-Users-qawatake-dev-github-com-qawatake-pitboard/50694791-efee-47fc-83da-e2ad13c535a4/scratchpad/g19/index.html`。git は使っていない。

- 決まりは `internal/page/lists.go`: `Loose` (並びのどれかが 2 行以上なら全部の間を空ける)、`HeadWord` (`**頭**: 文` / 全角の `：`)、`TextMeasure` = 72。`Thing.HeadWord` で頭の語を渡し、`Title` は本文
- 端末: 並び (同じ親、空行まで) ごとに決め、緩い並びは行の間に空行。入れ子は自分で決める。頭の語は `頭: 文` が 1 行に入れば今までどおり、入らなければ頭だけの行 (太字・コロンなし) と、1 段下げた本文。段落と箇条の文は 72 桁まで
- ブラウザ: `render.fit` が描いたあとに測る (`.hasHead.def`、`.li.loose` = .5em)。節の直下の箇条の行 (止まる行) は頭の語をいつも run-in で
- 色・太さは足していない。小見出しは今までどおり
- `-h` Manners に 2 文 (箇条は短い 1 句、長くなるなら頭の語か子の箇条、Item の中身は人が読むべきものだけ)
- テスト: page `TestHeadWord`、viewer `TestLooseLists` (幅 40 は緩い、80・120 は詰める: 端末の文の幅は 76 で頭打ちなので 80 と 120 は同じ幅になる)・`TestHeadWordLayout` (定義の形、run-in、72 桁)。ギャラリー `text` に 4 変種 (詰めた並び・緩い並び・run-in・定義) と `differences` (`list-tight` `list-loose` `head-runin` `head-def`、DOM の class で確かめる)
- 気づいたこと: 最初「中身だけ」を合成した節で変種を作ったら、中身の箇条が止まる行として描かれ、生きたページ (開いた Item の中身) と違った。開いた Item を選ぶ形に直した (fidelity の決まりどおり)
