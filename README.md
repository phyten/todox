# todox — TODO/FIXME explorer for Git repositories

[![Lint](https://github.com/phyten/todox/actions/workflows/lint.yml/badge.svg)](https://github.com/phyten/todox/actions/workflows/lint.yml)
[![Test](https://github.com/phyten/todox/actions/workflows/test.yml/badge.svg)](https://github.com/phyten/todox/actions/workflows/test.yml)
[![Build](https://github.com/phyten/todox/actions/workflows/build.yml/badge.svg)](https://github.com/phyten/todox/actions/workflows/build.yml)

`todox` scans your repository for uppercase **`TODO` / `FIXME`** markers and helps you
identify **who introduced or last touched** those lines in seconds—either from the CLI or a lightweight web UI.

- `--mode last` (default): show the **most recent author** of the line (`git blame`).
- `--mode first`: show the **original author** who introduced the TODO/FIXME (`git log -L`).
- Filtering options: `--author`, `--type {todo|fixme|both}`.
- Extra columns: `--with-comment`, `--with-message`, `--with-age`, `--full` (shortcut for comment+message with truncation).
- Length control: `--truncate`, `--truncate-comment`, `--truncate-message`.
- Output formats: `table`, `tsv`, `json`.
- Progress bar: one-line TTY updates (disable with `--no-progress`).
- Web mode: `todox serve` exposes a minimal UI plus a JSON API.

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

### Output selection

- `-o, --output {table|tsv|json}`: choose the output format (default: table)
- `--fields type,author,date,...`: choose the columns for table/TSV (comma separated; overrides `--with-*`)

> JSON output always includes an `age_days` field for each item.

### Extra columns (hidden by default)

- `--with-comment`: include the TODO/FIXME line text
- `--with-snippet`: alias of `--with-comment` (kept for backward compatibility)
- `--with-message`: include the commit subject (first line)
- `--with-age`: append an AGE (days since author date) column to table/TSV outputs
- `--full`: shorthand for `--with-comment --with-message`

### Truncation controls

- `--truncate N`: truncate both COMMENT and MESSAGE to `N` characters (0 = unlimited)
- `--truncate-comment N`: truncate only COMMENT
- `--truncate-message N`: truncate only MESSAGE

### Sorting

- `--sort key[,key...]`: multi-level sort. Prefix with `-` for descending, `+` (or nothing) for ascending.
  Supported keys: `age`, `date`, `author`, `email`, `type`, `file`, `line`, `commit`, `location` (`file,line`).

### Progress / blame behaviour

- `--no-progress` / `--progress`: disable or force the progress display
- `--no-ignore-ws`: run `git blame` without `-w` so whitespace-only edits are considered latest

### Help & language

- `-h, --help [en|ja]`: show help (English by default, pass `ja` for Japanese)
- `--help=ja`, `--help-ja`: convenient aliases to show Japanese help immediately
- `--lang {en|ja}`: set the help language for the current invocation
- `GTA_LANG=ja` (environment): default to Japanese help (`GIT_TODO_AUTHORS_LANG` also works)

Full help: `./bin/todox -h` (bilingual output and examples).

### Input normalization & validation (CLI / Web)

Both the CLI flags and the `/api/scan` query parameters share the same normalization layer. All inputs are case-insensitive unless noted.

| Parameter | Accepted values | Validation |
| --- | --- | --- |
| Boolean flags (`--with-comment`, `with_comment`, `--with-message`, `with_message`, `ignore_ws`, etc.) | `1`, `true`, `yes`, `on` → `true`; `0`, `false`, `no`, `off` → `false` | Empty values mean "not specified". Any other literal returns an error. |
| `--type`, `type` | `todo`, `fixme`, `both` | Unknown values are rejected. |
| `--mode`, `mode` | `last`, `first` | Unknown values are rejected. |
| `--output` | `table`, `tsv`, `json` | Unknown values are rejected (CLI only). |
| `--jobs`, `jobs` | Integers in `[1, 64]` | Values outside the range are rejected. |
| `--truncate`, `--truncate-comment`, `--truncate-message` (and the API equivalents) | Integers ≥ 0 | Negative values are rejected. When both COMMENT and MESSAGE columns are enabled and no truncate is supplied, a default of 120 runes is applied. |

---

## Caveats & known limitations

- `--mode first` relies heavily on `git log -L`, which can be slow on very large repositories. A progress bar and ETA are displayed.
- `git` must be available at runtime—even inside containers.
- Only uppercase `TODO` / `FIXME` markers are detected.

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
