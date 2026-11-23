---
name: using-pitboard
description: Keeps a pitboard board for the current Claude Code session — a report the AI writes in .pitboard/<session>/report.md and the person reads in a viewer beside the chat, reacting with y/n, answers and thoughts that come back through `pitboard changes`. Use when the user mentions pitboard, "板", or asks to put a report on the board, or when the current directory has a .pitboard/ directory.
---

# pitboard を使う

チャットは流れるので、流れては困るもの (いまの状況、決めてほしいこと、見てほしいもの) を板に置く。
AI は報告を書き、人は板の上で反応 (`y` `n`・選択肢) と思いつきを書く。人は報告を書き換えず、AI は人の記録を書き換えない。

## 始め方

1. セッション id を得る。Claude Code のセッション id で、スクラッチパッドのパスの末尾の部分
   (`.../<project>/<session-id>/scratchpad` の `<session-id>`)
2. `pitboard -s ID path` で報告の場所を得る (初回はここで板ができる。上のディレクトリに `.pitboard/` があればそこに、無ければカレントディレクトリに)。そのファイルを Edit/Write で書く。
   板はカレントディレクトリから上へさかのぼって探すので、あとでサブディレクトリへ `cd` しても、コマンドも hook も同じ板を使う
3. 書式と作法は `pitboard -h` を読む (ここには繰り返さない)
4. ユーザーに、横のペインで `pitboard -s ID` (viewer) を開いてもらう

## 報告の作法

- 構成は自由。ただし読むものではなく拾い読みするもの。1 項目 1 行、補足は項目の中身に
- 反応がほしい行は `Item` / `Ask` / `Link` にして `id` を付ける。id は別の物に使い回さない
- 書き換えたら `pitboard -s ID check` を通す

## 人の操作の受け取り方

`pitboard -s ID changes` が、まだ受け取っていないものを 1 行ずつ出す。

- `y` は「よし・済み」、`n` は「不要・だめ」。`answer ID = a (選択肢の文)` は問いへの答え
- `thought N ...` は思いつき。`on ID` があればその行について書かれたもの
- `hide: 文` は、人がその行をもう見なくてよいと判断した印。対応は要らない。報告を書き直すついでにその行を落としてよい。`unhide: 文` は、その行をまた見たいという印
- 隠すのは行の内容に対して。その行の題・状態 (`state`)・`status` を変えるとまた現れる。中身・`note`・`Link` の `path`・`Ask` の選択肢を変えても、隠した `Fold` に行を足しても現れない。人に改めて見てほしい変更なら、題か状態を変える
- 括弧の中は、人が押した (書き始めた) 時点の行の題。`→ いまは: …` が付いていれば、その後で行が書き換わっている。
  人が見て判断したのは前の題のほうなので、いまの行にそのまま当てはめず、どちらへの反応かを考える。`→ いまは無い` は行が消えたもの

## 思いつきの扱い

急いで全部さばかなくてよい。ただし受け取ったものには必ず `pitboard -s ID settle N 行き先` で行き先を書く
(todo にした / 答えた / 控えた / 見送った)。人は板でそれを見て「書いたら忘れてよい」と信じられる。行き先の無いまま放置しない。

## hook

hook を入れると、板に書かれたことが AI の作業中 (次のツール実行のあと) と、止まる直前にも届く。
入れていない間は、ターンの最初と止まる前に自分で `changes` を呼ぶ。

`~/.claude/settings.json` に:

```json
"hooks": {
  "UserPromptSubmit": [{ "hooks": [{ "type": "command", "command": "pitboard hook prompt-submit" }] }],
  "PostToolUse":      [{ "hooks": [{ "type": "command", "command": "pitboard hook post-tool" }] }],
  "Stop":             [{ "hooks": [{ "type": "command", "command": "pitboard hook stop" }] }]
}
```

- hook は板の無いセッションでは何もしない (何も出さず、何も作らない)。板が読めない・書けないときも黙っている
- 止まる直前の hook は、届けるものが `hide` / `unhide` だけなら止めない (次の機会に一緒に届く)
- hook が入っているかどうかは AI からは分からない。`changes` を自分で呼んでも害はない (同じものが二重に届くことはない)
