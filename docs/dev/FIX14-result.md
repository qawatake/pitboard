# FIX14 の結果

- `re` を削除: parser (`Node.Re`・隠す鍵の `|re=`・`reNumber`)、page (`Quote` `QuoteRows` `QuoteText`・`Thing.Quote`)、viewer (`quoted`・`quote_test.go`)。`re="…"` は `<Item> has no attribute "re"` (check で確認)
- `-h`: 例の返事を `> Is there one board per project?` + 中身 1 行目の結論に。作法と "A layout to start from" に、題は `> ` + 人の質問 (長ければ縮める・要約、1 行が望ましい、文の途中で止めず句の切れ目で完結させる)、中身 1 行目に結論 1 文、1 つで複数のコメントに答えてよい
- SPEC.md・SPEC-format.md を同じ内容に (`re` の行・check の一覧・隠す鍵から削除)。テストは `TestReAttributeGone` (エラーになる・`> ` の題はただの文) に置き換え。vet/test/build 済
- 推測: 以前 `re` 付きで隠した返事は、鍵から `|re=N` が消えるので、AI が `re` を外して書き直すと隠れていない扱いに戻る (題も変わるはずなので実害は小さい)
