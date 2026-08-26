# 2026-08-26 — list_dir workspace exploration tool

## What changed

Vivy's agent toolset gained a `list_dir` tool (readonly, auto-executing), so a
model can explore an unfamiliar run workspace without already knowing text to
search for (`search_files` needs a known needle; `read_file` needs a known
path).

Scope of the change:

- `internal/tools/filesystem.go` — new Vivy-owned contract: `ListDirName`,
  `DirListRequest` (`path`, `recursive`, `depth`, `max_entries`),
  `DirEntry`/`DirListResult`, `ListDir` added to `FileOperations`, and the
  `listDirTool` with spec + arg decoding in the existing string-args style.
- `internal/runtime/filesystem_backend.go` — `EinoFilesystemBackend.ListDir`:
  sandbox read validation (same pattern as `ReadFile`), non-recursive
  `os.ReadDir` listing, and a bounded `WalkDir` for recursive mode. Recursive
  walks list ignored directories (`.git`, `.svn`, `node_modules`) but never
  traverse them, mirroring the search walk policy. Bounds: entry cap 200
  (request-clamped, `Truncated` flag on overflow), depth default 4 / max 10.
  Non-directory targets fail with an explicit error; path escapes, symlinks,
  and protected paths are rejected by the existing `safeWorkspacePath` machinery.
- Registration: `NewListDir(files)` added to all three builtin constructor
  lists in `internal/tools/tools.go` (`BuiltinWithFileOps`,
  `BuiltinWithCommands`, `baseToolsForSearch`).
- Default enablement: `list_dir` added to `Tools.Enabled` defaults and to the
  sandbox `auto_approve_tools` defaults in `internal/config/config.go`, plus
  both lists in `config.example.yaml` and the example snippet in
  `docs/dev/sandbox.md`.

Versus the DIVA reference this ports the idea from, Vivy's version is
recursive-capable (`recursive` + `depth` parameters) instead of flat-only.

## Explicitly not done

- No Eino ADK middleware wiring change — the Eino `filesystem.Backend` side
  (`LsInfo`/`GlobInfo`/`GrepRaw`) was already implemented; this delivery only
  adds the Vivy direct tool family member.
- No UI change (tool catalogs are server-driven; the UI renders them
  dynamically).
- No other agentg capability-gap items (B, C, ...) — this delivery is item A
  only.
- No TODO §0.1 entry: found and fixed in the same iteration.

## Verification

See `verification.md`. Gate: `just ci` green (fmt-check, vet, full test,
headless compile, UI tests + build). Real-path smoke: control-plane RPC
`session/create` → `preflight/run` selected exactly `["list_dir"]` and the
policy engine returned `allow` (readonly default).

## Acceptance

See `acceptance.md`.
