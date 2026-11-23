# FEAT7 の結果

vet / test / build は通った。pty 越しに確かめた (スクラッチパッド、幅 80): 別のシェルで待たせた `pitboard -s s1 wait` が、viewer で `y` → `s` を押すと `y plan (…)` を出して終了コード 0 で終わり、右上の `AI 待機中` が出て、終わると消えた。実物の `herdr agent prompt` / `herdr pane send-*` は実行していない。

## 項目ごと

1. `pitboard -s SESSION wait [--timeout DURATION]`: 届けていない `send` があれば即座に `changes` と同じ出力。無ければ 0.3 秒ごとに events.jsonl の大きさ・更新時刻を見て待つ。timeout と SIGINT / SIGTERM は何も出さず 0。`.pitboard/<session>/waiting` に pid、2 つめは `already waiting (pid N)`、死んだ pid は無いもの扱い
2. viewer: 待っている `wait` がいれば `send` を追記するだけで「送りました」。いなければ `PITBOARD_SEND` → herdr。どれも無ければ「送る印は付けました。AI が待っていないので、チャットで一言伝えてください。」+ 直し方。右上に薄く `AI 待機中`
3. `-h` (コマンド一覧、作法、Send を wait → hook → 打ち込みの順に)、仕様書 (送る・コマンド・新しい「wait」節・作法)、README、skill を合わせた

## 推測で決めた点

- 「届けていない `send`」は「いちばん新しい `send` の番号 > `seen`」とした。hook が先に届けたら、`wait` はそのまま待ち続ける
- 2 つめの `wait` は、届けていない `send` があっても先に「already waiting」で終わる (古いほうが 0.3 秒以内に届けるので)
- `waiting` は `O_EXCL` で作る。中身が空 (書き込み途中) なら 50ms 待って見直す。消すのは中身が自分の pid のときだけ
- `wait` が出すものが空 (送られたのが届けない操作だけ) でも、`send` があれば終わる
- 「どれも無い」ときの 2 行目は「AI に pitboard wait を動かしてもらうか、PITBOARD_SEND を設定してください (pitboard -h の "Send")」。herdr の agent がいない・実行に失敗したときは、今までどおり「AI を起こせなかったので…」
- `AI 待機中` は右上の並びで `送信済み` の左 (`反応待ち` `新着` の右)。tick (0.5 秒) ごとに見直す
- `--timeout=30m` の形も受ける。負の値と読めない値はエラー
- skill の description は変えていない (本文の「In short」だけ wait を足した)
