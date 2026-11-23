# REVIEW8 結果: 文書と道具の説明の整合

## まとめ

1. SPEC.md に、置き換え済みの古い仕様 (`^id` の Markdown 書式、`.pitboard/report.md`、`ita`、`ctrl+e` で外のエディタ、「画像はやらない」) がそのまま残っている。いちばん大きい食い違いの源
2. 「行き先」(destination / where it goes) が `-h`・hook の出力に英語で残る。hook の出力は `settle N TEXT [N TEXT ...]` と、いまは通らない複数の形を勧めている (コードどうしの食い違い)
3. 「板」は SKILL.md の description に 1 か所。「済」「受け取った」「新しい」(新着の名残) は SPEC に 1 か所ずつ。緑の ● の呼び名「未読」は画面・SPEC・`-h` (unread) で揃っている
4. `u` が `Option` の禁止キーから漏れている (`report.go:71`、`-h`、SPEC-format、SPEC)。ほかの viewer キーに漏れは無い
5. 同じ説明が 3〜5 か所にあるのは: 緑の ● が消える条件、コメントの状態、送る順、`Option` の禁止キー、AI の作法。正を 1 つ決めて残りは参照にする案を d に書いた

行番号は、`README.md` `docs/spec/*.md` `skills/pitboard/SKILL.md` はファイルの行、`-h` は `pitboard -h` の出力の行 (コードは `main.go` の usage 文字列)。

---

## a. 古い語

| # | 場所 | いまの記述 | 直し方 |
|---|---|---|---|
| a1 | `skills/pitboard/SKILL.md:3` | `Use when the user mentions pitboard or "板"` / `Keeps a pitboard board` | 「板」を消す (ユーザーがまだ「板」と言うなら trigger として残す判断もある。要判断)。`Keeps a pitboard for the current session` に |
| a2 | `docs/spec/SPEC.md:1` | `# pitboard (仮称) 仕様 v0` | `(仮称)` を消す |
| a3 | `docs/spec/SPEC.md:73` | `試作 ita の editorLines と同じ` | 試作への言及ごと消す (`ita` は旧名) |
| a4 | `docs/spec/SPEC-format.md:14` | `<Title>ita を作る</Title>` | `<Title>pitboard を作る</Title>` など |
| a5 | `docs/spec/SPEC.md:199` | コメントの欄の例 `… 済` | `対応済` |
| a6 | `docs/spec/SPEC.md:349` | `受け取ったものには settle で…` | `届いたものには` (状態の語「既読」に合わせる) |
| a7 | `docs/spec/SPEC.md:218` | `緑 = 動いている・新しい (未読の ●、…)` | 「新しい」は新着の名残。`緑 = 動いている・読んでほしい` に |
| a8 | `-h` 9 (`main.go` usage) | `say how far thought N is and where it went` | `set thought N's state and what you did with it` |
| a9 | `-h` 272-273 | `the board shows it as read, without a destination` | `the board shows it as 既読 again, with no state from you` |
| a10 | `-h` 293-296 | `give every thought you received a destination` / `Never leave one without a destination` | `settle every thought …` / `Never leave one unsettled` |
| a11 | `hook.go:62` (と SPEC.md:327) | `Settle each thought with "…" once you know where it goes.` | `once you know what to do with it.` (b1 と一緒に直す) |
| a12 | `-h` 全体、SKILL.md:3,14、hook.go:62 | 英語の地の文で `thought` (旧「思いつき」) | SPEC.md:101 は「記録・コマンド・AI 向け出力の `thought N`」だけを変えないとしている。地の文は `comment` にし、`thought N` は「出力の語」として 1 回説明する案。`-h` 292 の `(The board calls them comments.)` をその説明に格上げ。要判断 |
| a13 | `-h` 全体 | 英語の `the board` | 「板」の英訳ではあるが、英語としては自然 (pitboard は board の一種)。残してよいと考える。揃えるなら `pitboard` に。要判断 |

「未読」(緑の ●) は `view.go:1394` (右上 `未読 N`)、SPEC.md:168-188、`-h` 149 `(unread)` で揃っている。`?` の一覧は「緑の ●」と呼び名を出さない (下の e7)。「思いつき」「行き先」「新着」は日本語では残っていない。

## b. 実装と違う記述 (コードが正)

| # | 場所 | いまの記述 | 実装 | 直し方 |
|---|---|---|---|---|
| b1 | `hook.go:62`、SPEC.md:327 | hook の出力が `settle N TEXT [N TEXT ...]` を勧める | `main.go:426-434`: 状態が先頭に無ければ 1 組だけで、`settle 9 a 10 b` は 9 に「a 10 b」が入る | コード側の不具合。`settle N [--doing\|--hold\|--done] TEXT (several: --STATE N TEXT ...)` に。SPEC.md:327 も |
| b2 | `docs/spec/SPEC-format.md:55`、`-h` 131、SPEC.md:190 | `Option` に使えないキー `c j k m n q r s x y` | `view.go:419` で `u` が `a.list`/報告より先に取り消しに使われる | g1 を参照。`u` を足す |
| b3 | SPEC.md:364 | `外のエディタ (ctrl+e)` | `ctrl+g` (`ctrl+e` は行末) | `ctrl+g` |
| b4 | SPEC.md:14-15, 221 | `.pitboard/report.md` `.pitboard/events.jsonl` | `.pitboard/<session>/…` | `<session>/` を入れる |
| b5 | SPEC.md:22-50, 342 | `^id`、`- [ ]`、`{状態}`、`<details>` の書き方、`必ず ^id を付ける` | SPEC-format.md の部品 (`Item id=…`) | 「報告の書き方 (部品)」の節のうち、表と `^id` の箇条を消して「SPEC-format.md」へのリンクにする。カーソル・再読み込みの決まり (29-34) は残す。342 は `Item` `Ask` `Link` に `id` を付ける、に |
| b6 | SPEC.md:370 | 今回やらないこと: `ブラウザ、画像、…` | 画像は実装済み (SPEC.md:227-238 の「画像を貼る」) | 「画像」を消す。節ごと消してもよい |
| b7 | SPEC-format.md:122 | 存在しないセッションで「セッション abx はまだありません」 | `view.go:2375` 「セッション %s の報告はまだありません」(SPEC.md:224 と同じ) | SPEC-format 側を合わせる |
| b8 | SPEC.md:155 | 案内に `tab` `q` `H` `s` は出さない | `s 送る` は未送信があれば出る (`view.go:1773`)。`u 取り消す` `space 次へ` も出る | `tab` `q` `H` だけに。`s` `u` `space` は条件つきで出ると書く |
| b9 | SPEC.md:158 | 一覧の案内は `enter 開く` `tab 報告へ` `m 書く` `? キー` | `view.go:1651-1663`: 行の上では `enter 開く  e 直す  x 取り消す`、コメントが無いときだけ `tab 報告へ` | 158 を消し、255 に一本化 |
| b10 | SPEC.md:255 | コメントが無ければ `tab 報告へ  ? キー` | `m 書く` も出る (`oneLine` が常に足す) | `tab 報告へ  m 書く  ? キー` |
| b11 | README.md:8 | `herdr か PITBOARD_SEND で端末に打ち込む` | `send.go:68-`: `PITBOARD_SEND` が先、無ければ herdr | `PITBOARD_SEND か herdr で` |
| b12 | SPEC.md:31 | 置かれたカーソルで効くのは `c` `m` `enter` | `r` も効く (`readTarget` は `placed` を見ない) | `r` を足す (意図と違えばコード側) |
| b13 | SPEC.md:360 | `pitboard/go.mod (module pitboard)` | `module github.com/qawatake/pitboard` | 直すか、「作り」の節ごと整理 (f2) |
| b14 | `-h` 137-138 | Fold の `id`: `keeps it open across retitling when folds are added or removed above it` | SPEC-format.md:57: id が無くても題の変化では閉じない。id が効くのは上の Fold が増減したとき | `id (optional; keeps it open when folds above it are added or removed)` |

緑の ● が消える条件 (`r`、`y`/`n`・答え、`c` で置いたコメント、`x` の隠す/戻す、`A`) は、README.md:7・SPEC.md:176-180・`-h` 150-152・`?` の `r` の行でコードと合っている (README は「答え」を省いているだけ)。キーの意味 (`space` `r` `u` `A` `x` `e` `s` `H`)、属性 (`ask` `wait` `look`)、`settle` の形 (`-h` 8-10, 297-305、SPEC.md:265-269)、`wait` の振る舞いは合っている。

## c. 文書どうしの食い違い

| # | 場所 | 食い違い | 直し方 |
|---|---|---|---|
| c1 | SPEC.md:123 と SPEC.md:313、`-h` 333-346 | 123 は起こし方を wait → PITBOARD_SEND → herdr、313 と `-h` は wait → hook → 打ち込み。hook は `s` の起こし方ではなく「届く道」 | 313 を「届く道は wait、hook、端末への打ち込み。`s` が起こすのは wait か打ち込み」に。`-h` の 2. も「(not woken by s; delivers while you work)」と添える |
| c2 | README.md:25 と SKILL.md、SPEC.md:335 | README は skill を「`-h` を読む」と**だけ**書いた、SPEC は「数行だけ」。実物は wait・settle・look の作法を 1 行ずつ持つ | README を「`-h` を読む、と要点数行だけの薄いもの」に |
| c3 | SPEC.md:110, 302 と SPEC.md:334-338 | 「`-h` と skill にも書く」。skill は薄くする方針 | 「`-h` に書く」 |
| c4 | SPEC.md:14-15, 221 と SPEC.md:17、SPEC-format.md:113 | 置き場所 (b4) | b4 |
| c5 | SPEC.md:22-50 と SPEC-format.md 全体 | 部品の書き方が 2 通り (b5) | b5 |
| c6 | SPEC.md:158 と SPEC.md:255 | 一覧の案内 (b9) | b9 |
| c7 | SPEC.md:224 と SPEC-format.md:122 | 存在しないセッションの文言 (b7) | b7 |
| c8 | SPEC.md:155 と SPEC.md:127, 189, 248 | 案内に `s` を出さない / 出す (b8) | b8 |
| c9 | SPEC.md:99 | `(--done。どれも薄く。いちばん新しい settle が有効)。薄く)。` と括弧が壊れている | `→ AI が settle で書いた状態: 対応中 / 保留 / 対応済 (いちばん新しい settle が有効)。送信済から後はどれも薄く。` |

## d. 重複 (3 か所以上)

| # | 何 | ある場所 | 提案 |
|---|---|---|---|
| d1 | 緑の ● が消える条件 | README.md:7、SPEC.md:176-180、`-h` 150-152、`?` の `r`、(SKILL.md:15 は付け方) | 正は SPEC.md:176-180。`-h` は AI に要る「人が `r` か反応で消す。カーソルでは消えない」の 1 文に。README は「読んだら `r`」だけに。`?` は今のままでよい |
| d2 | コメントの状態の並び | README.md:10、SPEC.md:99, 141, 213-214、`-h` 306-308、(`store.go:827-851`) | 正は SPEC.md:99。141 と 213-214 は「(上の状態)」と参照に。README と `-h` は語の並びだけ (今のまま) |
| d3 | 送る順と手段 | README.md:8、SPEC.md:123, 131, 306-313、`-h` 328-361、`?` の `s`、`send.go:27` | 正は SPEC.md「送る」123。313 は c1 のとおり 1 文に縮める。README は人がすることだけ (f1)。`-h` は AI がすること (wait を動かす / `pitboard:` 行が来たら changes) が中心なので今のままでよい |
| d4 | `Option` に使えないキー | SPEC-format.md:55、SPEC.md:84, 190、`-h` 131、`report.go:71` | 正は `report.go:71` の `viewerKeys`。`-h` は `main.go:153` に手で書いているので、`viewerKeys` から組み立てる (今回の `u` の漏れはこの手書きが原因の一つ)。SPEC-format.md:55 だけに一覧を置き、SPEC.md:84, 190 は「`Option` のキーに使える/使えない」を言わず SPEC-format を参照 |
| d5 | AI の作法 | SPEC.md:340-356、`-h` 181-236、SKILL.md:11-15 | 正は `-h` (AI はそれを読む)。SPEC.md:340-356 は「作法は `-h` の Manners」と 1 行に (いま `^id` 342 や「受け取った」349 のように古くなっている)。SKILL は今の数行でよい |
| d6 | 隠した行がまた現れる条件 | SPEC.md:109-110、`-h` 285-289、SPEC.md:302 | 正は SPEC.md:109-110。302 は c3 のとおり |
| d7 | `u` で取り消せる操作 | README.md:6、SPEC.md:59, 242、`?` の `u` | 正は SPEC.md:242。59 は記録の形だけにして、対象の列挙を消す |

## e. 抜け

**実装にあるのに文書に無い**

| # | 何 | 直し方 |
|---|---|---|
| e1 | 人が hook を入れる手順 (`~/.claude/settings.json`) は `-h` (AI 向け) にしか無い。`-h` 326 は「AI には入っているか分からない」 | README に「hook を入れると作業中にも届く。設定例は `pitboard -h` の Hooks」の 1 行 |
| e2 | `PITBOARD_SEND` の設定 (人がする) も `-h` だけ | README に 1 行 (tmux の例は `-h` を参照) |
| e3 | README に viewer のキーの入口が無い (`?` で一覧、`space` `tab` `H` `A` など) | README に「キーは viewer で `?`」の 1 行 |
| e4 | 画像を貼る (`ctrl+v`、パスの貼り付け) は README に無い | README の人の操作の箇条に一言 |
| e5 | `settle` の `-doing` `-hold` `-done` (1 本線) も通る (`main.go:409-414`) | 意図的なら SPEC.md:265 に。そうでなければ放置でよい |
| e6 | `ctrl+c` で viewer が終わる (`view.go:401`) | `?` の `q` の行に `(ctrl+c も)` |
| e7 | 右上の `未読 N` `反応待ち N` `AI 待機中` の意味が `?` の一覧に無い (`?` は「緑の ●」と呼ぶ) | `?` の `space` か `r` の行で「緑の ● (右上の未読)」と結ぶ |
| e8 | コメントの欄・一覧でも `space` `A` `u` が効く (`view.go:419, 1063-1066`)。SPEC.md:209 の「今までどおり」の列挙に無い | SPEC.md:209 に足す (一覧では `space` `A` は効かない、も) |

**消えた機能の説明が残っている**: b3 (`ctrl+e`)、b5 (`^id` の書式)、a3 (`ita` の試作)、SPEC.md:3, 24 と SPEC-format.md:68 の `sketch/pv` `BRIEF*.md` (この repo に無い。「試作の決まりをそのまま使う」は読み手が辿れない)。b6 は逆に、ある機能を「やらない」と書いている。

## f. 読み手に合っているか

| # | 場所 | 問題 | 直し方 |
|---|---|---|---|
| f1 | README.md:8 | 初めての人に「AI が止まる前に `pitboard wait` をバックグラウンドで…」は AI の仕事の説明。人がすること (届かないときの設定) が見えにくい | 「`s` で AI に送る。AI が待っていれば (右上 `AI 待機中`) すぐ届く。届かない端末では `PITBOARD_SEND` を設定する (`pitboard -h` の Send)」 |
| f2 | SPEC.md:358-366「作り」、368-370「今回やらないこと」 | 実装者への依頼書の文 (`sketch/` を変更しない、git は触らない、inbox 不要、今回) が仕様に残っている | 依頼に属する行を消す。残すのは「標準ライブラリだけ」「0.5 秒ごとに見る」「反応が人の指していない行に付かない」「シグナルで端末を戻す」 |
| f3 | SPEC.md:340-356 | AI 向けの作法の本文が仕様にある | d5 |
| f4 | `-h` 306-308 | 日本語の状態の語を英訳つきで出している | AI が人の画面の語を知るためなので妥当。そのまま |
| f5 | `?` の一覧 | `x (コメント)` と `e (コメント)` が `enter` と `y` の間にあり、報告の `x` (隠す) より先に出る | コメントの欄の行 (`x (コメント)` `e (コメント)`) を `tab` の近くにまとめる |

## g. `Option` のキーと viewer のキーの突き合わせ

報告の画面で viewer が先に取る小文字のキー (`view.go:395-490`): `? u q j k (space) A r m s x H (tab enter)`、`react` の中で `y n c`。英小文字 1 字に限ると **c j k m n q r s u x y**。`e` は欄と一覧だけ (報告では `react` に回るので選択肢に使える。SPEC.md:84 のとおり)。

| # | 漏れ | どうなるか | 直し方 |
|---|---|---|---|
| g1 | `u` | `check` を通る (`report.go:71` の `viewerKeys = "cjkmnqrsxy"`)。`keysFor` (`view.go:527`) も選択肢として案内に `u` を出すが、押すと `view.go:419` で取り消しになり、答えられない | `viewerKeys` に `u` を足す。`-h` 131 (`main.go:153`)、SPEC-format.md:55、SPEC.md:190 も (d4 のとおり `-h` は `viewerKeys` から組み立てる) |

ほかの漏れは無い (`A` `H` は大文字、`space` `?` は英小文字でないので `check` がもともと弾く)。
