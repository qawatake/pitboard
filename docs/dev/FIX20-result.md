# FIX20 の結果

vet・test・build・`scripts/check.sh` 通過 (ギャラリー 205 枚・生きたページとの比較 28 件: `/private/tmp/claude-501/-Users-qawatake-dev-github-com-qawatake-pitboard/02e727dc-20e5-4942-bb45-3cd5ab84a6d4/scratchpad/gallery20/index.html`)。git には触っていない。
- 構成案: `-h` の 1 を「題だけ」に変え、要約は節と重なって古くなるので置かない理由を 1 行足した。`-h` の例からも要約の段落を消し、SPEC も同じ。書式はそのまま (SPEC-format の書式の例にある題の下の段落は、書ける例として残した)
- 言葉: 一覧の見出し `ボード` → `pitboard`、空の一覧の文 → 「まだ何もありません。…」。`internal/web` に `TestNoOldWords` (ページの js/html と cmd・internal のすべての Go の文字列から 板・ボード を探す。クリップボードは除く)、端末の `TestNoOldWords` にも `ボード`
- エディタ: 入力欄の下の説明の行を消した (同じことは `?` にある)。画像は欄のすぐ下に高さ 240px・欄の幅まで、置いたコメント (行の下・コメントの欄) は 320px・列の幅まで。どちらも元より大きくしない・折り返す・クリックで全体
- ギャラリー: エディタの「one large image」「three images」、コメントの画像のある変種を 1280/390 × 明暗に。SPEC を更新
