# FEAT12: repo の構造を `cmd/pitboard` と `internal/` に分ける

**挙動は変えない。** 動かすだけの変更。この依頼だけで完結する。

## 読むもの

`docs/dev/REVIEW6-result.md` の「## 2. repo の構造」(2-a から最後まで)。そこに勧める構造、移す手順、やらないほうがよいことがある。それに従う。ただし、そのあとにファイルが増えている (`marks.go` `wait.go` `images.go` など) ので、同じ考え方で置き場所を決める。

## 決定 (REVIEW6 の提案への補足)

- パッケージは `cmd/pitboard` (main)、`internal/report` (報告の木と解析)、`internal/board` (`.pitboard/<session>/` の記録)、`internal/viewer` (画面) の 4 つ。これ以上は分けない
- 依存の向き: `cmd/pitboard` → `viewer` `board` `report`、`viewer` → `board` `report`、`board` → `report`。逆向きは作らない。`board` が画面の語や色を返している所は、列挙を返して `viewer` で語と色にする
- `go install` のパスは `github.com/qawatake/pitboard/cmd/pitboard@latest` になる。README を直す
- skill の埋め込みは、REVIEW6 の 2-d の勧めに従う。`skills/pitboard/SKILL.md` と埋め込んだ文が同じであることのテストを残す
- テスト: 経緯の名前 (`featN_test.go` `fixN_test.go`) をやめ、確かめている物の名前にして、それぞれのパッケージへ移す。テストの中身 (確かめていること) は減らさない。移す前後でテストの関数の数を数えて、結果に書く
- `view.go` は REVIEW6 の案のように役割で分ける。1 ファイル 600 行を目安に
- 公開する名前 (大文字) は、パッケージをまたぐのに要るものだけ
- `docs/dev/` はそのまま残す

## 進め方

- 小さな段階に分け、段階ごとに vet / test / build が通ること (機械的な移動 → 名前の公開 → 境界のずれの直し → テストの移動と改名、の順)
- 途中で仕様の食い違いやバグを見つけても直さない。結果に書く

## 終わったら

- vet / test / build。`go build ./cmd/pitboard` の実行ファイルで、pty 越しに 1 度、開く・`c`・`s`・`changes`・`settle`・`wait` が動くこと。スクラッチパッドの空のディレクトリで。どの repo の `.pitboard/` も触らない。実物の `herdr agent prompt` / `herdr pane send-*` は実行しない
- 仕様書・README の、パスに触れている所を直す
- git は触らない
- `docs/dev/FEAT12-result.md` に書く: できた構造の絵 (ファイルと行数)、テストの関数の数 (前 → 後)、推測で決めた点、見つけた別件。画面への返事は「書きました」の 1 行だけ
