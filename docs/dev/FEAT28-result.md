# FEAT28 の結果

`scripts/check.sh` 通過。
- `Ask` にコメントを書くと、選択肢を選んだときと同じく黄の `●` (反応待ち) が消える。コメントは書いたときの行の内容 (`sig`) を `line` に覚える。`Ask` が書き換わる、またはコメントを取り消すと、黄の `●` はまた付く。判定は `page.Board.Waiting` 1 か所で、端末とブラウザの両方に効く
- 行が消えたときの警告: `Ask` も `wait="you"` と同じく、送ったコメントがあれば用が済んだとみなす
- `-h` の Marks と SPEC の「反応待ち」「用が済んだ」を同じ内容にした
- テスト: `TestCommentAnswersAsk` (page)、`TestRemovedAskCommented` (board)
