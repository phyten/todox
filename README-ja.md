# todox — Git リポジトリ向け TODO/FIXME 探索ツール（日本語版）

[![Lint](https://github.com/phyten/todox/actions/workflows/lint.yml/badge.svg)](https://github.com/phyten/todox/actions/workflows/lint.yml)
[![Test](https://github.com/phyten/todox/actions/workflows/test.yml/badge.svg)](https://github.com/phyten/todox/actions/workflows/test.yml)
[![Build](https://github.com/phyten/todox/actions/workflows/build.yml/badge.svg)](https://github.com/phyten/todox/actions/workflows/build.yml)

英語版 README は [README.md](./README.md) を参照してください。

`todox` はリポジトリ内の **大文字の `TODO` / `FIXME`** を検索し、
その行を**誰が追加・変更したのか**（最終 / 初回導入）を素早く洗い出せる CLI / Web ツールです。

- `--mode last`（既定）：その行を**最後に変更**した人（`git blame`）
- `--mode first`：その `TODO/FIXME` を**最初に導入**した人（`git log -L`）
- フィルタ：`--author`, `--type {todo|fixme|both}`
- 追加列：`--with-comment`（行本文を TODO/FIXME から表示）、`--with-message`（コミット件名 1 行目）、`--full`
- 文字数制御：`--truncate`, `--truncate-comment`, `--truncate-message`
- 出力：`table` / `tsv` / `json`
- 進捗表示：TTY のみ stderr に 1 行上書き（`--no-progress` あり）
- Web：`todox serve` で簡易 UI と JSON API

> 実装の詳細や AI と協働する運用は [`AGENTS.md`](./AGENTS.md) を参照してください。

---

## クイックスタート

### Homebrew（macOS / Linux）

```bash
brew tap phyten/todox
brew install todox
# または: brew install phyten/todox/todox
```

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

## Dev Container（推奨の開発環境）

Dev Containers CLI を使ってリポジトリを再現性高く立ち上げられます。

```bash
devcontainer up --workspace-folder .
devcontainer exec --workspace-folder . bash
make build
```

> Codespaces でも `.devcontainer/devcontainer.json` が読み込まれます。
> ローカル CLI ではポート 8080 を publish 済みです（`runArgs -p 8080:8080`）。

---

## CLI オプション（抜粋）

- `--type {todo|fixme|both}` : 対象タグ（既定: both）
- `--mode {last|first}` : 作者の定義（既定: last）
- `--author REGEX` : 作者名/メールの正規表現フィルタ
- `--with-comment` / `--with-message` / `--full`
- `--truncate N` / `--truncate-comment N` / `--truncate-message N`
- `--output {table|tsv|json}`
- `--no-progress` / `--progress`
- `--no-ignore-ws` : `git blame` に `-w` を付けない（空白変更も最新扱い）

ヘルプ：`./bin/todox -h`（英語/日本語切り替え対応）

---

## 注意・既知の制限

- `--mode first` は `git log -L` を多用するため、大規模リポジトリでは時間がかかります（進捗/ETA 表示あり）。
- `git` を必ずインストールしてください。コンテナ/Docker でもランタイムに `git` が必要です。
- 検索対象は **大文字の `TODO` / `FIXME`** のみです（小文字は対象外）。

---

## 開発（Make タスク）

- `make build` … `bin/todox` を生成
- `make serve` … Web モードで起動
- `make lint` … `golangci-lint run`（`golangci-lint` が PATH に必要）
- `make fmt` / `make vet` / `make clean`

---

## Lint

`golangci-lint` を使った静的解析を `make lint` で実行できます。

- 初回は `go install github.com/golangci/golangci-lint/cmd/golangci-lint@v2.4.0` でバイナリを取得してください。
- devcontainer の外から実行する場合は `./scripts/dcrun make lint` を推奨します。
- devcontainer 内のシェルであれば `make lint` をそのまま実行できます。
- GitHub Actions の `Lint` / `Test` / `Build` ワークフローで自動実行しています。

---

## Release

タグを `v*` 形式で push すると GitHub Actions の `Release` ワークフローが起動し、
Linux / macOS / Windows 向けのバイナリをクロスコンパイルしてリリースページに添付します。

Homebrew tap を自動更新したい場合は、事前に以下を準備してください。

- `phyten/homebrew-todox` のような tap リポジトリを作成（`Formula/todox.rb` をワークフローが自動生成します）
- tap リポジトリへ push 可能な PAT を発行し、`HOMEBREW_TAP_TOKEN` として Actions シークレットに登録

---

## ロードマップ（抜粋）

- `--with-age`（AGE 列）と `--sort`, `--group-by`
- リモート（GitHub/GitLab/Gitea）への行リンク生成
- Markdown / CSV 出力、fzf/TUI、`-M/-C` での行移動検出
- ファイル単位 blame の一括取得による高速化

---

## License

MIT
