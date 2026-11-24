# FEAT25 の結果

`scripts/check.sh` 通過 (vet・test・ギャラリー 208 枚・生きたページとの比較 28 件)。git には触っていない。
- y / n をなくした: `page` (Actions.YN・案内・断り「j k で動かすと y n が効きます」・DropRank の n / y は `RankAnswer` 1 つに)、端末 (キー・`?`・右端の y / n)、ブラウザ (`?`・右端・`.yes` / `.no`)、`changes`・`-h`・README・SKILL・SPEC・e2e・ギャラリー。行にできることは、答える (Ask だけ)・コメント・隠す・読んだ、の 4 つ
- `ask="y"` は check のエラー: `ask="y" was removed: write an <Ask> with <Option>s instead (e.g. <Option key="a">入れる</Option> …)`。黄の ● は答えていない Ask だけに付く (`wait="you"` は今までどおり space の行き先)
- `ViewerKeys` は `cgjkmqrsux` になった。y と n は Option のキーに使える (ギャラリーの `merge` と e2e の `hook` で使っている)
- 前の記録: `option` の無い y / n の react は `Event.Legacy` として、Replay・未送信の数・`changes` で読み飛ばす (その undo も)。ファイルは書き換えない。答えの記録には `option` を必ず入れるので、キーが y の答えと前の版の y を取り違えない
- `-h`: Ask の例を「入れる / 見てから」にした。作法に「はい / いいえ を聞くときも、選択肢でそれぞれの意味を書く」を足した。`changes` の例は answer と retract answer にした
- B: 画像の見本は、どこでも文の列の幅の半分を一辺とする正方形に収める。比は変えず、元より大きくせず、折り返して並ぶ (`.thumbs` を container にして `50cqi`)。ギャラリーには、入力欄の 1 枚 (大)・1 枚 (縦長)・3 枚 と、行の下のコメントの大と縦長を足した (1280 / 390)。縦長の見本は `gallery/tall.svg`
- わざと残した y / n: `look="y"`・`wait="you"`、黄の数の語「反応待ち」、`Event.Legacy` とその説明、`ctrl+e` / `ctrl+y` のスクロール、SPEC の前の記録の項目と check のエラーの文、`-h` の「ask="y" was removed」の 1 行
