# snav

> [!WARNING]
> Experimental: behavior may change between releases.

<p align="center">
  <img src="./assets/snav.gif" alt="snav demo" />
</p>

<p align="center">
  <strong>Interactive symbol finder for codebases.</strong><br />
</p>

## What is snav?

`snav` is a terminal UI for jumping to symbols in a repository.

Type a query, pick a result, and open the exact `file:line:col`.

## Why use it

- Find functions, types, classes, constants, and more across the repo
- Fuzzy-ranked results while you type
- Syntax-highlighted preview before opening
- Fast reopen with local cache
- Works on macOS, Linux, and Windows

## Install

### 1) Install `ripgrep`

`snav` requires `rg` on your `PATH`.

### 2) Install `snav`

```bash
curl --fail --silent --show-error --location https://raw.githubusercontent.com/oomathias/snav/main/install | bash
```

Default install path: `/usr/local/bin`.

Optional local install path:

```bash
SNAV_INSTALL_DIR="$HOME/.local/bin" curl --fail --silent --show-error --location https://raw.githubusercontent.com/oomathias/snav/main/install | bash
```

## Zed setup

### 1) Add a task

`~/.config/zed/tasks.json`:

```json
[
  {
    "label": "snav",
    "command": "snav --exclude-tests --root $ZED_WORKTREE_ROOT",
    "use_new_terminal": false,
    "allow_concurrent_runs": false,
    "reveal": "always",
    "reveal_target": "center",
    "hide": "always",
    "shell": "system",
    "show_summary": false,
    "show_command": false
  }
]
```

### 2) Add a keybinding

`~/.config/zed/keymap.json`:

```json
[
  {
    "context": "Workspace",
    "bindings": {
      "cmd-shift-t": [
        "task::Spawn",
        {
          "task_name": "snav"
        }
      ]
    }
  }
]
```

### 3) Use it

- Press your keybinding
- Type to filter symbols
- Move with `j/k` or arrows
- Press `enter` to open

## Terminal usage

```bash
snav --root .
```

Keys:

- Type to filter symbols
- `j/k` or arrows: move
- `tab`: toggle preview
- `enter`: open selected result
- `y`: copy `path:line:col`
- `esc` or `ctrl+c`: quit

## Common flags

- `--exclude-tests`: ignore common test files/directories
- `--no-ignore`: include files ignored by `.gitignore`, `.ignore`, `.rgignore`
- `--theme github`: set color theme
- `--highlight-context synthetic`: use line-only highlighting
- `--editor-cmd "code --goto {target}"`: custom open command

## Cache

`snav` keeps one local index cache for the most recent root and scan options.
When settings match, cached results load first and a rescan refreshes in the background.

## License

MIT - see `LICENSE`.
