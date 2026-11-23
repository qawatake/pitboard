# WEB1 の結果

vet / test / build は通る。git には触っていない。作った実行ファイルで pty (スクラッチパッドの空のディレクトリ、幅 70・高さ 22) 越しに、開く・`j`・`enter`・`c` で書いて置く・`y`・`x`・`u`・`s`・`g` → 字 を順に押し、画面と `events.jsonl` が今までどおりなのを見た。送るは `PITBOARD_SEND` をファイルへ書き出すだけのコマンドにして確かめた (herdr は呼んでいない)。

新しい層は `internal/page`。`internal/viewer` は報告の読み込み・記録の状態・印・右端の語・操作をこの層に任せ、残りは描画とカーソルだけになった (viewer は約 650 行減った)。

## 公開する型と関数

報告と記録
- `Report`: 最後に読んだ報告 (`Path` `Mtime` `Err` `Missing` `Nodes` `Probs` `ReadErr`)
- `NewReport(path) *Report`: 報告を読む
- `(*Report) Load()`: 読み直す (読めなければ前のものを残す)
- `(*Report) Stale() bool`: ファイルが変わったか
- `Board`: セッションの報告と記録と、この画面で人がしたこと (`Store` `NoSession` `Report` `State` `Viewed` `Unsent` `Unread` `Waiter` `Hist`)
- `Open(st) *Board`: 置き場を開く (`nil` はセッション無し)
- `(*Board) Load()`: 記録を読み直す
- `(*Board) LoadViewed()`: 読んだ印を読み直す
- `(*Board) Poll() bool`: ディスクで変わった記録・読んだ印・`wait` の有無を読み直す (記録が変わったら true)
- `(*Board) CanWrite() bool`: 記録できるか (セッションがあるか)
- `(*Board) Thought(n) *board.Thought`: 番号でコメント
- `(*Board) IsHidden(n) bool`: 人が隠した行か
- `(*Board) HiddenCount() int`: 隠した行の数
- `Safely(f) error`: panic をエラーにする
- `ModTime(p) time.Time`: ファイルの更新時刻

見せる物の木
- `(*Board) Page(showHidden) *Page`: 画面に並ぶ物の木と全体の情報を組み立てる
- `Page`: 報告の代わりに出す文 (`Message`)、`Err` `ReadErr`、誤りの行 (`Problems` `Partial`)、`Notices`、`Things`、反応待ち・未読・未送信・送信済の数、AI 待機中、隠した行の数、`CanWrite` `CanUndo` `CanSend`、飛び先 (`Jumps`)、コメントの一覧 (`Comments`)
- `Thing`: 1 行。種類、宛先 (`Key` `Target`)、題 (`[]Span`)、止まる行か、開けるか、できる操作、印、あなた待ち、隠されているか、操作に添える内容の鍵 (`Seen`)、右端 (`Right`)、`State` `Note` `Path` `URL` `At`、引用 (`Quote`)、付いたコメント、中身 (`Body` `Kids` `Subs`)、選択肢のキーと選ばれたか、番号・段・小見出し・空行、コード
- `Kind` と `KindNotice` 〜 `KindCode`: 物の種類 (お知らせ・題・節・項目・問い・選択肢・リンク・折りたたみ・段落・箇条・コード)
- `Span` / `Style` (`StylePlain` `StyleBold` `StyleCode` `StyleLink`): インラインの整形の区間 (エスケープ列は持たない)
- `Spans(s) []Span`: インラインの Markdown を区間にする (viewer の `inline` もこれを使う)
- `Comment`: 見せるコメント (番号、文、宛先と今の題・書いたときの題、状態の語、AI の対応、まだのときの説明、済んだか、画像の番号)

行ごとの決まり
- `Stops(n) bool`: 人が指せる (止まる) 行か
- `(*Board) CanOpen(n) bool`: 開けるか
- `Actions`: できる操作 (`YN` `Options` `Comment` `Hide` `Read`)
- `(*Board) ActionsOf(n) Actions`: その行でいまできる操作
- `KeysFor(n)`: その行が受ける反応 (y/n・選択肢・コメント)
- `Seen` / `SeenOf(n)`: 人が見ていた行の内容の鍵 (操作に添える)
- `(*Board) Quote(n) *board.Thought`: `re` が指すコメント
- `Mark` と `MarkNone` `MarkNew` `MarkWaiting`: 印
- `ViewKey(n) string`: 未読の判定に使う「行の内容」
- `(*Board) Waiting(n) bool` / `IsNew(n) bool` / `MarkOf(n) Mark`: 反応待ち・未読・行の印 (`Fold` は中身のいちばん強い印)
- `(*Board) UnreadIn(n)`: 読んだにしたとき消える行
- `(*Board) MarkViewed(ns...)`: 見たことを記録する
- `Marked` / `(*Board) Lines(showHidden)`: 印のある行へ飛ぶための、ページ順の行と印と囲む `Fold`
- `(*Board) Counts(showHidden)`: 反応待ちと未読の数
- `(*Board) AnyMarked(showHidden)`: 飛ぶ先があるか
- `(*Board) Jumps(showHidden)`: `g` の飛び先 (お知らせ・節・最上位の `Fold`)
- `(*Board) Newest()` / `Pending()`: コメントの並び (新しい順 / 欄の順)
- `(*Board) About(t)`: コメントが付いた行の題

語
- `Tone` (`ToneQuiet` `ToneNeed` `ToneMove`) と `Word`: 右端の語と、色の意味 (薄い / 人の手が要る / 動いている)
- `Right` / `(*Board) RightOf(n)`: 行の右端 (人の反応のキー、選んだ選択肢の文、コメントの数、入力の状態の語、AI の status か `作業中`、お知らせの時刻)
- `AtWord(t, now)`: お知らせの時刻の書き方
- `StatusWord(s)` / `(*Board) ThoughtWord(t)` / `(*Board) ThoughtNote(t)` / `SettleHead(state)`: コメントの状態の語と説明
- `WorkingWord` `UndoingWord`、`MsgChanged` `MsgMoved` `MsgNoReport` `MsgEmpty` `MsgNoBoard` `MsgNoSession` `MsgProblems` `MsgPartial`、`MsgNoSuchSession(s)`、`ProblemLine(p)`: 画面の文言
- `TextHead(s)`: 知らせの文に入れるコメントの頭

操作 (どれも記録して読み直し、人に見せる文かエラーを返す)
- `(*Board) React(seen, value)`: y / n / 選択肢。行が `seen` と違えば `ErrChanged` で何も記録しない。その行は読んだことになる
- `(*Board) Hide(seen)`: 隠す・表示に戻す (同じく `seen` で守る)
- `(*Board) Comment(target, title, text)`: コメントを置く (行に置けば読んだことになる)
- `(*Board) Edit(n, text)`: コメントを直す
- `(*Board) Retract(n)`: コメントを取り消す
- `(*Board) Read(key, how)` / `ReadAll(how)`: 読んだ / すべて読んだ
- `(*Board) Undo()`: この画面でした操作を新しい順に 1 つ取り消す
- `(*Board) Send(progress)`: 未送信を送り、AI を起こす (`wait` → `$PITBOARD_SEND` → herdr)。起こす間に見せる文を `progress` に渡す
- `Done`: 取り消しの履歴の 1 件
- `ErrChanged` / `ErrNothingToUndo`: 断った理由
- `SendTimeout` / `WakeMessage(st, n)`: 起こす処理の打ち切りと、AI の端末に打つ 1 行

## viewer に残したもの

- 開閉 (`open`)、カーソル (`cur` と、置かれたカーソル `placed`・書き換わったあとの `lock`・自動で動かした行の 0.4 秒 `auto`)、スクロール、画面の外のカーソル、コメントの欄・一覧のカーソル: ブラウザは別の持ち方をする (どこを指しているかは画面ごと)
- 「操作してよいか」の端末固有の守り (`target` `hideTarget` `readTarget`: 画面に出ているか、人が動かしたカーソルか、0.4 秒): 依頼どおり。どの画面でも要る守り (行の内容が変わっていないか) は `Seen` で page に移した
- 描画 (行の組み立て、折り返し、字下げ、右端の語の置き方、お知らせの固定、印の列)、色の実際の値 (`toneColor`)、キーの割り当てと案内の行、`?` の一覧、エディタ、クリップボードの画像: 端末に固有
- `space` で飛ぶ動き (`jump`) と `g` の飛び方 (`goTo`): 飛ぶ先の一覧は page (`Lines` `Jumps`)、カーソルとスクロールの動かし方は viewer
- 報告の読み直しの判断 (`Stale` を見て `reload`): 読み直すとカーソルを動かすので
- 置き場が無いときの文の出し分け (`MsgNoBoard`): `.pitboard/` を探す基点は画面を開いた場所なので

## テストを直したところ

- viewer のテストで、記録の状態のフィールドが `page.Board` に移ったための名前の付け替えだけ: `a.state` → `a.b.State`、`a.unsent` → `a.b.Unsent`、`a.hist` → `a.b.Hist`、`a.waiter` → `a.b.Waiter`、`pv.nodes` → `pv.Nodes`、`wakeMessage` → `page.WakeMessage`、`sendTimeout` → `page.SendTimeout`。判定や期待値は変えていない
- page に `page_test.go` を足した (木・`Seen` の守り・隠す・コメントと引用と送る・読んだ・報告が無いときの文)

## 推測で決めた点

- 名前は勧めどおり `internal/page`
- viewer は木 (`Page`) をまるごとは使わず、木を組む同じ関数 (`RightOf` `MarkOf` `CanOpen` `ActionsOf`/`KeysFor` `Quote` `Lines` `Jumps` `Spans` …) を行ごとに呼ぶ形にした。viewer の描画は開閉とカーソルに沿って木をたどるので、木を作り直す形にすると描画の書き直しが大きく、挙動を変えずに移せる範囲を超えると判断した。決まりは 1 か所にある
- 端末のキー名が入る文 (`隠しました (H で表示)`、`取り消しました (u で戻す)`、`r (読んだ) を取り消しました`) は、page は `隠しました` `取り消しました` を返し、キー名の部分は viewer が足す。読んだの取り消しの文は、操作の呼び名 (`how`) を画面が渡す
- `React` は、端末では案内に出していない値 (Item への `a` など) を page でも断るようにした (端末では元々届かないので挙動は同じ)
- 送るの「送っています…」は `progress` に渡し、端末は画面を描き直して、その間に押されたキーを捨てる (`dropBefore`) のは viewer に残した

## 途中で見つけたこと (直していない)

- `x` で隠すと、その行は読んだことにもなる (緑の `●` が消える)。`u` で隠すのを取り消しても、読んだことは戻らない (緑の `●` は戻らない)。仕様書の「隠した (戻した)」で読んだことになる、の範囲だが、取り消しで戻らないのは意図か確かめたい
- `-h` の `Option` のキーの説明と `ViewerKeys` は `g` を含むが、`KeysFor` は `ViewerKeys + "?"` で除くので、ブラウザでも同じ文字は選べない (ブラウザにはキーの衝突が無いので、WEB2 で外すかどうか)

## WEB2 (ブラウザの画面) で足りなそうなもの

- 変わったことを知らせる仕組み: いまは `Report.Stale` と `Board.Poll` を呼ぶ側が回す。サーバーからブラウザへ押し出す (SSE など) 場合は、その間隔とまとめ方を決める
- 画像: コメントの画像は番号 (`Comment.Images`) までで、ファイルを配る口が無い (`board.Store` の画像のパスから配る必要がある)。貼り付けも、ブラウザからのアップロードを `Store.SaveImage` に渡す口が要る
- 複数の画面: `Hist` (取り消しの履歴) は `Board` ごとなので、ブラウザのタブごと (あるいは接続ごと) に `Board` を持つか決める。読んだ印 (`viewed`) は置き場で共有される
- `Page` を JSON で出すときの形: `Thing.Node` (元の `report.Node`) は出さない前提。`Tone` `Kind` `Style` `Mark` は数なので、名前で出すなら変換が要る
- `Seen` は 3 つの文字列。ブラウザに持たせて操作に添えて返してもらう (ハッシュにしてもよい)
- 開閉の初期状態 (`Fold` と `Ask` は閉じて始まる、お知らせの中身は閉じて始まる) は viewer の決まりのまま。ブラウザでも同じにするなら page に「最初に開いているか」を持たせる
- お知らせを上に固定する高さの決まり (画面の 1/3) や、欄に出すコメントの数 (3 件) は端末の都合として viewer に残した。ブラウザでは別に決める
- 起こす処理 (`Send`) は数秒かかり得る。HTTP の応答をそのあいだ待たせるか、受けてから結果を別に知らせるか
