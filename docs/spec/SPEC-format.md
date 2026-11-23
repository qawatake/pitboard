# 報告の書き方 (SPEC.md の「報告の書き方 (部品)」を置き換える)

理由: Markdown に印 (`^id`、`{状態}`、`[?]`) を足す書き方は、試作が流用できるという作る側の都合で選んでいた。
あるべき姿から選び直す。印を足す方式は、部品や属性が増えるたびに新しい記号を発明することになり、
ふつうの文と衝突し、書き間違いが「ただの文」として黙って描かれてしまう。
部品は名前で呼び、属性は名前つきで渡し、知らないものは `check` がエラーにする。

## 形

行頭が `<大文字で始まる名前` の行が部品の開始、`</名前>` が終わり。それ以外の行は文 (インラインの Markdown: `code`、**強調**、[題](url))。
部品名の大文字小文字違い (`<item` `</section>` など) は `check` のエラー (部品として読みはする)。属性の書き損じ (`lang=diff` など) もエラー。
タグは必ず行頭 (字下げは可) に置く。文の途中の `<` は部品ではない。

    <Title>ita を作る</Title>

    inbox を広げた板を作る。いまは報告の書き方を決めている。

    <Section title="あなたへ">
      <Ask id="unit">ページの単位
        <Option key="a">セッションごと</Option>
        <Option key="b">プロジェクトごと</Option>
      </Ask>
      <Item id="hook">hook を入れるか決める
        入れると、書いたことが AI の作業中にも届く。`~/.claude/settings.json` を触る。
      </Item>
    </Section>

    <Section title="進行中">
      <Item id="impl" state="doing" status="作業中">最小版を実装する</Item>
      <Item id="parts" status="あなた待ち" note="上の問いの答え">部品の一覧を決める</Item>
    </Section>

    <Section title="見せたいもの">
      <Link id="spec" path="pitboard/SPEC.md">最小版の仕様</Link>
      <Link id="pr" url="https://example.com/pull/1">プルリクエスト</Link>
      <Code lang="diff">
      -old
      +new
      </Code>
    </Section>

    <Fold title="済んだこと (2)">
      <Item id="tui" state="done">viewer を 1 列にする</Item>
      - id の要らない、ただの行
    </Fold>

## 部品 (これ以外は `check` がエラーにする)

| 部品 | 属性 | 中身 | 人ができる反応 |
|---|---|---|---|
| `Title` | - | 題 (1 行) | - |
| `Section` | `title` (必須) | 部品と文 | - |
| `Item` | `id` (必須)、`state` = `todo` (既定) / `doing` / `done`、`status` (右端に出す短い語)、`note` (status の理由。開いたとき見せる) | 1 行目が題、2 行目以降が開いたときの中身 (文と `Code`) | `y` / `n` / 思いつき |
| `Ask` | `id` (必須) | 1 行目が問い、続けて `Option` を 2 つ以上 | 選択肢のキー / 思いつき |
| `Option` | `key` (必須。英小文字 1 字、`Ask` の中で一意。viewer のキー `c` `j` `k` `m` `n` `q` `x` `y` は不可) | 選択肢の文 (1 行) | - |
| `Link` | `id` (必須)、`path` か `url` のどちらか 1 つ (両方・どちらも無しはエラー。`url` は `http://` `https://` のみ) | 題 (1 行) | `y` / `n` / 思いつき (開かない。enter は何もしない) |
| `Fold` | `title` (必須)、`id` (任意。開閉とカーソルを id で追う。無ければページ内で何番目の Fold かで追い、題の変化では閉じない) | 部品と文。閉じた状態で始まる | enter で開閉 |
| `Code` | `lang` (任意。`diff` なら色を付ける) | 終わりのタグまで生のまま。字下げは開始タグの位置を基準に取り除く | - |
| (文) | - | 段落と `- ` の箇条書き | `- ` の行は `x` で隠せる |

- 属性は `名前="値"`。値の中の `"` は `&quot;`。知らない属性はエラー
- `id` は `[a-z0-9-]{1,16}`、ページ内で一意
- 入れ子: `Section` と `Fold` は `Item` `Ask` `Link` `Code` `Fold` と文を持てる。`Section` の中に `Section` は不可。`Item` の中は文と `Code` だけ
- `Link` は板の中でも外のアプリでも開かない。`→ 題` の下に、字下げしてパスか URL を薄く出す (常に見せ、折り返して `…` で切らない)。`path` は書かれたとおり、`url` は下線つきの OSC 8 ハイパーリンク

## 描き方

試作 `sketch/pv` と BRIEF 1〜3 の決まりをそのまま使う (行頭の印、右端の状態、余白、罫線なし、選択行は `>` だけ)。
`Item`: `todo` は `○`、`doing` は `~`、`done` は `✓` で薄く。中身か `note` があれば開ける (`todo`/`doing` は `▸`/`▾` を優先、`done` は `✓` のまま)。
`Ask` は `?`、選択肢は字下げして常に見せ、選ばれたものに印を付ける。`Link` は `→`。`Fold` は `▸`/`▾`。

カーソルが止まるのは `Item` `Ask` `Link` `Fold` と、`Section` `Fold` と最上位にある `- ` のただの行 (`Item` の中身の行には止まらない)。
`Ask` の印は、問いが長くて切れる (開ける) ときも `?` のまま。

## 解析が壊れないこと

解析はどんな入力でも panic しない。万一 viewer の再読み込みや描画で失敗しても落ちず、「報告を読めませんでした」と前の画面を残す。

## check が見ること

知らない部品・属性、必須の属性の欠け、`id` の重複と書式、閉じていない部品、入れ子の違反、`Option` が 2 つ未満、`state` の値。
行番号つきで出し、終了コード 1。viewer は、エラーのある報告でも落ちずに、読めた所までと、エラーの行を画面の上に出す。

## 置き場所 (SPEC.md の「置き場所」を置き換える)

板は **セッションごと**。`.pitboard/<session>/report.md` と `.pitboard/<session>/events.jsonl`。

- 板はカレントディレクトリから親へさかのぼって探す (`.pitboard/<session>/` が最初に見つかった所)。`changes` `settle` `check` `path` `view` と hook (`cwd` から) で同じ。ホームディレクトリより上へはさかのぼらない
- `path` で新しく作るときは、さかのぼって `.pitboard/` があればそこに、無ければカレントディレクトリに作る

- セッションは `pitboard -s SESSION <command>` か `$PITBOARD_SESSION`。`[A-Za-z0-9_-]+`。既存のセッションは一意な先頭数文字でよい
- viewer は `-s` を省くと、さかのぼっていちばん近い `.pitboard/` の中で、いちばん最近更新されたセッションを開く
- `pitboard path` が、そのセッションの report.md のパスを出す (AI がそこを編集する)
- `-s` はコマンドの直前か直後に書く (`pitboard -s abc path` / `pitboard path -s abc`)。それより後ろ (settle の TEXT など) の `-s` は文として残る
- viewer に存在しないセッションを渡すと「セッション abx はまだありません」と出す。viewer はセッションのディレクトリを作らない (作るのは `pitboard path`)
