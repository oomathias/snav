# AGENTS.md

Guidance for coding agents working in this repository.

## Repository snapshot

- Language: Go (`src/go.mod`, Go `1.24.2`).
- App type: terminal UI CLI (`snav`) for symbol navigation.
- Entrypoint: `src/main.go`.
- Key internal packages: `src/internal/candidate`, `src/internal/highlighter`, `src/internal/lang`.
- Tests: unit tests near code, plus black-box/integration-style tests in `src/tests`.
- Task runner and pinned toolchain: `mise.toml`.

## Layout and prerequisites

- `src/`: all Go code and tests (module root).
- `bin/`: local build output (`bin/snav`).
- `assets/`: demo/media assets.
- CI checks in `.github/workflows/ci.yml` (`fmt-check`, `lint`, `test`).
- Required tools: `mise`, `rg`, and a C toolchain (`clang` or `gcc`) for tree-sitter grammars.

## Canonical commands (repo root)

Prefer `mise` tasks unless you need tool-level debugging.

```bash
# one-time setup
mise install

# run app locally
mise run snav -- --root .

# formatting and validation
mise run fmt
mise run fmt-check
mise run lint
mise run test
mise run ci

# build/install
mise run build
mise run link

# benchmarks
mise run bench
```

## Direct Go commands (without mise)

Use these when you need tighter control than task wrappers.

```bash
# test and vet
go -C src test ./...
go -C src vet ./...

# build
go -C src build -buildvcs=false -o ../bin/snav .

# benchmarks
go -C src test ./... -bench . -benchmem
```

## Running a single test (important)

```bash
# single test across all packages
go -C src test ./... -run '^TestName$'

# single package + single test (fast path)
go -C src test ./internal/candidate -run '^TestFilterCandidatesSubsetMatchesFull$'

# integration-style tests package
go -C src test ./tests -run '^TestExternalHelpFlag$'

# one benchmark only
go -C src test ./... -run '^$' -bench '^BenchmarkFilterCandidates50k$' -benchmem
```

LLVM benchmarks need a large checkout path:

```bash
SNAV_BENCH_ROOT=/path/to/llvm go -C src test ./... -run '^$' -bench '^BenchmarkLLVM'
```

## Package-scoped lint and test tips

- `mise run lint` already runs `go vet` and `golangci-lint`.
- To lint a specific package, run from module root (`src/`): `golangci-lint run ./internal/candidate`.
- For quick inner-loop checks, prefer package-scoped tests over `./...` when possible.
- Typical package paths: `./internal/candidate`, `./internal/highlighter`, `./internal/lang`, `./tests`.

## Code style: formatting and imports

- Always run `gofmt` (`mise run fmt`) after edits.
- Keep import groups in this order when applicable:
  1. standard library,
  2. local module imports (`snav/...`),
  3. third-party imports.
- Keep one blank line between import groups.
- Let `gofmt` decide spacing/alignment; do not hand-align.
- Keep functions readable; extract helpers when blocks become dense.

## Code style: types and data modeling

- Prefer concrete structs and typed aliases over unnecessary interfaces.
- Reuse existing alias/enum patterns (example: `type LangID = lang.ID`).
- Use explicit config structs for wiring (`config`, `ProducerConfig`, `HighlighterConfig`).
- Preallocate slices where expected size is known (`make(..., 0, n)`).
- Return `nil` slices for empty semantic values when that is the existing pattern.

## Code style: naming

- Exported identifiers: `PascalCase`.
- Unexported identifiers: `camelCase`.
- Tests: `TestXxx...`; benchmarks: `BenchmarkXxx...`.
- Prefer domain-specific names (for example `FilterCandidatesRangeWithQueryRunes`).
- Keep local loop/index names concise (`i`, `j`, `idx`, `ok`, `err`).

## Code style: error handling

- Return errors instead of panicking on runtime paths.
- Wrap propagated errors with context and `%w`.
- Keep CLI-facing errors clear and actionable.
- In `main`, print fatal errors to stderr and exit non-zero.
- Use `_ = ...` best-effort cleanup only when cleanup failure must not hide the primary error.
- Optional external tools should degrade gracefully (for example `exec.LookPath`).

## Code style: concurrency and performance

- Use `context.Context` for cancellation boundaries (`StartProducer` pattern).
- Producer goroutines own closing of their output and done channels.
- Guard shared mutable state/maps with `sync.Mutex` or `sync.RWMutex`.
- Keep UI/event loops non-blocking; use buffered channels where established.
- Keep hot paths allocation-aware (rune reuse, preallocated buffers, cached lines/spans).

## Code style: paths, text, and portability

- Use `filepath` helpers for path operations.
- Normalize CRLF before line splitting (`strings.ReplaceAll(..., "\r\n", "\n")`).
- Be explicit about rune-vs-byte behavior in rendering/highlighting code.
- Preserve platform-specific behavior using `runtime.GOOS` branches.

## Testing conventions

- Keep focused unit tests near the package under test.
- Use `package main_test` for black-box tests in `src/tests`.
- Use `t.Helper()` for shared helpers and `t.Cleanup()` for restoring globals.
- Use `t.TempDir()` for filesystem isolation.
- Prefer deterministic assertions with clear `got`/`want` messages.
- For multi-case scenarios, use table-driven tests with `t.Run`.

## Agent checklist before finishing

- Keep changes scoped; avoid unrelated refactors.
- Add or update tests when behavior changes.
- Run at least: `mise run fmt-check`, `mise run lint`, `mise run test`.
- Run targeted benchmarks for performance-sensitive changes.
- If CLI flags/help text change, update coverage in `src/tests/cli_test.go`.
