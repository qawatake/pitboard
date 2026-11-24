# WEB9 の結果

`scripts/check.sh` 通過 (vet・test・ギャラリー 242 枚・生きたページとの比較 32 件)。git には触っていない。「訂正」に従い、画像の位置と大きさは前のまま (半分の幅の正方形に収める) で、縁だけを足した
- 浮くもの (`m` の欄・コメントの欄・`?`・画像の全体) は 1 つの面: 地 `--surface` (明るい表示は白、暗い表示は `#252528`)、`line` の 1px の縁、角丸 14px、前より濃い 2 重の影。`--edge` はやめた。390px のシートは、`m` の欄だけ上の縁と角丸を残し、欄と `?` は画面いっぱいで縁なし
- 入力欄は `m` も行の下も 1 枚のカード: 見出しは小さく薄い字 (青い見出しをやめた)、文の欄は縁なし、フォーカスはカードの縁を `structure` に。画像の見本は文の下のカードの中
- 見本: 1px の縁・角丸 8px・間は 12px (文からも)。入力欄の見本の右上に `×` (文からその `[Image #N]` と前の空白を消し、文に戻る)。Tab で届き enter で押せ、そこで `esc` は書くのをやめる。playwright-cli で生きたページで確かめた。画像の全体は幕の上に縁とページの地を敷く
- 測った比 (縁 / 地): 明るい表示 1.77 (ページ)・1.34 (欄を開いて薄く覆ったページ)、暗い表示 1.96 (ページ)・1.74 (面)・2.17 (覆ったページ)。**私の判断**: 覆ったページとの比が 1.25 だったので、明るい表示の `--line` を `#c9c9cf` から `#c2c2c8` に少し濃くした (帯とバーの線も同じだけ濃くなる)。`TestPaletteCSS` の中立の色に `surface` を足した
- 違い `float-edge` (開いている浮いたものすべてに 1px 以上の縁、地と違う色、ページとの比 1.3 以上) と `image-edge` (見本に縁と角丸、隣と 12px 以上、入力欄では `×`) を足し、editor・comments・help・image・comment に宣言した
- 変種: editor に「m: with text」「m: one image」「m: three images」「m: seven images」「a white image, on a line」(12〜16)、image に「a white image」(03)。白い画像 `gallery/white.svg` を 4 番に当てた。生きたページとの比較に editor-12・editor-15・image-03 を足した
- SPEC (画像の見本・浮いた欄の面・`m` の欄・飾りの決まり・`?`)・web-states・`?` の「画像」の行も直した

ギャラリー: /private/tmp/claude-501/-Users-qawatake-dev-github-com-qawatake-pitboard/ee44e2d6-4132-4979-80c5-ee11ecd209ad/scratchpad/gallery-web9/index.html
