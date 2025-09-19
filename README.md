# todox — TODO/FIXME explorer for Git repositories

`todox` は、リポジトリ内の **大文字 `TODO` / `FIXME`** を検索し、  
**誰がその行を書いたか**（最終 or 初回導入）を素早く特定できる CLI / Web ツールです。

- `--mode last`（既定）：その行を**最後に変更**した人（`git blame`）
- `--mode first`：その `TODO/FIXME` を**最初に導入**した人（`git log -L`）
- フィルタ：`--author`, `--type {todo|fixme|both}`
- 追加列：`--with-comment`（行本文を TODO/FIXME から表示）、`--with-message`（コミット件名1行目）、`--full`
- 文字数制御：`--truncate`, `--truncate-comment`, `--truncate-message`
- 出力：`table` / `tsv` / `json`
- 進捗表示：TTY 時のみ stderr で 1 行上書き（`--no-progress` あり）
- Web：`todox serve` で簡易 UI と JSON API

> 実装の詳細や AI と協働する運用は [`AGENTS.md`](./AGENTS.md) を参照。

---

## Quick start

### 依存
- `git`（内部で CLI を呼び出します）
- Go 1.22 以降（ソースからビルドする場合）

### ビルド & 実行（ローカル）
```bash
go mod tidy
make build
./bin/todox -h
```

### 例

```bash
# すべての TODO/FIXME の最終変更者（表形式）
./bin/todox

# FIXME だけ、初回導入者、行本文＋件名を 80 文字にトリム
./bin/todox --type fixme --mode first --full --truncate 80

# 作者名/メールで絞り込み（正規表現）
./bin/todox -a 'Mikiyasu|phyten'

# TSV / JSON で出力
./bin/todox --output tsv  > todo.tsv
./bin/todox --output json > todo.json
```

### Web モード

```bash
./bin/todox serve -p 8080
# -> http://localhost:8080 （API: /api/scan）
```

---

## Devcontainer（推奨の開発環境）

Dev Containers CLI で reproducible に開発できます。

```bash
devcontainer up --workspace-folder .
devcontainer exec --workspace-folder . bash
make build
```

> Codespaces でも `.devcontainer/devcontainer.json` が読み込まれます。
> ローカル CLI では 8080 を publish 済み（`runArgs -p 8080:8080`）。

---

## CLI オプション（抜粋）

* `--type {todo|fixme|both}` : 対象タグ（既定: both）
* `--mode {last|first}` : 作者の定義（既定: last）
* `--author REGEX` : 作者名/メールの正規表現フィルタ
* `--with-comment` / `--with-message` / `--full`
* `--truncate N` / `--truncate-comment N` / `--truncate-message N`
* `--output {table|tsv|json}`
* `--no-progress` / `--progress`
* `--no-ignore-ws` : `git blame` に `-w` を付けない（空白変更も最新扱い）

ヘルプ：`./bin/todox -h`（英語/日本語切替付き）

---

## 注意・既知の制限

* `--mode first` は `git log -L` を多用するため、大規模リポジトリでは時間がかかります（進捗/ETA 表示あり）。
* `git` を必ずインストールしてください。コンテナ/Docker でもランタイムに `git` が必要です。
* 検索対象は **大文字の `TODO` / `FIXME`** です（小文字は対象外）。

---

## 開発（Make タスク）

* `make build` … `bin/todox` を生成
* `make serve` … Web モードで起動
* `make lint` … `golangci-lint run`（`golangci-lint` が PATH に必要）
* `make fmt` / `make vet` / `make clean`

---

## ロードマップ（抜粋）

* `--with-age`（AGE 列）と `--sort`, `--group-by`
* リモート（GitHub/GitLab/Gitea）への行リンク生成
* Markdown / CSV 出力、fzf/TUI、`-M/-C` での行移動検出
* ファイル単位 blame の一括取得で高速化

---

## License

MIT
