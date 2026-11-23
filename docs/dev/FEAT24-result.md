# FEAT24 の結果

vet / test / build は通る。git には触っていない。

- 最上位の `Fold` (報告の木のいちばん外にあるもの) の題と行頭の `▸` / `▾` を、`Section` の見出しと同じ太字の青にした。開いたときも同じ (`rows`、`internal/viewer/rows.go`、`TestOuterFoldLikeASection`)
- `Section` や `Fold` の中の `Fold` は地の色のまま
- `H` で隠した行として出すときは今までどおり薄く (印も `⌀` のまま)
- 仕様書 SPEC-format.md の描き方に 1 行。`-h` には `Fold` の見た目の記述が無いので触っていない
