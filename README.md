# snav

<p align="center">
  <img src="./docs/snav.gif" alt="snav demo" />
</p>

<p align="center">
  <strong>Interactive symbol finder for large codebases.</strong><br />
  Fast symbol search, fuzzy ranking, and instant open in your editor.
</p>

## Why snav

- Streams candidates with `rg --vimgrep --trim`
- Ranks by symbol key first, then surrounding context
- Renders syntax-highlighted source lines with optional preview
- Supports themes and custom editor open commands
- Runs on macOS, Linux, and Windows

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/m7b/snav/main/install.sh | bash
```

Build from source:

```bash
go build -buildvcs=false -o snav ./src
./snav -root .
```

## Requirements

- Go 1.24+
- ripgrep (`rg`)
- C toolchain for Tree-sitter grammars (`clang` or `gcc`)

## Quick usage

```bash
snav -root .
```

Useful flags:

- `-theme github`
- `-highlight-context file`
- `-editor-cmd "code -g {target}"`
- `-no-test` and `-no-ignore`

## Keybindings

- `j/k` or arrows: move
- `tab`: toggle preview
- `enter`: open result
- `y`: copy `path:line:col`
- `esc` or `ctrl+c`: quit

## License

MIT - see `LICENSE`.
