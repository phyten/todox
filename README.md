# todox — TODO/FIXME explorer for Git repositories

[![Lint](https://github.com/phyten/todox/actions/workflows/lint.yml/badge.svg)](https://github.com/phyten/todox/actions/workflows/lint.yml)
[![Test](https://github.com/phyten/todox/actions/workflows/test.yml/badge.svg)](https://github.com/phyten/todox/actions/workflows/test.yml)
[![Build](https://github.com/phyten/todox/actions/workflows/build.yml/badge.svg)](https://github.com/phyten/todox/actions/workflows/build.yml)

`todox` scans your repository for **`TODO` / `FIXME`** markers (case-insensitive) and helps you identify **who introduced or last touched** those lines in seconds—either from the CLI or a lightweight web UI.

- `--mode last` (default): show the **most recent author** of the line (`git blame`).
- `--mode first`: show the **original author** who introduced the TODO/FIXME (`git log -L`).
- Filtering options: `--author`, `--type {todo|fixme|both}`.
- Extra columns: `--with-comment`, `--with-message`, `--with-age`, `--full` (shortcut for comment+message with truncation).
- Length control: `--truncate`, `--truncate-comment`, `--truncate-message`.
- Output formats: `table`, `tsv`, `json`.
- Color-aware tables: `--color {auto|always|never}` with automatic detection of `NO_COLOR`, `CLICOLOR`, and friends.
- Accessible label palette: TODO/FIXME colors adapt to light/dark terminal backgrounds for WCAG AA contrast.
- Progress bar: one-line TTY updates with smoothed ETA/P90 bands (disable with `--no-progress`).
- Web mode: `todox serve` exposes a minimal UI plus a JSON API and streaming progress via `/api/scan/stream`.

> For automation rules and AI collaboration guidelines, see [`AGENTS.md`](./AGENTS.md).
>
> 日本語ドキュメントは [README-ja.md](./README-ja.md) を参照してください。

---

## Quick start

### Homebrew (macOS / Linux)

```bash
brew tap phyten/todox
brew install todox
# or: brew install phyten/todox/todox
```

### Prerequisites

- `git` (the tool shells out to the Git CLI)
- Go 1.22 or newer (when building from source)

### Local build & run

```bash
go mod tidy
make build
./bin/todox -h
```

### Examples

```bash
# List TODO/FIXME items with the most recent author (table output)
./bin/todox

# Focus on FIXMEs, show the original author, truncate comment + subject to 80 characters
./bin/todox --type fixme --mode first --full --truncate 80

# Filter by author name or email (regular expression)
./bin/todox -a 'Alice|alice@example.com'

# Surface the stalest TODO/FIXME items first and display AGE in the output
./bin/todox --with-age --sort -age

# Export as TSV or JSON
./bin/todox --output tsv  > todo.tsv
./bin/todox --output json > todo.json
```

### Web mode

```bash
./bin/todox serve -p 8080
# -> http://localhost:8080 (JSON API: /api/scan)
```

The web form mirrors the server defaults: *ignore whitespace* starts checked (matching `ignore_ws=true`) and the *jobs* field is blank (auto). The `path`, `exclude`, and `path_regex` inputs are sent only when non-empty, and the *exclude typical dirs* checkbox emits `exclude_typical=1` when checked. Leaving everything untouched keeps the API behaviour unchanged.

Modern browsers open an `EventSource` to `/api/scan/stream`, showing live stage progress (`scan → attr → pr`), throughput, ETA, and a cancel link that simply closes the stream (which cancels the server-side scan). Browsers without SSE support automatically fall back to a single `fetch(/api/scan)` request, preserving the previous behaviour. The cancel link also aborts the fallback request via `AbortController`. When `with_pr_links=0`, the `pr` step is hidden from the UI. During transient network issues the UI shows a small "reconnecting…" message while the browser auto-reconnects (3s).

Click any table header to sort results locally (ascending/descending toggle; empty values always sink to the bottom). The PR column now renders `#<number> <title> (state)` and hovering the link shows the first 280 characters of the PR description. Tooltips collapse whitespace and honour the existing escaping so the raw Markdown remains unchanged in the JSON payload.

The server currently emits `progress`, `result`, `ping`, and both `error` and `server_error` events (the latter is the preferred payload; `error` is kept for backward compatibility). Clients should listen to `server_error` first and keep `error` handling as a fallback until older builds are updated.

---

The embedded CSS honours `prefers-color-scheme` so light/dark mode follows the OS, and TODO/FIXME badges use WCAG AA-compliant foreground/background pairs.

---

## Configuration

`todox` reads optional `.todox.yaml` / `.todox.toml` / `.todox.json` files and applies their values before command-line flags. The search order is:

1. `TODOX_CONFIG=/abs/path/to/file` (if present)
2. The repository root (`--repo` or `.`) and its parents
3. `$XDG_CONFIG_HOME/todox/config.{yaml,toml,json}`
4. `$HOME/.todox.{yaml,toml,json}`

The first file found wins; there is no multi-level merge. Final values are resolved with the following precedence: **CLI flags > `TODOX_*` environment variables > config file > built-in defaults**. Both the CLI and the web server share the same resolution logic.

> If a key appears both at the **top level** and under `engine:`/`ui:`, the top-level value takes precedence (mixing scopes is discouraged).

Example `.todox.yaml`:

```yaml
type: fixme
paths:
  - internal
  - cmd
with_comment: true
with_pr_links: true
ui:
  pr_state: open
  pr_limit: 5
  sort: -age
```

Scalar and list fields accept either a single string (`path: src`) or an array (`path: ["src", "cmd"]`). Booleans use the same syntax as CLI flags (`true`, `false`, `1`, `0`, `yes`, `no`, ...).

Frequently used environment variables (all prefixed with `TODOX_`):

| Setting | Environment variable | Example |
| --- | --- | --- |
| `type` | `TODOX_TYPE` | `fixme` |
| `mode` | `TODOX_MODE` | `first` |
| `author` | `TODOX_AUTHOR` | `alice@example.com` |
| `paths` | `TODOX_PATH` | `src,cmd` |
| `path_regex` | `TODOX_PATH_REGEX` | `.*\.go$` |
| `excludes` | `TODOX_EXCLUDE` | `vendor/**,dist/**` |
| `exclude_typical` | `TODOX_EXCLUDE_TYPICAL` | `true` |
| `with_comment` | `TODOX_WITH_COMMENT` | `true` |
| `with_message` | `TODOX_WITH_MESSAGE` | `1` |
| `ignore_ws` | `TODOX_IGNORE_WS` | `false` |
| `with_age` | `TODOX_WITH_AGE` | `yes` |
| `with_commit_link` | `TODOX_WITH_COMMIT_LINK` | `true` |
| `with_pr_links` | `TODOX_WITH_PR_LINKS` | `true` |
| `pr_state` | `TODOX_PR_STATE` | `merged` |
| `pr_prefer` | `TODOX_PR_PREFER` | `open` |
| `pr_limit` | `TODOX_PR_LIMIT` | `5` |
| `fields` | `TODOX_FIELDS` | `type,author,date` |
| `sort` | `TODOX_SORT` | `-age,file` |
| `truncate` | `TODOX_TRUNCATE` | `120` |
| `truncate_comment` | `TODOX_TRUNCATE_COMMENT` | `80` |
| `truncate_message` | `TODOX_TRUNCATE_MESSAGE` | `72` |
| `output` | `TODOX_OUTPUT` | `json` |
| `color` | `TODOX_COLOR` | `never` |
| `jobs` | `TODOX_JOBS` | `8` |
| `repo` | `TODOX_REPO` | `/path/to/repo` |

Unset variables simply fall back to the config file (or built-in) defaults. Invalid values are rejected with the same error messages as their CLI counterparts.

---

## Dev Container (recommended development setup)

The provided Dev Container gives you a reproducible environment.

```bash
devcontainer up --workspace-folder .
devcontainer exec --workspace-folder . bash
make build
```

> GitHub Codespaces automatically reads `.devcontainer/devcontainer.json`.
> The local configuration publishes port 8080 (`runArgs -p 8080:8080`).

---

## CLI options (highlights)

### Search & attribution

- `-t, --type {todo|fixme|both}`: which markers to scan (default: both)
- `-m, --mode {last|first}`: author definition (default: last)
- `-a, --author REGEX`: filter by author name or email (extended regex)
- `--detect {auto|parse|regex}`: choose between the parser-based engine, legacy regex scanning, or automatic fallback logic
- `--detect-langs go,js,py,...`: restrict parser-based detection to the provided languages (CSV or repeated flags). When combined
  with `--detect=parse`, files whose detected language is not in the list are skipped (no regex fallback). With
  `--detect=auto`, excluded files fall back to the heuristic/plain-text scanner. Common shorthands such as `js`, `ts`, `py`,
  `rb`, `sh`, `ps1`, and `c++` are normalized automatically.
- `--tags TODO,FIXME,NOTE`: override the detection tag set (case-insensitive; CSV or repeated flags)
- `--include-strings` / `--comments-only` / `--no-strings`: control whether string literals are scanned.
  When multiple CLI flags are present, the **last one wins**. In config files and environment variables,
  `comments_only` / `no_strings` override `include_strings` if both are provided. Web API query parameters
  follow the same override rule as config/env (a later `comments_only` or `no_strings` value wins over
  earlier `include_strings`).
- `--max-file-bytes N`: fall back to the heuristic/plain-text scanner for files larger than `N` bytes (0 = unlimited). This
  fallback applies even when `--detect=parse` is selected.
- `--no-prefilter`: disable the default `git grep` prefilter before parser-based scanning

### Path filtering

- `--path LIST`: limit the `git grep` scope to the provided pathspec(s). Accepts comma-separated values and repeated flags.
- `--exclude LIST`: exclude pathspecs/globs (comma-separated and repeatable). `:(exclude)` / `:!` prefixes are respected.
- `--path-regex REGEXP`: keep only matches whose file path satisfies any of the given Go regular expressions.
- `--exclude-typical`: enable the built-in exclude set (`vendor/**`, `node_modules/**`, `dist/**`, `build/**`, `target/**`, `*.min.*`).

### Output selection

- `-o, --output {table|tsv|json}`: choose the output format (default: table)
- `--fields type,author,date,...`: choose the columns for table/TSV (comma separated; overrides `--with-*`)
- `--color {auto|always|never}`: control terminal coloring for the table output (default: auto)

### Color mode & environment variables

- `--color auto` respects terminal capabilities:
  - `NO_COLOR` disables colors (and wins over any force flags).
  - `CLICOLOR=0` disables colors when auto-detected (also winning over force flags).
  - `CLICOLOR_FORCE`/`FORCE_COLOR` force-enable colors when set to anything other than `"0"`.
  - `TERM=dumb` disables colors regardless of other environment variables.
  - Otherwise the CLI checks whether `stdout` is a TTY (stderr is ignored).
- Color profiles are inferred automatically:
  - `COLORTERM=truecolor|24bit` → True Color gradients for the AGE column.
  - `TERM=*256color` → ANSI 256-color gradients.
  - Other terminals fall back to the basic 8-color palette (TODO = yellow, FIXME = red; overall contrast follows your terminal's palette definitions).
- The AGE gradient scales itself to your repository. The 95th percentile of ages (with a minimum of 120 days)
  is treated as "fully red" so that outliers do not drown out day-to-day differences.
- `COLORFGBG` (when exported by your terminal) guides TODO/FIXME hues so bright/light backgrounds keep ≥4.5:1 contrast; unknown terminals fall back to a safe dark profile.
- When `COLORFGBG` is absent but `TERM` includes `"light"`, todox assumes a light background before defaulting to the dark palette.
- In True Color mode the TODO/FIXME palette is validated against WCAG AA contrast thresholds; ANSI 256 and 8-color modes keep the same hue intent but final contrast depends on your terminal's palette definitions.
- When piping colored output, use a pager that preserves ANSI escapes, for example:

```
todox --with-age --color always | less -R
```
- Example output with gradients for AGE and colored TODO/FIXME markers:

![Colorized table output](docs/color-table.png)

> JSON output always includes an `age_days` field for each item.
> When commit link generation is enabled, each item exposes a `url` and the top-level result includes `has_url`. Enabling PR links populates `prs[]` alongside `has_prs`; every entry exposes `{number,state,url,title,body}` (the new fields are additive so older clients remain compatible). These fields are part of the public API surface and will remain stable.
> Note that `has_url` / `has_prs` indicate that the data was collected (via `--with-*` or explicit `--fields` selections); a column may still be hidden depending on the renderer.

`--fields` only affects rendering. Data acquisition still follows the `--with-*` flags as well as any explicit field names. For example, `--fields type,url` keeps `NeedURL=true` even without `--with-commit-link`, so the URL column renders correctly across table/TSV/JSON outputs. When `--fields` is specified it replaces the default column list even if `--with-comment` / `--with-message` were set earlier—include `comment`/`message` explicitly in `--fields` when you still want them visible.

For example:

```bash
# Ensure the comment column is still rendered after overriding --fields
todox --with-comment --fields type,author,comment
```

Fields you can reference via `--fields` (table/TSV outputs):
- `type`, `tag`, `kind`, `lang`
- `author`, `email`, `date`, `age`, `commit`, `location` (`file:line`)
- `text`, `span`
- `comment`, `message`
- `url` (alias: `commit_url`; renders as `COMMIT_URL` to avoid a header clash)
- `pr`, `prs`, `pr_urls`

`type` reports the normalized tag (e.g. `TODO`, `FIXME`), while `tag` returns the canonical tag that was matched. Today both
values are uppercased and therefore usually identical; future releases may surface the source text in `tag`. `kind` identifies
the match source (`comment`, `string`, `heredoc`, …) and `lang` indicates the parser language. `span` prints
`start_line:start_col-end_line:end_col` using 1-based coordinates; column offsets count bytes in the UTF-8 source.

### Extra columns (hidden by default)

- `--with-comment`: include the TODO/FIXME line text starting at the first matched tag
- `--with-snippet`: alias of `--with-comment` (kept for backward compatibility)
- `--with-message`: include the commit subject (first line)
- `--with-age`: append an AGE (days since author date) column to table/TSV outputs
- `--with-commit-link`: include a URL column with GitHub blob links (uses the `origin` remote by default)
  - `--with-link` remains available as a deprecated alias for backward compatibility.
  - Suppress the deprecated-alias warning by setting `TODOX_NO_DEPRECATION_WARNINGS=1` (handy for CI pipelines).
  - Override the remote name with `TODOX_LINK_REMOTE=<name>` when `origin` is not available (for example `upstream`).
  - Override the scheme with `TODOX_LINK_SCHEME=http` when your GitHub Enterprise appliance is served over plain HTTP.
  - Remote resolution failures do not abort the scan; URLs are left blank and a warning is recorded in `errors[]` / `error_count`.
  - Markdown files append `?plain=1#L<n>` to avoid GitHub anchor collisions with the rendered view.
- `--with-pr-links`: attach pull requests that contain each commit.
  - Combine with `--pr-state {all|open|closed|merged}` to filter by state, `--pr-limit N` (1–20, default 3) to cap the number of PRs per item, and `--pr-prefer {open|merged|closed|none}` to influence ordering when multiple states are present.
  - Results populate `prs[]` per item and set `has_prs=true` in JSON/table metadata. Each entry exposes `{number,state,url,title,body}` (empty strings are omitted from JSON via `omitempty`).
  - Authenticate with the GitHub CLI (`gh`) or export `GH_TOKEN` / `GITHUB_TOKEN` for REST access when scanning private repositories; anonymous requests can hit rate limits quickly.
  - Tune the PR fetching worker pool with `TODOX_GH_JOBS=<n>` (1–32). The default uses the smaller of `jobs` and 32.
- `--full`: shorthand for `--with-comment --with-message`

### Truncation controls

- `--truncate N`: truncate both COMMENT and MESSAGE to `N` columns of visible width (0 = unlimited)
- `--truncate-comment N`: truncate only COMMENT (visible width)
- `--truncate-message N`: truncate only MESSAGE (visible width)

Display widths follow Unicode wcwidth rules: grapheme clusters (emoji, combining marks) stay intact. Set `TODOX_EASTASIAN=1` (or `true`) to treat ambiguous-width characters as double-width for East Asian terminals.

### Sorting

- `--sort key[,key...]`: multi-level sort. Prefix with `-` for descending, `+` (or nothing) for ascending.
  Supported keys: `age`, `date`, `author`, `email`, `type`, `file`, `line`, `commit`, `location` (`file,line`).

### Progress / blame behaviour

- `--no-progress` / `--progress`: disable or force the progress display
- `--no-ignore-ws`: run `git blame` without `-w` so whitespace-only edits are considered latest
- Web API: pass `ignore_ws=0` to honour whitespace edits and `jobs=<n>` (1–64) to cap worker concurrency

### Help & language

- `-h, --help [en|ja]`: show help (English by default, pass `ja` for Japanese)
- `--help=ja`, `--help-ja`: convenient aliases to show Japanese help immediately
- `--lang {en|ja}`: set the help language for the current invocation
- `GTA_LANG=ja` (environment): default to Japanese help (`GIT_TODO_AUTHORS_LANG` also works)

Full help: `./bin/todox -h` (bilingual output and examples).

### GitHub helpers

- `todox pr find --commit <sha>`: list pull requests containing the commit
- `todox pr open --commit <sha>`: open the first matching pull request in your browser
- `todox pr create --commit <sha>`: create a pull request via the GitHub CLI (`gh`). Supports `--source` and `--base` overrides. Lookup helpers fall back to REST when `GH_TOKEN`/`GITHUB_TOKEN` is present, but creation itself still requires the `gh` binary.

### Input normalization & validation (CLI / Web)

Both the CLI flags and the `/api/scan` query parameters share the same normalization layer. All inputs are case-insensitive unless noted.

| Parameter | Accepted values | Validation |
| --- | --- | --- |
| Boolean flags (`--with-comment`, `with_comment`, `--with-message`, `with_message`, `--with-commit-link`, `with_commit_link`, `--with-pr-links`, `with_pr_links`, `ignore_ws`, etc.; `--with-link` / `with_link` remain as deprecated aliases) | `1`, `true`, `yes`, `on` → `true`; `0`, `false`, `no`, `off` → `false` | Empty values mean "not specified". Any other literal returns an error. |
| `--type`, `type` | `todo`, `fixme`, `both` | Unknown values are rejected. |
| `--mode`, `mode` | `last`, `first` | Unknown values are rejected. |
| `--output` | `table`, `tsv`, `json` | Unknown values are rejected (CLI only). |
| `--jobs`, `jobs` | Integers in `[1, 64]` | Values outside the range are rejected. |
| `--path`, `path` | Pathspecs/globs, comma-separated or repeated | Values are trimmed. Empty entries are ignored. |
| `--exclude`, `exclude` | Same as above | `:(exclude)` / `:!` prefixes are preserved; otherwise `:(glob,exclude)` is added internally. |
| `--path-regex`, `path_regex` | Go regular expressions | Each entry must compile. Invalid patterns return an error. |
| `--exclude-typical`, `exclude_typical` | Boolean (same literals as other flags) | Enables the built-in set: `vendor/**`, `node_modules/**`, `dist/**`, `build/**`, `target/**`, `*.min.*`. |
| `--truncate`, `--truncate-comment`, `--truncate-message` (and the API equivalents) | Integers ≥ 0 | Negative values are rejected. When both COMMENT and MESSAGE columns are enabled and no truncate is supplied, a default of 120 display columns is applied. |

Default for `jobs`: `min(runtime.NumCPU(), 64)` (number of CPU cores capped at 64).

---

## Caveats & known limitations

- `--mode first` relies heavily on `git log -L`, which can be slow on very large repositories. A progress bar and ETA are displayed.
- `git` must be available at runtime—even inside containers.
- `TODO` / `FIXME` detection is case-insensitive. Use `--tags` if you need to narrow the accepted marker set.
- TSV/table headers render `commit_url` as `COMMIT_URL` to avoid a clash with the existing `URL` column in earlier releases.
- `span` coordinates count bytes within each line. Grapheme-aware columns may surface in a future release.

---

## Development (Make targets)

- `make build`: produce `bin/todox`
- `make serve`: launch the web UI
- `make lint`: run `golangci-lint` (binary must be on `PATH`)
- `make fmt` / `make vet` / `make clean`

---

## Linting

Static analysis is powered by `golangci-lint` via `make lint`.

- Install the binary once with `go install github.com/golangci/golangci-lint/cmd/golangci-lint@v2.4.0`
- Outside the dev container, prefer `./scripts/dcrun make lint`
- Inside the container, `make lint` works directly
- CI runs the `Lint`, `Test`, and `Build` workflows automatically

---

## Release process

Pushing a `v*` tag triggers the `Release` workflow, which cross-compiles binaries for Linux, macOS, and Windows and attaches them to the GitHub release.

To update a Homebrew tap automatically, prepare:

- A tap repository such as `phyten/homebrew-todox` (the workflow generates `Formula/todox.rb`)
- A personal access token with push permission stored as the `HOMEBREW_TAP_TOKEN` secret

---

## Roadmap (highlights)

- Additional sorting/grouping options building on the new `--with-age` column
- Deep links to remote hosts (GitHub / GitLab / Gitea)
- Additional outputs (Markdown, CSV), fzf/TUI integration, detection of moved lines via `-M/-C`
- Faster scans by batching file-level blame queries

---

## License

MIT
