# 🦘 snav

<p align="center">
  <strong>Interactive symbol finder</strong><br />
</p>

<p align="center">
  <i>IDE - Zed</i>
</p>
<p align="center">
  <video controls preload="metadata" src="https://github.com/user-attachments/assets/43d65503-fce5-4b78-be50-de55429a5b4b"></video>
</p>

<p align="center">
  <i>Terminal - Ghostty</i>
</p>
<p align="center">
  <video controls preload="metadata" src="https://github.com/user-attachments/assets/8d6067bb-777e-47f0-a273-f528ea9af823"></video>
</p>

## What is snav?

`snav` is a terminal UI for jumping to any symbols.

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
curl -fsSL https://raw.githubusercontent.com/oomathias/snav/main/install | bash
```

Default install path: `/usr/local/bin`.

Optional local install path:

```bash
SNAV_INSTALL_DIR="$HOME/.local/bin" curl -fsSL https://raw.githubusercontent.com/oomathias/snav/main/install | bash
```

By default, the installer auto-enforces signature verification when `cosign` is available.

Optional: enforce signature verification explicitly (requires `cosign`):

```bash
SNAV_REQUIRE_SIGNATURE=1 curl -fsSL https://raw.githubusercontent.com/oomathias/snav/main/install | bash
```

## Terminal usage

```bash
snav --root .
```

Keys:

- Type to filter symbols
- `up/down` or `ctrl+p/ctrl+n`: move
- `tab`: toggle preview
- `enter`: open selected result
- `ctrl+space`: copy `path:line:col`
- `esc` or `ctrl+c`: quit

## Common flags

- `--exclude-tests`: ignore common test files/directories
- `--no-ignore`: include files ignored by `.gitignore`, `.ignore`, `.rgignore`
- `--theme github`: set color theme
- `--highlight-context synthetic`: use line-only highlighting
- `--editor-cmd "code --goto {target}"`: custom open command

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
- Jump around

## Cache

`snav` keeps one local index cache for the most recent root and scan options.
When settings match, cached results load first and a rescan refreshes in the background.

## License

MIT - see `LICENSE`.
