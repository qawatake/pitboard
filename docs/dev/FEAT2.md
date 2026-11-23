# pitboard: 送る機能、薄い skill、`m` の案内

この repo (`github.com/qawatake/pitboard`) で作業する。仕様は `docs/spec/SPEC.md` と `docs/spec/SPEC-format.md`。
終わったら仕様書・`-h`・README も合わせる。依存は Go の標準ライブラリだけ。

## 1. 板から AI に「送る」

hook は、AI が動いている間 (ツール実行の合間、止まる直前) にしか届けられない。AI が止まっているときに板へ書いたものは、
人がチャットに何か打つまで届かない。そこで、人が板の上から AI を起こせるようにする (ユーザーの要望: 「こちらからの送信機能。cmd+enter のような」)。

- 報告の画面で `s`: まだ AI に届いていない操作があれば、AI のいるターミナルへ短い文を打ち込んで起こす。無ければ下の行に「送るものはありません」
- エディタ (思いつきを書いている間): `enter` は今までどおり書くだけ。**`ctrl+s` は書いて、すぐ送る**
- 打ち込む文 (1 行):
  `pitboard: the person sent N item(s) from the board. Run "pitboard -s SESSION changes" and act on them.`
  内容そのものは打ち込まない (AI が `changes` で読む。届けた位置の管理を 1 か所にするため)
- 送り方は次の順で決める:
  1. 環境変数 `PITBOARD_SEND` があれば、`sh -c "$PITBOARD_SEND"` を実行する。環境変数 `PITBOARD_SESSION` と `PITBOARD_MESSAGE` を渡す
  2. `HERDR_ENV=1` なら、`herdr agent list` (JSON) から `agent_session.value` がこのセッション id と一致する agent を探し、
     `herdr agent prompt <その pane_id> <文>` を実行する (シェルを通さず、引数で渡す)
  3. どちらでもなければ、下の行に「送り先がありません (herdr の中で開くか、PITBOARD_SEND を設定)」
- 実行は 5 秒で打ち切る。失敗したら、その旨を下の行に出す (stderr の先頭 1 行を添える)。成功したら「AI に送りました」
- 送っても `seen` は進めない (届いたことにするのは、AI が `changes` を呼んだとき)
- 思いつきの欄の見出しの右は、未読があるとき `未読 2 · s で送る` とする
- `s` は viewer のキーなので、`Option` の `key` に使えないキーに足す
- `herdr agent list` の JSON の形は実物で確かめること (`herdr agent list` を実行してよい。`herdr agent prompt` は**実行しない**。
  送る処理のテストは、`herdr` の代役のスクリプトを PATH の先頭に置いて行う)

## 2. `m` (どこでも書ける) を案内に戻す

ユーザーが「どこにカーソルがあっても思いつきを書ける機能はなくなった?」と尋ねた。機能はあるが、案内から消したので見つけられなかった。

- 案内の行に `m 書く` を常に出す (`? キー` の手前)。行についての `c` は `c 行に書く` とする
- 幅が足りないときに落とす順: `x` → `enter` → `n` → `y` → `c`。`m 書く` と `? キー` は最後まで残す

## 3. skill を薄くする (ghtkn の形)

ghtkn は repo に `skills/ghtkn/SKILL.md` を置き、skill には説明を持たせず、バイナリに埋め込んだ文書を読ませている
(道具を更新しても skill が古くならない)。同じ形にする。

- `skills/pitboard/SKILL.md` を作り、今の `skill.md` は消す。`pitboard --skill` はこのファイルを埋め込んで出す (`//go:embed skills/pitboard/SKILL.md`)
- skill の中身は 2 つだけ: frontmatter (name は `pitboard`。description は英語で、いつ使うか) と、
  「`pitboard -h` を読んでから使う。書いてあるのは、いま入っている版の使い方」という数行
- これまで skill に書いていた内容 (始め方、報告の作法、人の操作の読み方、思いつきには必ず行き先を書くこと、hook の設定例、送る機能) は、
  `pitboard -h` に移す。`-h` が長くなりすぎるなら `pitboard -h` は要約 + コマンド一覧にして、`pitboard docs` で全文を出す形にしてよい (判断は任せる。決めた形を報告に書く)
- README に入れ方を書く: `gh skill install qawatake/pitboard pitboard`、または `pitboard --skill > ~/.claude/skills/pitboard/SKILL.md`

## 終わったら

vet / test / build。1 と 2 は pty 越しに自分で確かめる (スクラッチパッドの空のディレクトリで。herdr は代役で)。
`~/.claude/` には何も書かない。git は commit も push もしない (依頼側がする)。
報告は `docs/dev/FEAT2-result.md` に: 項目ごとに 1 行と、推測で決めた点だけ。画面への返事は「書きました」の 1 行。
