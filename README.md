# pitboard

AI のセッションの横のペインに置く板。チャットは流れるので、流れては困るものをここに置く。

- **報告**は AI だけが書く。構成は自由で、部品 (`Item` `Ask` `Link` `Fold` など) は固定
- **人**は板の上で、少ない操作で反応する: `y` / `n`、選択肢のキー、行についての思いつき (`c`)、宛先なしの思いつき (`m`)、行を隠す (`x`)
- AI が止まっているときは、板から AI を起こせる (`s`、書きながらなら `ctrl+s`)。herdr の中なら何も設定せずに、ほかの端末では `PITBOARD_SEND` で
- 人の操作は報告を書き換えない。「どの行への何か」として記録され、AI が読んで報告を書き換える
- 思いつきには行き先が付く (未読 → 受け取った → 済)。書いたら忘れてよい

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
