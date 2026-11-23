# WEBFIX1 の結果

vet / test (e2e 込み) / build 通過。git には触っていない。ブラウザは playwright-cli (Playwright 同梱の chromium_headless_shell、`--no-open`、スクラッチの置き場) だけ。端末は pty で抜き取り確認 (placed の `y` は「j k で動かすと y n が効きます」、`x` は「隠しました (H で表示、u で戻す)」)。

- A. 守りは `internal/page/guard.go` に 1 つ: `Touch` (Placed / Locked / Auto+Shown / Open / Again) → `Board.Allow(n, key, touch)` が `Verdict{Do, Value, Msg, Again}`、`Board.Offer` が出してよいキーと語、`Rewritten`・`HiddenKeys`/`Reappeared` も page。viewer はこれを呼ぶだけ (`react`/`hide`/`read` と下の案内を置き換え)。web は `POST /api/act` の `key` と `hint`。JS は `Cur{Start, How, Ago, Sel, Open}` (事実) を送るだけで、規則の文も判断も持たない。操作の文 (隠す・y n・読んだ・置いた・直した・取り消した) は page から両方同じ。端末が言わないものは端末側で捨てる。行頭の印 (`Sign`/`OpenSign`)・閉じた問いの答えの頭 (`OptionHead`)・`g` の字 (`JumpKey`)・誤りの見出し (`ProblemHead`) は JSON で渡す。表 `internal/page/guardtable` を page・viewer・web のテストが同じく回す
- B. フォーカス = カーソル (Tab・クリックで移ると「人が動かした」)。操作のあとはカーソルの行にフォーカス。`?` と画像はフォーカスを閉じ込め、閉じると戻す
- C. SSE の誤り・403・届かないとき、上に page の文の帯を出し、表示を薄くし、3 秒ごとに静かに読み直す。届けば消える (ネットを切って戻す・起動し直して URL を開き直す、で確認)
- D. カーソル移動は 2 行の class とボタンだけ。読み直しは行をキーで使い回し、並びが同じ所は DOM を動かさない。`/api/page` は ETag で 304、空の項目は JSON に出さない (5000 行で 4.4 MB → 2.6 MB)。行に `content-visibility: auto`。5000 Item で `j` 平均 5.3 ms・最大 7.3 ms、読み直し+描画 約 90 ms
- E. 幅 390 の `● a 本文` は 1 行、下の余白はバーの高さに合わせる、`retract`/`edit` の無い番号は `ErrNoComment`、1 MB 超の act は「長すぎるので受け取れませんでした」、読み直しの失敗で知らせを上書きしない、`g` の文は字を押すと消える、コメント一覧から戻るとカーソル (とその来方) が戻る
- REVIEW12 の高・中の再現をすべてやり直し、端末と同じ動きと文になった (placed の `x`、Tab 後の `n`、隠して移った先の 0.4 秒 (1 回目は断り、同じキーでもう一度は効く)、書き換えのロック、隠した行がまた出た知らせ、起動し直し、5000 行)

推測で決めたこと:
- 端末の `placed` は「x で隠して移った先も placed のまま」なので、web も `Start` を事実として別に送る (How=app と両立)
- 書き換えのロックは、カーソルが来たときの行のトークン (`Sel`) と今の行を page の `Rewritten` で比べてサーバーが決める。端末と同じく、ロック時は何も言わず、キーを押すと「報告が書き換わりました…」
- コメントの `x` の 0.4 秒もサーバーが `Touch.Settling` で決める (黙って捨てる。端末と同じ)
- どの英字キーもサーバーに送り、何もしないキーは黙る (端末と同じ)。そのとき下の知らせは消さない
- `?` の説明 (`HELP`) とコメント欄の件数は WEB2 のまま JS

確かめていないこと: 本物の IME・スクリーンリーダー・Firefox / Safari。送るのが遅いときの表示。`content-visibility` の効かないブラウザでの速さ。
