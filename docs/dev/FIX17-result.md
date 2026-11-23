# FIX17 の結果

- fixture を作るときだけ、行の `Token` (内容のハッシュ、52 個) を `tok-<id>` (id の無い行は `tok-<順番>`) に置き換えた (`placeholdTokens`)。ほかにトークンらしい値は無かった。本物のトークンの作り方は変えていない。gallery は同じ値どうしで合わせるだけなので動く (`sh scripts/web-gallery.sh`: 135 枚、問題 0)
- `TestGalleryFixtures` は、16 字以上の16進や base64 らしい列 (24 字以上、英字と数字が混じる) が fixture にあれば落ちる (`secretLike`、`TestSecretLike` は見本を実行時に作る)
- `git add -A` → `gitleaks protect --staged --no-banner` → no leaks found → `git reset -q`。最初は私の足したテストの見本 (16進の偽トークン) が 1 件引っかかったので、見本を実行時に組み立てるように直した。コミットはしていない。index はいま HEAD と同じ
- vet / test (`-count=1`、e2e 込み) / build 通過
