# snav

`snav` is an interactive symbol finder for large repositories.

It streams candidates from ripgrep, ranks by symbol key first, and renders a two-line TUI row per hit:

- `path:line:col` (de-emphasized metadata)
- syntax-highlighted source line (Tree-sitter)

## Highlights

- Fast candidate ingestion with `rg --vimgrep --trim`
- Fuzzy filtering with primary score on extracted symbol key
- Incremental UI with lazy highlight scheduling (visible rows + buffer)
- Tree-sitter highlighting cache with worker pool
- Two highlight modes:
  - `synthetic` (default): tiny wrapped parse for speed
  - `file`: real file slice context for better accuracy
- Theme support via Chroma styles (`--theme`)
- Cross-platform open/copy behavior with overrideable editor command

## Project Layout

- `src/` application source and tests
- `go.mod`, `go.sum` module definition
- `mise.toml` task runner config

## Requirements

- Go 1.24+
- ripgrep (`rg`)
- C toolchain for CGO Tree-sitter grammars (for example `clang`/`gcc`)

Optional:

- `zed` for direct open with line/column
- clipboard tool:
  - macOS: `pbcopy`
  - Linux: `wl-copy`, `xclip`, or `xsel`
  - Windows: `clip`

## Quick Start

Run directly:

```bash
go run ./src -root .
```

Build binary:

```bash
go build -buildvcs=false -o snav ./src
./snav -root .
```

## Install Script

You can install from GitHub Releases with:

```bash
curl -fsSL https://raw.githubusercontent.com/m7b/snav/main/install.sh | bash
```

Optional environment overrides:

- `SNAV_REPO` override GitHub repo (`owner/name`)
- `SNAV_VERSION` install a specific tag (for example `v0.1.0`)
- `SNAV_INSTALL_DIR` install directory (default `/usr/local/bin`)

Examples:

```bash
curl -fsSL https://raw.githubusercontent.com/m7b/snav/main/install.sh | SNAV_VERSION=v0.1.0 bash
curl -fsSL https://raw.githubusercontent.com/m7b/snav/main/install.sh | SNAV_INSTALL_DIR="$HOME/.local/bin" bash
```

## Releasing

GoReleaser config lives in `.goreleaser.yaml` (with schema support from `https://goreleaser.com/static/schema.json`).

GitHub release workflow is in `.github/workflows/release.yml` and runs on pushed tags matching `v*`.

Release flow:

```bash
git tag v0.1.0
git push origin v0.1.0
```

## mise Tasks

If you use [mise](https://mise.jdx.dev/):

```bash
mise run dev
mise run test
mise run build
mise run bench
mise run cli
```

Available tasks:

- `fmt` format Go sources
- `test` run unit/integration tests
- `bench` run benchmarks
- `build` build local `snav` binary
- `dev` run via `go run` (interactive)
- `cli` build then run binary (interactive)
- `clean` remove built binary

## CLI Flags

- `-root` search root (default `.`)
- `-pattern` ripgrep regex for candidate extraction
- `-preview` toggle preview pane
- `-cache-size` highlight cache size
- `-workers` highlight worker count
- `-visible-buffer` extra rows to pre-highlight
- `-debounce-ms` filter debounce in milliseconds
- `-highlight-context` `synthetic|file`
- `-context-radius` file context radius for `file` mode
- `-editor-cmd` override open command (`{file}` `{line}` `{col}` `{target}`)
- `-no-ignore` disable ripgrep ignore files (`.gitignore`, `.ignore`, `.rgignore`)
- `-no-test` exclude common test/spec paths and generic `*test*`/`*spec*` files
- `-theme` chroma style name (for example `nord`, `dracula`, `monokai`, `github`)

Editor command examples:

- VS Code: `-editor-cmd "code -g {target}"`
- Helix: `-editor-cmd "hx {file}:{line}:{col}"`
- Vim: `-editor-cmd "vim +{line} {file}"`

## Keybindings

- `up/down`, `j/k` move selection
- `pgup/pgdn`, `ctrl+u/ctrl+d` jump page
- `tab` toggle preview
- `y` copy `path:line:col`
- `enter` open in editor/file viewer
- `esc`, `ctrl+c` quit

## Benchmarks

```bash
go test ./src -bench . -benchmem
```
