# REVIEW6 の結果: 画面の言葉づかいと、repo の構造

## まとめ

- 言葉: いちばんの問題は「送る」が **AI に送る** と **スクロール** の 2 つの意味で使われていること、同じ「送ったが AI が読んでいない」状態を右上は `送信済み`、思いつきは `未読` と呼んでいること、報告を「ページ」「報告」「板」の 3 語で呼んでいること
- 状態の 4 語は **未送信 → 送信済み → 既読 → 対応済み** を勧める (メッセージアプリの言い方。右上の `送信済み N` とそのまま揃う)。「思いつき」は **コメント** への改名を勧める (`c` キーとも合う)
- 文が事実と違う所が 2 つある: `?` の enter の説明 (「次に AI が動いたときに届く」→ 実際は送るまで届かない) と、外部エディタの「この線より上が送られます」(→ 実際は送らずに置くだけ)
- 構造: `cmd/pitboard` (main・コマンド・hook・skill) + `internal/report` (書式の解析) + `internal/board` (記録) + `internal/viewer` (画面・端末・起こす) の 4 つを勧める。SKILL.md は Go の定数に持ち、ファイルとの一致をテストで守る
- テストは `featN_test.go` `fixN_test.go` を中身の名前に付け直し、`view.go` (2334 行) は同じパッケージのままファイルを 5〜6 に割る

---

## 1. 画面の言葉づかい

### 1-a. 全体で見つけた問題

| # | 問題 | 場所 |
|---|---|---|
| a1 | **「送る」が 2 つの意味**: AI に送る (`s`, `ctrl+s`, 「s で送る」) と、スクロール (`j k ↑↓ 送る`、「半画面ずつ送る」「1 行送る」「上の報告を送る」)。案内の行に `j k ↑↓ 送る` と `s` の説明が並ぶと取り違える | `view.go:1270` `:1287-1288` `:1317` `:1443` |
| a2 | **同じ状態に 2 語**: 送ったが AI がまだ読んでいないものを、右上は `送信済み N`、思いつきの状態は `未読` と呼ぶ | `view.go:1181` `store.go:545` |
| a3 | **報告に 3 語**: 「ページが変わった」(msgChanged, msgMoved)、「報告」(ほとんど)、「板はまだありません。AI が報告を書くと…」 | `view.go:20-24` |
| a4 | **板とセッション**: 板が無いときは「板はまだありません」、`-s` の板が無いときは「セッション %s はまだありません」。人から見ると同じ物 | `view.go:24` `:2053` |
| a5 | **「済」が 2 つの意味**: 思いつきの「済」(AI が行き先を書いた) と、右上の「送信済み」、仕様の項目「済み」(`[x]`)。思いつきの「済」だけ送り仮名なし | `store.go:541` |
| a6 | **「戻す」が 4 つの意味**: 隠した行を出す (`x 戻す`、「戻しました」)、取り消しを戻す (`u で戻す`、「戻しました」)、画面の移動 (`tab 報告へ`、「報告に戻る」「k で報告へ戻る」)、カーソルを戻す。メッセージ「戻しました」は 2 つの操作で同じ文 | `view.go:613` `:1039` |
| a7 | **事実と違う**: `?` の `enter 書く (AI には、次に AI が動いたときに届く)`。仕様では送る (`s`) までは届かない | `view.go:1306` |
| a8 | **事実と違う**: 外部エディタの `この線より上が送られます。空のまま閉じると送りません` と「空だったので送りませんでした」。実際は `write` するだけで送らない (enter と同じ) | `view.go:864` `:878` |
| a9 | **道具の内側の言葉**: 「送る印は付けました」(send の境界のこと)、「行き先」(`settle` の訳。AI 側の都合)、「受け取った」(`changes` を読んだこと) | `view.go:838` `:1796` `:2219` `store.go:547` |
| a10 | **書く**: `m 書く`、`c 行に書く`、エディタの見出し「思いつきを書く」は「書き始める」。エディタの `enter 書く` は「書き終えて板に置く」。同じ語で始まりと終わりを指す | `view.go:1887` |
| a11 | 「ターミナル」と「端末」の混在 (`?` の `s` だけ「ターミナル」) | `view.go:1298` |
| a12 | 「AI」「あなた」: 画面の文は人を主語にせず、相手は一貫して「AI」。よい。ただし行の状態の色付けが `あなた待ち` の前方一致だけで、`pitboard -h` の例は英語 (`waiting on you`) なので、英語で書いた報告は色が付かない (言葉というより仕様の穴) | `view.go:1975` `main.go:84` |
| a13 | (画面の外) AI に渡す `changes` の出力は英語なのに、`→ いまは: …` `→ いまは無い` だけ日本語。AI 向けは英語に揃えるほうがよい (`→ now: …` `→ now gone`)。`-h` の例とテストも合わせて変える | `store.go:402-406` `main.go:161-175` |

### 1-b. 「思いつき」という呼び名

人が板に書く短い文で、問い・頼みごと・感想・メモが混ざる。「思いつき」は「ふと浮かんだアイデア」に寄っていて、問いや頼みごとを書くときに合わない。4 字と長めで、欄の見出しや「思いつきを書く」が重い。

| 候補 | よい点 | 弱い点 |
|---|---|---|
| **コメント** (勧める) | 行に付けるもの (`c`) としてだれでも知っている (コードレビュー・共有文書と同じ)。問い・頼み・感想・メモどれでも違和感がない。`c` キーの覚え方と合う。AI が「対応」する流れも自然 | カタカナ 4 字。宛先なし (`m`) のものも「コメント」になるが、共有文書でも全体へのコメントはあるので問題ない |
| 付箋 | 板の見立てに合う。2 字。貼って、片付いたら剥がす | 「付箋を書く」は少しぎこちない。問いを書く物という感じが弱い |
| ひとこと | 柔らかい。長さを示す | 長い文も書ける (ctrl+g) ので名前とずれる。「ひとことの一覧」が座りが悪い |
| メモ | 短い | 自分用の覚え書きに聞こえ、AI に届く物だと伝わらない |
| そのまま (思いつき) | 変える手間がない | 上のとおり |

内部の名前 (`thought`、記録の `op: "thought"`、`settle`) は変えない。記録の形式は既存の板と AI の `-h` が読むので、画面の語だけを変える。

### 1-c. 思いつき (コメント) の 4 つの状態

人から見て起きていること: (1) 板にあるがまだ送っていない → (2) 送ったが AI はまだ読んでいない → (3) AI が読んだ、まだ何もしていない → (4) AI がどう扱ったかを書いた (開くと読める)。

**案 A (勧める): 未送信 → 送信済み → 既読 → 対応済み**

- メッセージアプリ (送信 → 既読) と同じ並びなので、説明なしで (2)(3) の違いが分かる
- 右上の `送信済み N` と (2) が同じ語になり、a2 が消える
- (4) を「対応済み」にすると「済」の意味が 1 つになる (a5)。行き先が「やらない」「答えた」でも「対応した」で包める
- 全部体言。幅は 3〜4 字で、右端に置いて揃う

**案 B: まだ → 送った → 読んだ → 返事あり**

- かなと動詞の過去形で、主語を人と AI が交互にとる素朴な言い方
- 「返事あり」は開くと読めることを示す
- 弱い点: 「まだ」だけでは何がまだか分からない。右上の数 (`送った 2`) が文として不自然。体言 (返事あり) と動詞が混ざる

勧めるのは **案 A**。開いたときの説明文も揃える (下の表の t-xx)。

### 1-d. 用語の表

「そのまま」は変えなくてよいもの。コメント改名を採る前提で書く (採らないなら「コメント」を「思いつき」に読み替える)。

**状態・数**

| # | いまの語 | 提案 | 理由 |
|---|---|---|---|
| s1 | 未送信 (状態) | そのまま | 意味がはっきりしている |
| s2 | 未読 (状態) | 送信済み | a2。右上と揃える |
| s3 | 受け取った (状態) | 既読 | a9。動詞の過去形で浮いている。「受け取る」は AI 側の都合 |
| s4 | 済 (状態) | 対応済み | a5。何が済んだか分からない |
| s5 | `送信済み N` (右上) | そのまま | s2 と揃う |
| s6 | `● 未送信 N` (右上) | そのまま | |
| s7 | `s で送る` (欄の見出しの右) | そのまま | |
| s8 | `✎2` | そのまま | |
| s9 | `隠した行 3` / `隠した行 3 (表示中)` | そのまま | |

**開いたコメントの文**

| # | いまの語 | 提案 | 理由 |
|---|---|---|---|
| t1 | まだ送っていません (s で送る) | そのまま | |
| t2 | 送りました。AI はまだ読んでいません | そのまま | 状態語 (送信済み) と合っている |
| t3 | AI は受け取りました。行き先はまだです | AI は読みました。対応はまだです | s3・s4 と揃える |
| t4 | 行き先: … | 対応: … | a9。「行き先」は settle の直訳 |
| t5 | 行: … | そのまま | |
| t6 | (書いたときの題: …) | (書いたときは: …) | 「題」は報告の書式の言葉。人には「その行が書いたときどうだったか」で足りる |
| t7 | (報告にもうない行) | そのまま | |
| t8 | 理由: … | そのまま | |

**見出し・空のとき**

| # | いまの語 | 提案 | 理由 |
|---|---|---|---|
| h1 | 思いつき (欄・一覧の見出し) | コメント | 1-b |
| h2 | 思いつきを書く (エディタの見出し) | コメントを書く | 1-b |
| h3 | 「…」に書く | 「…」にコメント | 何を書くかを言う |
| h4 | まだありません。m で書けます | コメントはまだありません。m で書けます | 一覧で主語が無いと何が無いのか分からない |
| h5 | キー (`?` の見出し) | そのまま | |
| h6 | AI はまだ報告を書いていません | そのまま | |
| h7 | 板はまだありません。AI が報告を書くと、ここに出ます | そのまま | 板 = セッションごとの物、で揃える |
| h8 | セッション %s はまだありません | 板 %s はまだありません | a4。id を見せるので「板」+ id で足りる |
| h9 | このディレクトリと親に板 (.pitboard/) がありません。AI のセッションを動かしているディレクトリで起動してください | このディレクトリと親に板 (.pitboard/) がありません。AI を動かしているディレクトリで開いてください | 「AI のセッション」は内側寄り。短くする |
| h10 | 報告に書き方の誤りがあります (読めた所までを出しています) | そのまま | |
| h11 | N 行目: … | そのまま | |
| h12 | この行は書き換わりました | そのまま | |

**知らせ (下の行)**

| # | いまの語 | 提案 | 理由 |
|---|---|---|---|
| m1 | ページが変わったのでやり直してください | 報告が書き換わりました。もう一度押してください | a3。何をやり直すかを言う |
| m2 | ページが変わりました。カーソルの行を確かめてください | 報告が書き換わりました。カーソルの行を確かめてください | a3 |
| m3 | 報告を読めませんでした (前の画面を出しています): … | そのまま | |
| m4 | その操作でエラーが起きました: … | できませんでした: … | ほかの「〜できませんでした」と揃える |
| m5 | 記録できませんでした / 書けませんでした | どちらも「記録できませんでした」 | 同じ失敗 (記録への追記) に 2 語 |
| m6 | 隠しました (H で表示) / 隠しました | そのまま | |
| m7 | 戻しました (隠した行) | 表示に戻しました | a6。取り消しの「戻しました」と分ける |
| m8 | 取り消しました (u で戻す) | 取り消しました (u で元に戻す) | a6 |
| m9 | 戻しました (取り消しの取り消し) | 元に戻しました | a6 |
| m10 | 取り消せませんでした / 戻せませんでした | そのまま / 元に戻せませんでした | |
| m11 | 送るものはありません | 未送信のものはありません | 状態語と揃える |
| m12 | 送っています… / AI に送りました | そのまま | |
| m13 | 送る印は付けました。AI を起こせなかったので、チャットで一言伝えてください。 | 送りました。ただ AI を起こせなかったので、チャットで一言伝えてください | a9。「印」は内側の言葉。人にとっては送れている |
| m14 | 送れませんでした: … | そのまま | |
| m15 | 空だったので送りませんでした (外部エディタ) | 空だったので書きませんでした | a8 |
| m16 | 描画できませんでした / q 終了 | そのまま | |
| m17 | AI を起こす方法がありません。PITBOARD_SEND を設定してください (pitboard -h の "Send") | そのまま | |
| m18 | herdr にこのセッション (%s) の AI がいません。… | そのまま | herdr の文脈なので「セッション」でよい |

**案内の行・`?` の一覧**

| # | いまの語 | 提案 | 理由 |
|---|---|---|---|
| k1 | `j k ↑↓ 送る` (`?` 表示中・欄) | `j k ↑↓ スクロール` | a1 |
| k2 | `pgup pgdn 半画面ずつ送る` | `半画面ずつスクロール` | a1 |
| k3 | `j k` の説明「…なければ 1 行送る…」 | 「…なければ 1 行スクロール…」 | a1 |
| k4 | `pgup pgdn 上の報告を送る` (書いているとき) | `上の報告をスクロール` | a1 |
| k5 | `enter 書く (AI には、次に AI が動いたときに届く)` | `enter 板に置く (s で送るまで AI には届かない)` | a7 (事実の誤り)、a10 |
| k6 | エディタの案内 `enter 書く  ctrl+s 送る  esc やめる` | `enter 置く  ctrl+s 送る  esc やめる` | a10。「書く」は書き始めの語に残す。置く = 板に残すが送らない、で ctrl+s との差が出る |
| k7 | 外部エディタの `# …: この線より上が送られます。空のまま閉じると送りません。` | `# …: この線より上を板に置きます。空のまま閉じると何もしません。` | a8 |
| k8 | `ctrl+s 書いて、すぐ AI に送る` | `置いて、すぐ AI に送る` | k6 と揃える |
| k9 | `s AI に送る: まだ届いていないものがあると、AI のいるターミナルに一行打ち込んで知らせる` | `s AI に送る: 未送信のものをまとめて送り、AI の端末に 1 行打ち込んで起こす` | a11、状態語と揃える |
| k10 | `x 隠す` / `x 戻す` (隠した行の上) | `x 隠す` / `x 表示に戻す` | a6 |
| k11 | `x (思いつき) …その思いつきを取り消す。直後の u で戻す` | `…そのコメントを取り消す。直後の u で元に戻す` | a6 |
| k12 | `x 取り消す` (欄・一覧) | そのまま | |
| k13 | `y よし` / `n だめ` | そのまま | 短く、y/n の覚え方とも合う |
| k14 | `a b … 答える` | そのまま | |
| k15 | `c 行に書く` | `c 行にコメント` | 1-b。何を書くかを言う (幅が足りなければ `行に書く` のまま) |
| k16 | `m 書く` | そのまま | 見出しが「コメント」なら足りる |
| k17 | `j k ↑↓ 動かすと答えられる` / `動かすと y n が効く` / `動かすと x が効く` (短縮 `→ 答える` 等) | そのまま | 理由が分かれば十分。より短くするなら「動かすと効く」に統一 |
| k18 | `tab 報告へ` / `tab 思いつきの一覧 / 報告に戻る` | `tab 報告へ` / `tab コメントの一覧 / 報告へ` | a6 (移動は「へ」で揃える) |
| k19 | `enter 開く` / `閉じる`、`? キー`、`q 閉じる` / `q 終了` | そのまま | |
| k20 | `H 隠した行を表示する / しない` | そのまま | |
| k21 | `ctrl+g エディタ` / `$VISUAL / $EDITOR / vim で続きを書く` | そのまま | |
| k22 | 日本語入力の行、編集キーの行 (`1 字左 / 右` など) | そのまま | |

体言止めか動詞か: 案内の行のキー説明は動詞の終止形 (隠す、開く、送る、置く)、状態は体言 (未送信…対応済み)、見出しは名詞、で揃っている。上の提案もこれに従った。

1 行に収まるか: いま長いのは h9 と m13 で、どちらも 2 行になる前提の場所 (h9 は折り返す段落、m13 は 2 行の知らせ)。提案はどちらも少し短くした。

### 1-e. 直すときの注意

- 状態語・案内の語はテストが文字列で見ている (`feat*_test.go` など)。語を変えたら同じコミットでテストと `docs/spec/SPEC.md` (88-89 行、123-126 行、151-178 行) を直す
- `pitboard -h` (英語) は「thought」のままでよい。AI には内部の名前のほうが通じる

---

## 2. repo の構造

### 2-a. いまの姿

- 全部が repo 直下の `package main` (本体 約 4,900 行、テスト 約 2,500 行)
- 依存の向き (実際の呼び出しから): `main` → `hook` / `store` / `report` / `view`、`view` → `store` / `report` / `text` / `term` / `editor` / `send`、`store` → `page` (行の題を `changes` に書くため)
- 境界がずれている所:
  - `store.go` の `statusOf` が画面の語と色 (`"未送信", yellow`) を返している。記録が画面に依存している
  - `send.go` に、起こす処理 (`deliver`、herdr) と、記録の数え方 (`unsent`、`sentUnread`) が同居している
  - `term.go` に文字列用の `wrap` / `trunc`、`text.go` に装飾付きの `wrap` / `trunc` があり、名前が同じで別物
  - `page.go` は「ページ」ではなく報告の木 (`node`, `kind`) の定義。`report.go` はその解析器
  - `view.go` 2334 行に、アプリの状態・キー処理・エディタまわり・下の欄・一覧・`?`・描画が全部ある

### 2-b. 勧める構造

```
pitboard/
├── cmd/pitboard/
│   ├── main.go          main と run (コマンドの振り分け、-s の解釈)
│   ├── usage.go         -h の全文 (usage 定数)
│   ├── skill.go         SKILL.md と同じ文の定数 (下の 2-d)
│   ├── hook.go          pitboard hook
│   └── *_test.go        run・hook・skill のテスト
├── internal/
│   ├── report/          報告の書式: 木の型と解析。ファイルも端末も触らない
│   │   ├── node.go      (いまの page.go)
│   │   ├── parse.go     (いまの report.go)
│   │   └── parse_test.go (+ Fuzz)
│   ├── board/           .pitboard/<session>/ の記録: 探す、追記、changes、settle、再生
│   │   ├── find.go      sessions / ancestors / findSession / boardRoot
│   │   ├── events.go    Event、追記、ロック、Seen
│   │   ├── changes.go   lastSend / deliverable / describe / Changes / Settle、Unsent / SentUnread (いまの send.go から)
│   │   ├── state.go     replayEvents、thought、Status (語と色ではなく列挙を返す)
│   │   └── *_test.go
│   └── viewer/          画面: TUI 全体
│       ├── app.go       app、runViewer、loop、reload
│       ├── keys.go      handle / handleList / handlePane / react / hide / retract
│       ├── input.go     input、write、external (外部エディタ)
│       ├── editor.go    edit (いまのまま)
│       ├── wake.go      wake と deliver / herdrPane (いまの send.go の残り)
│       ├── draw.go      draw / compose / line / rows / contents / right / markOf
│       ├── bottom.go    案内の行、`?` の一覧、下の欄、コメントの一覧
│       ├── words.go     画面に出す日本語 (状態語・知らせ・案内) を 1 か所に
│       ├── styled.go    (いまの text.go。装飾付きの文字列)
│       ├── term.go      raw モード、キーの解析、幅 (いまの term.go)
│       └── *_test.go
├── skills/pitboard/SKILL.md   (gh skill install が読む。場所は変えない)
├── docs/spec/
├── go.mod
└── README.md
```

境界の理由:

- **report**: `docs/spec/SPEC-format.md` そのもの。純粋な関数で、Fuzz がある。AI が書く物の形を決める所なので、画面の都合が入らないよう切る価値がある
- **board**: ファイルの形式 (`events.jsonl`、`seen`) と、AI に何を届けるか (`changes`、送る境界) の規則。CLI (`changes` `settle` `hook`) と画面の両方が使う。ここが壊れると AI と人の話が食い違うので、画面から独立してテストしたい
- **viewer**: 残り全部。端末・エディタ・起こす処理を別パッケージにはしない (下の 2-f)

`internal/` の下なので、外から import できる物は 0 になる。

### 2-c. go install のパス

- 新しい: `go install github.com/qawatake/pitboard/cmd/pitboard@latest` (バイナリ名は `pitboard` のまま)
- 古いパス `github.com/qawatake/pitboard@latest` は、移したあとの版では「not a main package」で失敗する。古い版 (タグ) は引き続き古いパスで入る
- README の「使う」と、`pitboard -h` に install の行があればそれを直す。移す版でタグを切るなら、リリースノートに 1 行書く
- モジュールパス (`go.mod` の `module github.com/qawatake/pitboard`) は変えない

### 2-d. SKILL.md の埋め込み

`//go:embed` は親ディレクトリを指せないので、`cmd/pitboard` から `skills/pitboard/SKILL.md` は埋め込めない。案:

| 案 | 内容 | 判断 |
|---|---|---|
| **A (勧める)** | `cmd/pitboard/skill.go` に SKILL.md と同じ文を定数で持つ。テストで `../../skills/pitboard/SKILL.md` を読んで一致を確かめる (いまの `TestSkillIsThin` に足す) | SKILL.md は「`pitboard -h` を読め」だけの薄い物で、めったに変わらない。ずれたらテストが落ちる。公開物も増えない |
| B | repo 直下に `package pitboard` を残し、`embed.go` で `var Skill string` を公開する | ユーザーの「公開する物を減らす」に反する。ルートが import できるパッケージになり、README を読んだ人が混乱する |
| C | `go generate` で SKILL.md を `cmd/pitboard/` にコピーして埋め込む | 生成物をコミットする必要があり (go install は generate しない)、結局 2 部持つ。A よりも手間 |
| D | SKILL.md を `cmd/pitboard/` に移し、`skills/pitboard/` を symlink にする | embed は symlink を読まない。`gh skill install` 側の扱いも不確か |

A では、どちらを正とするかを決めておく: **SKILL.md を正、定数はそれに合わせる** (人が読んで直すのは md なので)。テストの失敗文に「SKILL.md に合わせて skill.go を直す」と書く。

### 2-e. テストと docs/dev

テストは各パッケージの中に置く (`_test.go` を同じディレクトリに)。いまの `featN` `fixN` は経緯の名前なので、中身で付け直す。移すときに一緒にやる:

| いまのテスト | 行き先 |
|---|---|
| `report_test.go` | `internal/report/parse_test.go` |
| `store_test.go`、`fix3` の Board/Ancestors/NewBoard/HookInputLimit 以外、`feat4` の Retract*Delivery / AfterSendBeforeRead、`feat5` の ChangesStopsAtTheLatestSend / SendRecordsOnly… | `internal/board/` の `find_test.go` `changes_test.go` `state_test.go` |
| `feat_test.go` の Hook*、`fix3` の Hook*/StopDoesNotBlock…、`feat4` の StopHookIgnoresTakenBack | `cmd/pitboard/hook_test.go` |
| `feat2` の SkillIsThin、`fix2` の SessionFlagOnlyNextToTheCommand、`store_test` の SessionFlagAfterCommand / CheckCommand | `cmd/pitboard/main_test.go` |
| `feat2` の Send* | `internal/viewer/wake_test.go` |
| `editor_test.go`、`feat2` の EditorCursor、`fix2` の PasteCRLF、`feat5` の KeysWithInputMethod | `internal/viewer/editor_test.go` `term_test.go` |
| 隠す (feat の Hide*、ChangedItemShowsAgain、fix3 の HideChecksTheFile / HMovesCursor) | `internal/viewer/hide_test.go` |
| 欄・一覧・取り消し (feat の ThoughtsPane、feat3 の Pane*、feat4 の RetractInPane / InList) | `internal/viewer/pane_test.go` |
| 案内の行・`?`・知らせ (feat の HelpScrolls / NarrowHints、feat2 の HintsShowM / UnsentMark、feat3 の MessagesAreOneLine / EditorHintsKeepEsc、feat4 の NoBoardMessages) | `internal/viewer/bottom_test.go` |
| 描画とカーソル (残り: JKBringTheCursorBack、LongAskKeepsItsMark、AskOpensToItsOptions、PlacedCursor*、ThoughtShowsTheTitle…、OpenedContentsColor、SmallDisplayFixes、TruncLongLineIsFast) | `internal/viewer/draw_test.go` `cursor_test.go` |

共有のテスト用の関数 (`testApp` `testStore` `screen` `plainLines` `sendAll` `fakeHerdr` など) は各パッケージの `helpers_test.go` に置く。board の `testStore` は viewer のテストでも要るので、`internal/board/boardtest` のような小さなパッケージにするか、viewer 側に同じ 10 行を持つ (後者で十分)。

**docs/dev/** は repo に残さないほうがよい。依頼と結果は、その時点のやり取りで、終わった物は仕様 (`docs/spec/`) とコミットに吸収されている。公開 repo で読む人には経緯のメモにしか見えない。勧める扱い:

- 進行中の依頼だけ `docs/dev/` に置き、終わったら消す (中身は git の履歴に残る)
- もしくは `docs/dev/` ごと `.gitignore` に入れ、エージェント間の受け渡しはローカルだけにする
- どちらにするかはエージェントの運用次第。いま別のエージェントが参照しているので、**構造を移すのとは別の、あとのコミットで**やる

### 2-f. ファイル名と中身

| ファイル | 合っているか | 提案 |
|---|---|---|
| `page.go` | 合っていない。報告の木の型 (`node` `kind`) と補助 | `report/node.go` |
| `report.go` | 合っている (解析器) | `report/parse.go` |
| `store.go` (549 行) | 名前は合うが中身が 4 つ: 板を探す / 記録の読み書き / changes の規則 / 再生と状態語 | 2-b のとおり 4 ファイルに。`statusOf` の語と色は viewer へ |
| `send.go` | 半分ずれ。数え方 (`unsent` `sentUnread`) は記録の規則 | 数え方は `board/changes.go`、起こす処理は `viewer/wake.go` |
| `text.go` | 名前が広すぎる (装飾付きの文字の列) | `viewer/styled.go`。`term.go` の文字列用 `wrap`/`trunc` と名前が重なるので、片方を `wrapPlain` などに |
| `term.go` | 合っている (raw モード、キー、幅) | そのまま |
| `editor.go` | 合っている | そのまま |
| `hook.go` | 合っている | `cmd/pitboard/hook.go` |
| `main.go` (384 行) | 半分が `usage` の文 | `usage.go` を分ける |
| `view.go` (2334 行) | 大きすぎる | 2-b の `app/keys/input/draw/bottom/words` に割る。パッケージは分けない |

### 2-g. 移す手順 (機械的にできる順)

各段で `go build ./... && go test ./...` が通る状態でコミットする。別のエージェントの作業が落ち着いてから始める (ほぼ全ファイルに触るので、並行すると衝突する)。

1. **同じパッケージのままファイルを整理する** (動作は変えない)
   - テストのファイル名を 2-e の名前に付け直す (関数を移すだけ)
   - `view.go` を 2-b の viewer の各ファイルに割る、`main.go` から `usage.go` を出す、`page.go` → `node.go`、`text.go` → `styled.go`
   - `send.go` の数え方を `store.go` 側へ、`statusOf` を「状態の列挙を返す関数」と「語と色を返す関数 (view 側)」に分ける
   - ここまでは `package main` のままなので、差分は移動だけで読みやすい
2. **internal/report を切り出す** (直下の main から import する形で): `node.go` `parse.go` を移し、`package report`。viewer と board が使う物を公開名にする (`node` → `Node`、フィールド `kind text id label status reason ask path url lang code body kids line depth key` → 大文字、`parseReport` → `Parse`、`kind` 定数)。gopls の rename (`gopls rename` かエディタの Rename) で機械的にできる。いちばん手間がかかるのはここ (viewer が node のフィールドをほぼ全部使っている)
3. **internal/board を切り出す**: `Store` `Event` `newStore` → `Open`、`state` → `State`、`thought` → `Thought`、`replayEvents` → `Replay`、`sessions` `findSession` `boardRoot` など、viewer と cmd が使う物だけ公開
4. **internal/viewer を切り出す**: 画面のファイルを移し、`runViewer` → `viewer.Run` だけを公開
5. **最後に main を cmd/pitboard へ git mv する**: 直下に残った `main.go` `usage.go` `hook.go` とそのテストを移し、`skill.go` (2-d の A) を足す。internal を先に出しておくと、途中で main が 2 つできる状態を通らない
6. README・`docs/spec/SPEC.md` の install の行とファイル名の言及を直す。`.gitignore` の `/pitboard` はそのまま (直下で `go build ./cmd/pitboard` すると同じ名前で出る)

### 2-h. やらないほうがよいこと

- **パッケージを細かく割らない**: `internal/term` `internal/editor` `internal/wake` `internal/text` のように道具ごとに切ると、viewer の 1 つの変更 (例: 案内の行の幅) が 3 パッケージに散る。使う所が 1 か所 (viewer) しかない物は viewer の中のファイルで十分
- **記録の形式 (events.jsonl の `op` 名、`thought` `settle`) を改名しない**: 既存の板と、AI が読む `-h` と hook の出力が依存している。画面の語の変更 (1 章) と構造の移動のどちらとも混ぜない
- **言葉の変更と構造の移動を同じコミットにしない**: 移動の差分はリネームとして見えてほしい。中身の変更が混ざると git が移動を追えなくなる
- **ルートに公開パッケージを置かない** (2-d の B): 公開物を減らすという目的に反する
- **`pkg/` を作らない**: 外に出す物が無いので要らない
- **テストを `test/` などに集めない**: Go の作法 (同じディレクトリ) から外れ、非公開の関数をテストできなくなる
- **一気に全部やらない**: 2-g の各段でテストを通してコミットする。とくに 1 (ファイル整理) を独立させると、あとの段の差分が小さくなる
