# WEB5 の結果

vet / test (`-count=1`、e2e 込み) / build 通過。git には触っていない。
- `web.Listen` (`internal/web/server.go`): 既定 (`PortDefault`) は 7482 (「PITB」)、ふさがっていれば 7491 まで次を試し、無ければシステムの空き。`--port N` はその番号だけで、ふさがっていれば「port N: … (leave out --port to use a free one)」。`serve` は既定のとき子に `--port` を渡さない
- cookie はもとからポートごと (`pitboard_PORT`)。同じポートの古い cookie は 403、出た URL (`?t=`) を開くと Set-Cookie で置き換わり通る (`TestStaleCookieOnSamePort`)
- テスト: `TestListenPorts` (7482 が空いていれば 7482・先頭がふさがれば次・範囲が全部ふさがれば別の空き・exact の誤り)、`TestServePort` (既定の解釈・`--port` がふさがっているとエラー)
- `-h`・README・SPEC に既定の番号と理由を 1 行。実機: スクラッチの 2 セッションで 7482・7483 になり、両方 `--stop` で止めた
