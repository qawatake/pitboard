# pitboard

AI のセッションの横のペインに置く板。チャットは流れるので、流れては困るものをここに置く。

- **報告**は AI だけが書く。構成は自由で、部品 (`Item` `Ask` `Link` `Fold` など) は固定
- **人**は板の上で、少ない操作で反応する: `y` / `n`、選択肢のキー、行についてのコメント (`c`)、宛先なしのコメント (`m`)、行を隠す (`x`)
- AI が止まっているときは、板から AI を起こせる (`s`)。AI が止まる前に `pitboard wait` をバックグラウンドで動かしておけば、端末によらず届く (板の右上に `AI 待機中`)。それが使えなければ herdr か `PITBOARD_SEND` で端末に打ち込む
- 人の操作は報告を書き換えない。「どの行への何か」として記録され、AI が読んで報告を書き換える
- コメントには状態が付く (未送信 → 送信済 → 既読 → 対応中 / 保留 / 対応済)。書いたら忘れてよい

名前の見立て: レース中のドライバーには話しかけられないので、周回ごとにボードの前を通るときに読ませる。

依存は Go の標準ライブラリだけ。

## 使う

```sh
go install github.com/qawatake/pitboard@latest

pitboard            # viewer。横のペインで開いたままにする (いちばん最近更新された板)
pitboard -h         # AI 向けの使い方 (始め方、報告の書式、作法、hook、送る)
```

AI に使わせるには skill を入れる。skill は「`pitboard -h` を読む」とだけ書いた薄いもので、使い方は入っている版の `-h` から読む。

```sh
gh skill install qawatake/pitboard pitboard
# または
pitboard --skill > ~/.claude/skills/pitboard/SKILL.md
```

板はセッションごとに `.pitboard/<session>/` にできる (カレントディレクトリから親へさかのぼって探す)。

## 文書

- [仕様](docs/spec/SPEC.md)
- [報告の書き方](docs/spec/SPEC-format.md)
