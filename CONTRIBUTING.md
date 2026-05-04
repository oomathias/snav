# Contributing

Thanks for helping improve `snav`.

## Start with an issue

Before opening a PR, open or find an issue so we can align on scope and direction:

- Bugs and regressions: https://github.com/oomathias/snav/issues/new?labels=bug
- Feature requests and ideas: https://github.com/oomathias/snav/issues/new?labels=enhancement

## Prerequisites

- `mise`
- ripgrep (`rg`)
- C toolchain for Tree-sitter grammars (`clang` or `gcc`)

## Setup

```bash
git clone https://github.com/oomathias/snav.git
cd snav
mise install
```

## Run the project locally

Run with `go run` via `mise`:

```bash
mise run snav -- --root .
```

Install the built binary to `~/.local/bin`:

```bash
mise run link
snav --root .
```

`link` runs `build` first, then installs `bin/snav` as `~/.local/bin/snav`.

## Run checks before opening a PR

Use the same checks as CI:

```bash
mise run fmt
mise run lint
mise run test
mise run ci
```

## Contribution workflow

- Create a branch from `main`
- Keep changes focused and include tests for behavior changes
- Use clear commit messages (Conventional Commits preferred, for example `feat:`, `fix:`, `refactor:`, `docs:`)
- Open a PR that links the issue and explains the problem, approach, and validation steps
