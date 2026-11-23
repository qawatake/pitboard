# FIX18 の結果

`scripts/check.sh` (vet・test (e2e 込み)・ギャラリー) 通過。git は使っていない。B・C は別のエージェントに任せ、差分を読んで確かめた。

- A 画面だけを動かすキー (カーソルは動かない): `ctrl+d` `ctrl+u`・`pgdn` `pgup`・`alt+↓` `alt+↑` (`ESC[1;3B` `ESC[1;9B` `ESC ESC [B` を読む) で半画面、`ctrl+e` `ctrl+y`・`↓` `↑` で 1 行。報告・コメントの欄 (`pfree`: カーソルに引き戻さない)・一覧で効く。`ctrl+n` `ctrl+p` は `j` `k` のまま。報告が画面より高ければ案内に `^d ^u スクロール` (いちばん先に落ちる)。`?` に全部
- A エディタ: `pgup` `pgdn` `alt+↑↓` で半画面、1 行目の `↑`・最後の行の `↓` で 1 行。入力欄は報告の中の場所にとどまって一緒に動き (`input.scroll`)、画面の外では端末のカーソルを隠す。文を変えるキーでまず戻る。高い報告では案内に `pgup pgdn スクロール`
- A `k` は、`j` で読み下ったカーソルの行の中身を半画面ずつ読み上り、題が出てから前の行へ
- A ブラウザ: 80 行の中身の下で入力欄を開いたまま、ホイールと PageDown で 0 → 473 → 1595 と動き、フォーカスと文は保たれ、打つと入力欄が戻る (headless Chromium で確かめた。変更なし)
- A2 代替スクロール (DEC 1007) を入るときに頼み (`CSI ? 1007 s` で覚えて `h`)、出るときに戻す (`r`)。マウスはつかまえない (1000/1002/1006 を送らないことをテスト)。Ghostty のバイナリに `mouse_alternate_scroll`、herdr (ユーザーの `TERM_PROGRAM`) のバイナリに `mouse_alternate_scroll` と "alternate scroll input" があり、ペインごとに 1007 を覚えてホイールを矢印で渡す。xterm は頼めば効く。kitty・WezTerm・iTerm2・Terminal.app は代替画面で元から矢印を送る (資料による。実機で見ていない)。`PITBOARD_MOUSE` は作っていない
- B `ack-removed` を消した。警告は `check` では出し続け、次に AI が動かす `changes`・`settle`・`wait`・hook の出力の末尾に 1 度だけ出して `rows.json` から消す。末尾の 1 行は `report.md has N problem(s). Run: …` (件数から警告を外した: 1 度で忘れるので「check を見よ」は空を指す)。`-h` は Manners の 1 文
- C `-h` Manners に 2 文 (長い説明は節を足して「あなたへ」から短く指す。Item の中身は 1 画面に。済んだ節は消す)。SKILL.md に同じ文は無い
- テスト: e2e `TestScrollKeys` `TestKReadsBackUp` `TestScrollWhileWriting`、viewer `TestScrollKeyParse` `TestAlternateScroll`。`TestMoveKeys` は `↑↓` を `j k` に。WEBFIX3-13 の続き: 端末でも `m` は `g` `enter` より先に落ちる (`? キー` は最後まで残す。全部を見せる口なので)
- 決めたこと: `↓` `↑` は画面送りなので、前の「`↑↓` も `j k` と同じ」はやめた (SPEC・`?`・案内は `j k` だけ)
