# WEB2 の結果

vet / test / build は通る。git には触っていない。`re` の行の決まり (FIX12) は FIX14 で `re` が無くなったので扱っていない。

## できたもの

コマンド: `pitboard [-s S] serve [--port N] [--no-open]`。セッションの決め方は viewer と同じ (無ければ最初に現れたセッションを待つ)。127.0.0.1 で待ち受け、URL を 1 行出して `open` / `xdg-open` で開く。ctrl+c / SIGTERM で閉じる (SSE が開いたままでも終了コード 0 を確認)。

経路
- `GET /?t=TOKEN`: 鍵を照合し、HttpOnly・SameSite=Strict・Path=/ の cookie (`pitboard_PORT`) を付けて `/` へ 303
- `GET /` `/app.js` `/app.css` `/favicon.svg`: 埋め込みの静的ファイル
- `GET /api/page[?hidden=1]`: `page.Page` の JSON (+ `Session` `NoBoard` `Base` `ImageMax`)
- `GET /api/events`: SSE。見張りが変化を見たら `event: changed`。15 秒ごとに keep-alive
- `POST /api/act`: `{Action, Target, Seen, Value, Text, N, ShowHidden}` → `{OK, Message, Changed, Hidden}`。react hide comment edit retract read readall undo
- `POST /api/send`: 送り終わるまで待って `{OK, Message}`
- `POST /api/image`: `{Data: base64}` → `{OK, Image: N, Message}`
- `GET /api/image/N`: セッションの画像を番号で
- 上記以外の `/api/` は 405、cookie なしは 403 (「pitboard serve が出した URL」を開くよう言う)、Host 違いは 403、Origin 違いの POST は 403、JSON でない POST は 415

ファイル
- `internal/web/server.go`: 起動・待ち受け・鍵と cookie・Host/Origin・ヘッダ・振り分け・静的ファイル・タブごとの Board
- `internal/web/api.go`: `/api/page` `/api/act` `/api/send` `/api/image` と画像の配信
- `internal/web/events.go`: 見張り (0.5 秒ごと) と SSE
- `internal/web/static/index.html` `app.css` `app.js` `favicon.svg`: 手書きの静的ファイル (依存なし、ビルドなし、ES module 1 本)
- `internal/web/web_test.go`: ヘッダ・鍵・Host/Origin・JSON の形・全操作の往復・古い `seen`・画像・SSE・`innerHTML` を使っていないこと
- `internal/page/web.go` (page に追加): 種類などの名前 (`MarshalText`/`UnmarshalText`)、`Next`、`Seen.Token`、`StartsOpen`、`Board.Summary`、`Board.Changed`、`Board.SaveImage` (中身を見て PNG JPEG GIF WebP だけ)、`Board.ImageFile`、`IsImageType`、`ErrNotImage`
- `internal/page/page.go` (page に追加): `Thing.Token` `Thing.Open` `Thing.Jump`、`Page.JumpTo` `Page.Next` `Page.Summary`、`Node`/`Seen`/`Jumps` は JSON に出さない、`Right.AtText`
- `internal/board/images.go`: `Store.ImageFile(n)` (既存の `imagePath` を外に)
- `internal/viewer/draw.go`: 右上の数 (`sendMark`) を `page.Board.Summary` から描くようにした (語・順・色は同じ)
- `cmd/pitboard/main.go` `serve_test.go`: `serve` と引数
- 文書: `-h` (コマンド一覧と "Browser" の段落)、README (1 段落)、SPEC (「ブラウザ」の節)

## 確かめたこと

Go のテスト (上記)。加えて、`playwright-cli` で実際のページを動かした (システムの Chrome は使っていない)。playwright-cli 既定の chromium は入っていなかったので、すでに入っていた Playwright 同梱の `chromium_headless_shell-1248` を `executablePath` に指定した (何もインストールしていない)。スクラッチパッドの置き場で、`PITBOARD_SEND` はファイルに書くだけのコマンド。終わったらブラウザは閉じ、サーバーは止めた。

- 最初の描画・右上の数・お知らせ・節・折りたたみの見出し・リンク・`<img …>` を含む行が文字のまま出る。コンソールのエラー・CSP 違反は 0 件
- キー: `j` `k`、閉じた問いで `a` (開くだけ) → `a` (答える)、`y`、`r`、`c` で書いて enter、`m`、`x` (隠す → 次の行へ、0.4 秒の守り)、`H`、`u`、`g` + 字 (節の最初の行へ)、space (wait="you" の行 → 折りたたみを開いてその中の未読へ)、`?` と esc、`s` (送りました・`PITBOARD_SEND` に 1 行)
- 守り: 最初は選んだだけで y n が効かない、選んでいる行の題を AI が書き換えると「報告が書き換わりました。カーソルの行を確かめてください」で n が記録されない、動かすと効く
- 生きた更新: エディタに書いている途中で報告が書き換わっても、書きかけ・フォーカスが残る。esc で閉じて m で開くと書きかけが戻る。別のタブ id からのコメントが下の欄に出る
- 画像: 貼り付けのイベントで `[Image #1]` が入りサムネイルが出る、置いたコメントに画像が出る、クリックで大きく出て esc で閉じフォーカスがサムネイルへ戻る
- コメント一覧: `j`、`e` で直す、`x` で取り消す、`u` で戻す、enter で中身
- 幅 390 のダークモードの見た目 (下の文が本文に重なっていたのを直した)。Tab でボタンに移る、エディタの中では `j` `k` `y` が文字になる
- 色のコントラスト: 薄い字も含め、両方のモードで背景 (選んだ行・下のバー・コード・送るボタン) に対して 5.0:1 以上 (計算)
- 端末: pty (80×30) でこの置き場の viewer を開き、右上の数 (未読・送信済・未送信)・行・コメントの欄が出て `q` で終わる

## 推測で決めた点

- 「最初に開いているか」は page の `StartsOpen` で、すべて閉じて始まる (端末の今の動きどおり)。端末は空の開閉表から始めるので、同じ結果になる
- `tab` キーはブラウザのフォーカス移動に残し、コメントの一覧は「コメント一覧」のボタンにした (Tab で全部にたどり着けることを優先)。SPEC と `?` に書いた
- Space は行のボタンの上でも「印のある行へ」(端末と同じ)。行の開閉は Enter とクリック
- `q` は無い (止めるのはサーバー側)
- 操作の守り (行が同じか) は、サーバーがタブに見せた行をトークンで覚え、その行の `Seen` で `page` の操作を呼ぶ形にした。コメントは書き始めたときの題で記録される (端末と同じ)。`read` は page に `seen` の守りが無いので、web 側で今の行と照合してから呼ぶ
- 端末が何も言わない操作 (コメントを置く・直す・画像を貼る) にも、読み上げ用に短い文を出す (「コメントを置きました (s で送る)」など)。空で enter は端末と同じ「空だったので書きませんでした」
- 下のコメントの欄は、まだ済んでいないもの全部 (端末は 3 件)。お知らせを固定する部分は画面の 4 割まで、その中でスクロール
- 選んだ行には、守りの間もボタンを出す。ボタンを押すのはその行を見て選んだことになる (キーの守りはそのまま)
- タブの履歴は 1 時間使われなければ捨てる。サーバーを起動し直すと履歴は無くなる
- 画像の配信は、置き場の画像を番号で、中身が画像のときだけ (コメントに付いているかは見ない)

## 確かめていないこと

- 実際の日本語入力 (IME) での enter: `isComposing` / keyCode 229 を見るコードを読んだだけで、IME を通しては試していない。変換中に届いた更新を確定まで待つ処理も同じ
- ドロップでの画像: コードを読んだだけ (貼り付けのイベントは試した)
- スクリーンリーダーでの読み上げ: アクセシビリティツリー (ボタン名・`aria-expanded`・印の「反応待ち」「未読」) を playwright のスナップショットで見ただけで、読み上げソフトでは試していない
- `prefers-reduced-motion`: スクロールを滑らかにする所が無いので、指定は実質何も変えない
- ライトモードの狭い幅、Firefox / Safari: 見ていない
- `xdg-open` (macOS 以外) で開く処理: 読んだだけ。macOS の `open` もテストでは呼んでいない (`--no-open` で動かした)
- 長く開いたままのタブの「1 時間で履歴を捨てる」: 読んだだけ

## 途中で見つけたこと (直していない)

- `x` で隠したあとの `u` は、端末と同じく読んだことは戻さない (WEB1 で挙げた点のまま)
- `-s` を付けずに `.pitboard/` の無い所で `pitboard serve` したときの `MsgNoBoard` の表示: コードを読んだだけで、テストも実画面も無い
