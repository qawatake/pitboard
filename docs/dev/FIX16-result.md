# FIX16 の結果

vet / test (`-count=1`、e2e 込み) / build 通過。git には触っていない。
- 覚え方: セッションの `rows.json` に id のある行 (題・注目の印・隠すキー・`ViewKey` のハッシュ) を、AI の動かすコマンド (`check` と末尾の 1 行を足すもの) のたびに足す。`ViewKey` は page から board へ移した (page は呼ぶだけ)
- 警告 (`board.Removed`): 消えた行に (a) 注目の印があって反応・読んだ・隠すが無い、(b) 対応済でないコメントがある、で `warning: row "ID" (題) was removed but the person has not hidden or answered it` / `… but it has an open comment (N)`。`check` は誤りのあとに出し、終了コードは変えない。viewer とブラウザは出さない
- 末尾の行は `report.md has N problem(s), M warning(s). Run: pitboard -s <id> check` (既存のテストも合わせた)。`pitboard -s S ack-removed` で消えている行を忘れる。行が戻る・コメントが対応済になれば警告は止まり、用の無い消えた行は黙って忘れる
- `-h`: コマンドの一覧と、作法「When to remove a row」(いつ消すか・道具が警告すること・直し方)。SPEC に「行を消したときの警告」
- テスト: `internal/board/removed_test.go` (答えていない行・開いたコメント・ack・戻る・doing はまだ開いている・done で止まる)、e2e `TestRemovedRowWarnings` (check の出力と終了コード 0・changes の行・ack-removed)
- 推測: 「人が最後に見た報告」は、AI のコマンドのたびの和集合で近似した (コマンドの間に足して消した行は覚えない)
