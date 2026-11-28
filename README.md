# pitboard

AI のセッションの横のペインに置く報告の画面。チャットは流れるので、流れては困るもの (いまの状況、あなたが決めること、見てほしいもの) を AI がここに書く。

名前の見立て: レース中のドライバーには話しかけられないので、周回ごとにボードの前を通るときに読ませる。

- 報告は AI だけが書く。あなたは pitboard の上で応える: 問いへの答え (選択肢のキー)、行についてのコメント (`c`)、宛先なしのコメント (`m`)、行を隠す (`x`)、直前の操作を取り消す (`u`)。コメントには画像も貼れる (`ctrl+v`、画像ファイルのパスの貼り付け)
- 答えやコメントは書いただけでは AI に届かない。まとめて `s` で送る。AI が待っていれば (右上に `AI 待機中`) すぐ届く
- 緑の `●` は AI が読んでほしい行。読んだら `r` で消す
- コメントには状態が付く (未送信 → 送信済 → 既読 → 対応中 / 保留 / 対応済)。書いたら忘れてよい
- ほかのキーは viewer で `?`

依存は Go の標準ライブラリだけ。

仕様の回帰テスト (ビルドした pitboard を CLI と pty で動かす) は `go test ./e2e` (`go test ./...` にも入る)。

## 入れる

```sh
go install github.com/qawatake/pitboard/cmd/pitboard@latest
```

AI に使わせるには skill を入れる。skill は「`pitboard -h` を読む」と要点、どの場面でどの話題 (`pitboard help TOPIC`) を読むかだけを書いた薄いもので、AI は入っている版の `-h` から使い方を読む。

```sh
gh skill install qawatake/pitboard pitboard
# または
pitboard --skill > ~/.claude/skills/pitboard/SKILL.md
```

## 開く

AI に「pitboard に報告して」と頼むと、AI が報告を書き始め、開き方を伝えてくる。AI を動かしているディレクトリで、横のペインに開いたままにする。

```sh
pitboard            # いちばん最近更新されたセッション
pitboard -s ID      # セッションを選ぶ (id は先頭数文字でよい)
```

ブラウザで見たいときは `pitboard -s ID serve`。サーバーはユーザーに 1 つで、裏で動く。動いていなければ起動し、そのボードを加えて、ボードの URL (`http://127.0.0.1:7482/s/<短い id>/?t=…`) を 1 行出してブラウザで開き、すぐ戻る (`--no-open` で開かずに URL だけ)。別のセッションや別のプロジェクトで実行すれば、同じサーバーにそのボードが加わる (起動し直さない。開いているタブもそのまま)。`/` は加えたボードの一覧。ページを 30 分開いていなければサーバーは自分で止まる。すぐ止めるのは `pitboard serve --stop`、サーバーとボードの一覧は `pitboard serve --status`、ボードを外すのは `pitboard -s ID serve --forget`、端末に付けたまま動かすのは `--foreground` (ctrl+c で止まる)。状態とログは `$XDG_STATE_HOME/pitboard/` (無ければ `~/.local/state/pitboard/`) に置き、加えたボードは次に起動したときも出る (ディレクトリが消えたものは黙って外す)。端末の画面と同じ部品・同じ言葉・同じ決まりで、端末とブラウザを同時に開いてもよい (片方でしたことはもう片方にすぐ出る)。キーは端末と同じで、マウスでも操作できる。127.0.0.1 だけで待ち受ける。URL の鍵は状態ディレクトリの `token` (0600) に一度作って使い続けるので、サーバーが止まって起動し直しても URL は同じで、開いていたタブはひとりでにつながり直す (そのときは `serve` は 2 つ目のタブを開かない。`--open` で必ず開く)。一度 `?t=…` 付きで開けば、cookie (30 日、使うたびに延びる) で `http://127.0.0.1:7482/s/<id>/` をブックマークから開ける。鍵を変えるのは `pitboard serve --rotate` (それまでの URL と cookie は効かなくなり、動いていればサーバーを起動し直す)。ポートは既定で 7482 (電話のキーで「PITB」)、ふさがっていれば 7491 までの次の空き、それも無ければ空いているもの。`--port N` はその番号だけ。

## 届かないとき

`s` で送っても AI が待っていないと、そのままでは届かない (画面にそう出る。チャットで一言伝えれば読む)。

- **Claude Code の hook に入れる** (おすすめ): 止まっている AI は `s` で起き、作業中の AI にはツールを使うたびに届く。AI は何も動かさなくてよい。設定例は `pitboard help hooks`
- **hook を入れないとき**: AI が止まる前に `pitboard wait` を裏で動かして待つ。それも無いときは、`PITBOARD_SEND` を設定すると AI の端末に 1 行打ち込んで起こす (tmux の例は `pitboard help send`)。herdr の中で AI を動かしていれば設定は要らない

## 日本語でコメントを書くとき

コメントを置いたあとも IME が日本語のままだと、続けて押す `s` などが変換に取られる。入力ソースを英数に切り替えるコマンドを `PITBOARD_IME_OFF` に置くと、端末の viewer はコメントのエディタが閉じるたびにそれを実行する (詳しくは `pitboard help ime`):

    export PITBOARD_IME_OFF="osascript -e 'tell application \"System Events\" to key code 102'"

macOS の英数キー (key code 102) を押すだけで、何も入れなくてよい。初回は端末アプリにアクセシビリティの許可を求められる (システム設定 > プライバシーとセキュリティ > アクセシビリティ)。切り替えのたびに出る「あ/A」の表示は `defaults write -g TSMLanguageIndicatorEnabled -bool false` で消せる (再ログイン後に効く)。

## 文書

- [仕様](docs/spec/SPEC.md)
- [報告の書き方](docs/spec/SPEC-format.md)
- AI 向けの使い方: `pitboard -h` (毎回読む分) と `pitboard help TOPIC` (format / changes / send / hooks / browser。要るときに読む分)
