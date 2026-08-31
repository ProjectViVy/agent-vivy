# VC-1c: grep/glob tools (rg-first + doublestar)

Date: 2026-08-31 · Lane: `agent-vivy-vc0` worktree, branch `feat/vc1a-bash-tool`

## What changed

Two new readonly first-party tools, `grep` and `glob`, close the core file-search
gap against Crush (VC-1c on the V0 → VC-3 board). They register through the same
type-assertion seam as the bash family: `internal/tools` gains a Vivy-owned
`GrepOperations` interface (no Eino types cross the seam, D-007), implemented by
`EinoFilesystemBackend`.

- `internal/tools/search.go` (new): `grepTool` / `globTool` specs, decoders,
  and `GrepOperations` interface. Both tools are `Readonly: true`, so the
  existing policy engine runs them without approval interrupts under any
  approval policy.
- `internal/runtime/search_backend.go` (new): the backend implementation.
  - **Grep**: ripgrep subprocess first (`-n --no-heading --no-messages
    --no-require-git --path-separator / [--glob include] -- pattern`, cwd =
    search root). rg honors `.gitignore` (with `--no-require-git` so run
    workspaces that are not git repos still get ignore semantics). On any rg
    failure the bounded pure-Go fallback walk runs (same semantics as the
    legacy `search_files` walk: prune `.git`/`.svn`/`node_modules`, skip
    binaries and oversized files). rg output is capped (result cap, 8 MiB
    stream cap, 30 s timeout); on cap the process is killed early while the
    pipe keeps draining, so `cmd.Wait` never deadlocks on a full pipe.
  - **Glob**: doublestar (`github.com/bmatcuk/doublestar/v4`, promoted from
    indirect to direct) over a `WalkDir` that prunes ignored directories and
    symlinks; `**` recursion now works (the previous hand-rolled
    `filepath.Match` matcher could not). Results sort newest-first (mtime
    desc, path asc tiebreak) and carry size + modified time. Patterns with
    `..` path segments are rejected before normalization.
- `internal/runtime/filesystem_backend.go`: `rgPath` probe (`exec.LookPath`)
  in the constructor; `GrepRaw` (the Eino middleware path) now delegates to
  the same fallback walk via the new `searchRoot`/`goGrep` helpers instead of
  duplicating the walk; `matchesGlob` delegates to doublestar-backed
  `globPatternMatches`, upgrading the Eino `GlobInfo` path to `**` too.
- `internal/tools/tools.go`: both `builtinWithWeb` and `baseToolsForSearch`
  append `NewGrep`/`NewGlob` when `files` implements `GrepOperations`.
- `internal/config/config.go`: `grep` and `glob` join the default
  `tools.enabled` surface (before `tool_search`).
- `go.mod`/`go.sum`: doublestar v4.10.0 now a direct dependency.

## Scope boundaries (Crush alignment, no extras)

- `grep` and `glob` mirror Crush's two tools: regex content search with an
  optional include glob; `**`-capable path matching, newest first. No
  multiline mode, no fuzzy finder, no file-content ranking, no additional
  flags beyond the two tools' parameters.
- Glob intentionally has **no** .gitignore awareness (doublestar is a pure
  matcher); only grep gains gitignore semantics, via rg. This matches the
  ratified reuse dispositions (reuse inventory §B: rg-first grep, doublestar
  glob) and is documented in both tool descriptions.

## Explicitly not done

- VC-1d (multiedit + patch whitespace tolerance + file_versions) — next track.
- The Eino middleware `SearchInfo` path still uses its own query search; only
  `GrepRaw`/`GlobInfo`/`matchesGlob` were unified.
- No UI changes (routeTree.gen.ts line-ending noise from a previous checkout
  was left untouched and is excluded from the commit).
