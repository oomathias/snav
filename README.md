# symfind

`symfind` is a cross-platform interactive symbol finder for large repos.

It streams candidates from `rg`, renders each hit as a two-line row, and applies a Tree-sitter-based syntax pass to the code line using a Nord palette.

Candidate discovery follows ripgrep ignore files (`.gitignore`, `.ignore`, `.rgignore`) by default.

## Features

- Two-line list rows:
  - `path:line:col` in dim gray
  - syntax-highlighted code line
- Selection background that keeps token colors intact
- Query emphasis (bold + underline) over token colors
- Fast filtering with primary fuzzy match on extracted symbol key
- Lazy highlighting: only visible rows plus a small buffer
- LRU highlight cache:
  - `synthetic`: key `(LangID, Text)`
  - `file`: key `(LangID, File, Line, Text)`
- Optional preview pane using the same highlighter pipeline
- Two highlighting strategies:
  - `synthetic` (Strategy A): tiny wrapped parse, very fast
  - `file` (Strategy B): file-slice context parse for higher fidelity

## Requirements

- macOS, Linux, or Windows
- Go toolchain
- `rg` (ripgrep)
- C toolchain for CGO/Tree-sitter grammars (for example `clang`/`gcc`)
- Optional: `zed` CLI for direct open-on-selection with line/column
- Clipboard tooling:
  - macOS: `pbcopy`
  - Linux: `wl-copy`, `xclip`, or `xsel`
  - Windows: `clip`

## Run

```bash
go run .
```

Flags:

- `-root` search root (default `.`)
- `-pattern` ripgrep regex for candidate extraction
- `-preview` toggle preview pane (default on)
- `-cache-size` highlight cache size (default `20000`)
- `-workers` highlight worker count (default `GOMAXPROCS-1`)
- `-visible-buffer` extra rows to pre-highlight (default `30`)
- `-debounce-ms` query debounce in ms (default `10`)
- `-highlight-context` one of `synthetic` or `file` (default `synthetic`)
- `-context-radius` file-context line radius (default `40`)
- `-editor-cmd` override open command. Placeholders: `{file}` `{line}` `{col}` `{target}`
- `-no-ignore` disable ripgrep ignore files (`.gitignore`, `.ignore`, `.rgignore`)

## Keybindings

- `up/down`, `j/k`: move selection
- `pgup/pgdn`, `ctrl+u/ctrl+d`: jump
- `tab`: toggle preview
- `y`: copy `path:line:col`
- `enter`: open in Zed (if available) or system file opener
- `esc`, `ctrl+c`: quit

Examples for `-editor-cmd`:

- VS Code: `-editor-cmd "code -g {target}"`
- Helix: `-editor-cmd "hx {file}:{line}:{col}"`
- Vim: `-editor-cmd "vim +{line} {file}"`

## Benchmarks

Run a quick benchmark pass:

```bash
go test -bench . -benchmem
```

Included benchmarks cover:

- filtering throughput on 50k candidates
- synthetic per-line highlight cost
- file-context per-line highlight cost
