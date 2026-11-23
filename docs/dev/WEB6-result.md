# WEB6 の結果

vet / test (`-count=1`、e2e 込み) / build 通過。git には触っていない。確認はスクラッチのボード 3 つ・一時的な `XDG_STATE_HOME`・`--port 0`・`playwright-cli` だけ。自分のサーバーは `--stop`、ブラウザは `close` で止め、`ps` で残っていないことを確かめた (`playwright-cli list` も空)。

## B. コメントは浮いた欄に
- `#pane` (高さ固定・中でスクロール) と「コメント一覧」の画面を消し、`#cpanel` 1 つにまとめた (`index.html` `app.js` `app.css`)。下のバーの `コメント N` (未済の数。`未送信 M` は黄) で開く。右下のダイアログ、480px 以下は幅いっぱいのシート、どちらもバーの上。高さは中身の高さで、70vh を超えたときだけ中でスクロール (390px で 591 = 844 の 7 割を確認)
- 並びは未済 → 済、それぞれ新しい順。状態の語・行の題 (リンク)・AI の返事・小さい画像。行の題のクリック、または行の上の `enter` で欄を閉じ、そのコメントの行へ (たたまれた親は開く)、中央にスクロール。欄の中で `j k e x u` が効く。`esc`・外のクリックで閉じ、Tab は中にとどまり、閉じると開く前のフォーカスに戻る (どれも実機で確認)
- 飾りは細い線と淡い影だけ。行に付いたコメントは開いた行の下にこれまでどおり出る
- 推測: 「端末の list key にあたるもの」はブラウザに無かったので (`tab` はフォーカス移動)、バーのボタンだけにした。行そのものをクリックしたときは選ぶだけにした (行の題のリンクで移動)。欄から `m` を押すと、欄を閉じてから宛先なしのエディタを開く

## C. 開閉の印
- Web: `page.SignClosed` / `SignOpen` を `Words` で渡し、その印だけ CSS の三角 (`clip-path`、0.78em 四方) で描く。色は元の印のまま。三角の中心と題の 1 行目の中心の差は 0.7px 以下。WEB4 の数値はそのまま: 1280 で 316 / 340 / 364、`>` 260、印 294 / 318、右端 1020。390 で 62.4 / 86.4 / 110.4、右端 374、横スクロール無し
- 端末: `▶` `▼` (U+25B6 / U+25BC) は East Asian Ambiguous で、CJK 向けの設定 (iTerm2 や WezTerm の「曖昧幅を全角」など) では 2 桁になり、`▶` は絵文字の形にもなる。`▸` `▾` (U+25B8 / U+25BE) は Neutral で常に 1 桁。なので字は変えず、開いた `▾` を太字にした (Notice も。最上位の Fold は元から太字の青)。字は `internal/page/signs.go` の定数で決め、viewer の 4 か所 (rows・notice・comments 欄・一覧) もそれを使う。テスト `TestOpenSignIsBold`。SPEC-format・SPEC に追記
- 確かめていないこと: 実際の Ghostty / kitty / tmux での見え方 (幅の表と仕様から判断した)

## A. サーバーは 1 つ、ボードはパスで
- `internal/web`: `Server` が `hosted` ボード (`/s/<id>/`) をいくつも持つ。タブごとの取り消しは (ボード, タブ)。`/` は一覧 (`home.html` `home.js`: 題・ディレクトリ (`~` 付き)・右上と同じ数、`j k enter`、どれかが変わると SSE で出し直す)。登録・外すは `Authorization: Bearer` (0600 の `serve.json` の鍵) だけで受け、cookie では受けない。`CheckBoard`: 絶対パス、実体パスで覚える、`.pitboard` とセッションのディレクトリはリンク不可・自分の所有。画像は `images/` 直下のリンクでない普通のファイルだけ
- `cmd/pitboard/serve.go`: 状態は `$XDG_STATE_HOME/pitboard/` (無ければ `~/.local/state/pitboard/`、0700)。`serve.json` `serve.log` `boards.json` (0600)。`serve` は起動 (必要なら) → 登録 → `/s/<id>/?t=…` を出して開く。`--stop` `--status` (サーバーとボード) `--forget` を追加。古いセッションごとの `serve.json` は見つけたら消し、`--stop` は古いサーバーも止める
- id は登録済みの中で一意なセッションの頭 (8 字以上)。一度付いたら変わらない (`aaaa1111` → 次の `cccc2222dddd` / `cccc2222eeee` は `cccc2222` / `cccc2222e`)。起動し直しても `boards.json` から同じ id で戻ることを実機で確認
- テスト: `boards_test.go` (パス分け・id・一覧・取り消しがボードごと・外す・cookie 無し/知らない id/`..`/`%2e%2e`・ログインの行き先 (`//evil` は `/`)・Bearer 必須・`/` やリンクやファイルの登録拒否・画像リンク・レジストリの復元)。`serve_test.go` (2 つのプロジェクトで同じサーバー・status・forget・再起動で復元・古い `serve.json`・古いサーバーを止める・状態ディレクトリ)。テストは一時 `XDG_STATE_HOME` と `--port 0` で、7482 には触らない
- 推測: セッションのディレクトリがまだ無いと `serve` はエラーにした (前は foreground で待っていた。1 つのサーバーでは待つ置き場が無いので)。セッションが 1 つも無ければ一覧を開く。同じセッション名が別のディレクトリにあれば id に `~2`。一覧の SSE もつながりとして数える (アイドル停止)
- 気づいたこと: 作業中に、私が起動していないサーバー (pid 61587、`~/go/bin/pitboard serve --foreground`、7483、状態は本物の `~/.local/state/pitboard/`、00:30 起動) が動いていた。この版の作業ツリーからインストールされたものに見えるが、触っていない

スクリーンショット: `/private/tmp/claude-501/-Users-qawatake-dev-github-com-qawatake-pitboard/d65df1a1-43fd-4667-8bc6-410598215dd3/scratchpad/w6/shots/` の `index-*`・`board-*`・`panel-*` (それぞれ 1280/390 × light/dark)、`fold-closed-1280-light.png`・`fold-open-1280-{light,dark}.png`
