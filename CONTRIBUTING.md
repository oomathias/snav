# Contributing

Thanks for helping improve `snav`.

## Start with a discussion

Before opening a PR, start a discussion so we can align on scope and direction:

- Issues: https://github.com/oomathias/snav/discussions/categories/issues
- Feature requests/ideas: https://github.com/oomathias/snav/discussions/categories/feature-requests-ideas

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
- Open a PR that explains the problem, approach, and validation steps
