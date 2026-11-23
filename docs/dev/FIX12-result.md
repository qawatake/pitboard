# FIX12 の結果

- page: 決まりを `page.QuoteRows` (3) と `page.QuoteText` (改行→空白、描き方の説明つき) に置いた (`internal/page/lines.go`)
- viewer: `re` の Item は 1 行目 (カーソル・印・右端の語) が `> ` + コメント、題と同じ色・太さ。3 行で `…`。題は次の行から引用の文の列にそろえて折り返し、常に出す (`quoted`、`rows.go`)。コメントが無ければふつうの Item
- 上に描く引用 (`quote`・`row.lead` とその scroll の処理) を消した
- `-h` の `re` と作法の文、SPEC-format.md・SPEC.md を新しい見え方に。`TestQuoteAThought` を書き直し。vet/test/build 済、pty (scratchpad、幅 80/40、開閉と `j`) で確認
- 推測: 隠した行・済んだ行では引用も題も薄い (行の色に従う)。開いたとき題は太字、引用は太字にしない
- 推測: 右端の語は引用の 1 行目に。入らないほど長いときは引用の下・題の上の行に出す
