# WEB7 の結果

`scripts/check.sh` 通過 (vet・test・ギャラリー 194 枚・生きたページとの比較 28 件)。git には触っていない。ギャラリー: `/private/tmp/claude-501/-Users-qawatake-dev-github-com-qawatake-pitboard/02e727dc-20e5-4942-bb45-3cd5ab84a6d4/scratchpad/gallery/index.html`
- 位置: `app.css` に `--float-inset` 1 本 (本文の列と同じ左右の端、下端はバーの 0.75rem 上、上へ伸びる)。`m` の欄とコメントの欄の両方が使う。480px 以下はこれまでどおりシート (欄の上の線も残した)
- **私の判断**: コメントの欄も同じ位置に移した (視線の問題は同じなので)。右下に戻したければ言ってください
- 影: `--shadow` 1 本 + `--edge`。明るい: 線なし、近い影 (1px 3px .07) と広い影 (10px 30px .12) の 2 重。暗い: `--line` の細い線とかすかな影。色付きの光は無し
- ギャラリー: 変種「m: long text grows upward」を足した (1280/390 × 明暗)。`loose-editor`・`panel-scrolled` の確かめに「列の端にそろい、下端がバーの少し上」を加えた。SPEC の 2 か所を更新
