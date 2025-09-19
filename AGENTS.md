# `AGENTS.md`

AGENTS.md — devcontainer 前提で AI エージェント（Claude / codex など）と安全に協働するための手引き

## 目的

このリポジトリは **Dev Container（devcontainer）** を前提とします。
エージェントは **devcontainer 内での実行を想定**し、**パッチ（diff）とコマンド列**を返してください。人間はそれを devcontainer 内で適用・実行・確認します。

> 重要: この文書では **三連バッククォート（\`\`\`）を使いません**。
> 複数行の内容は **マーカー方式**（`PATCH-BEGIN` / `PATCH-END`、`CMD>` など）で指示してください。

---

## TL;DR（人間向け）

* コンテナ起動（CLI）

  * `devcontainer up --workspace-folder .`

* devcontainer 内で実行

  * ホスト側からコマンドを流す場合は `./scripts/dcrun ...` で devcontainer に転送
  * すでに devcontainer のシェルにいる場合は `make build` や `./bin/todox -h` を直接実行して OK
  * 例: `./scripts/dcrun make build`（ホスト） / `make build`（devcontainer 内）

* エージェントのパッチを適用

  1. エージェント出力の **PATCH-BEGIN〜PATCH-END** を丸ごとコピーして `patch.diff` に保存
  2. `cat patch.diff | ./scripts/agent-apply`

---

## 使うスクリプト（レポジトリに同梱）

* `scripts/dcrun` … 任意コマンドを **常に devcontainer 内で**実行するラッパー
  例: `./scripts/dcrun go version`, `./scripts/dcrun make build`

  * devcontainer にアタッチ済みなら同じコマンドをそのまま実行可能（例: `go version`）

* `scripts/agent-apply` … **Unified diff** を標準入力で受けて `git apply --3way --index` を実行
  例: `cat patch.diff | ./scripts/agent-apply`

---

## エージェントへの依頼フォーマット

エージェントは、以下 **4 つのセクション**の順で出力してください。
（**フェンス禁止**。マーカーとプレフィックスを使います）

1. **PLAN** … 変更方針（箇条書き）
2. **PATCH** … `PATCH-BEGIN` 〜 `PATCH-END` の間に **Unified diff**
3. **COMMANDS** … 各行を `CMD>` ではじめる **devcontainer 内コマンド列**
4. **CHECKS** … 動作確認手順（コマンドまたは観察ポイント）

### 具体例（フェンス無し）

PLAN:

* `--with-age` オプションを追加し、AGE（日数）列を表示する
* `git show -s --format=%ct` の時刻と現在時刻から日数を計算
* TSV/JSON/表の全出力に AGE を反映

PATCH-BEGIN
diff --git a/internal/engine/types.go b/internal/engine/types.go
\--- a/internal/engine/types.go
+++ b/internal/engine/types.go
@@
type Item struct {
Kind    string `json:"kind"`
Author  string `json:"author"`
Email   string `json:"email"`
Date    string `json:"date"`

* AgeDays int    `json:"age_days"`
  Commit  string `json:"commit"`
  File    string `json:"file"`
  Line    int    `json:"line"`
  }
  PATCH-END

COMMANDS:
CMD> ./scripts/dcrun make build
CMD> ./scripts/dcrun ./bin/todox --output tsv | head -n 5

CHECKS:

* 出力に `age_days`（TSV/JSON）または `AGE`（表）が含まれていること
* 0〜数千の妥当な値が入っていること
* 既存フラグに回帰がないこと（`-h`、`--full` 等）

---

## ルール（ガードレール）

* **フェンス（\`\`\` や \~\~\~）禁止**。必ず **マーカー方式**を使うこと
* コマンドは **devcontainer 内実行**を前提にし、`CMD>` を付けること

  * ホスト側から操作する場合は `./scripts/dcrun ...` 形式を案内する
  * devcontainer 内での再現はそのまま（例: `make build`）で問題ないと明記しても良い
* パッチは **最小差分**に留めること（大規模リフォーマットや不要な変更は避ける）
* 秘密情報や外部ネットワーク設定の追加は行わないこと
* 変更後は `make build`、必要なら `make fmt` / `make vet` をコマンドに含めること

---

## 人間側の手順（詳細）

1. エージェントの出力から、`PATCH-BEGIN`〜`PATCH-END` を含めてコピーし、`patch.diff` に保存
2. `cat patch.diff | ./scripts/agent-apply` を実行
3. `./scripts/dcrun make build` でビルド
4. `COMMANDS` に列挙されたコマンドを順番に実行
5. `CHECKS` の観点で結果を確認（必要に応じて差戻し）

---

## トラブルシュート

* `command not found`
  → コマンドの前に `./scripts/dcrun` が付いているか確認

* `git apply` が失敗する
  → `patch.diff` が壊れていないか（`PATCH-BEGIN` と `PATCH-END` を含める／改行末尾）
  → 競合した場合は `git status` で確認し、手で解消してから `git add` → 再ビルド

* Web が見えない
  → `todox serve -p 8080` の後、ローカル devcontainer なら 8080 を publish、Codespaces ならポート転送が有効か確認

---

## 参考（レポジトリの基本）

* 言語: Go 1.22
* エントリポイント:

  * CLI: `./cmd/todox`
  * Web: `todox serve -p 8080`
* 主な Make タスク:

  * `make build` → `bin/todox` を生成
  * `make serve` → ビルド後に `todox serve -p 8080`
  * `make fmt` / `make vet` / `make clean`
* devcontainer:

  * 画像: `mcr.microsoft.com/devcontainers/base:ubuntu-24.04`
  * Features: Go / Git
  * ポート: 8080（Codespaces では forward、ローカル devcontainer では publish）

---

### 備考：どうしてもフェンスを使いたい場面が出たら

ビューアの自動 UI を抑止したいだけなら、フェンスの代わりに **HTML タグ**（`<pre><code>...</code></pre>`）を使う方法もあります。
ただし環境によっては結局コピー UI が出ることがあるため、本書では **マーカー方式**を標準としています。
