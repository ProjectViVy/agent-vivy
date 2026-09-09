# WEB-2: Restricted-mode false rejection when WriteFile sandbox validation precedes MkdirAll

Date: 2026-09-01 | Branch: `feat/vc1a-bash-tool` | worktree: `agent-vivy-vc0`

## What changed

`EinoFilesystemBackend.WriteFile` in `internal/runtime/filesystem_backend.go` runs
`ValidatePathWithMode` before MkdirAll. That validation already falls back when the target
file does not exist (using EvalSymlinks on the parent), but when the parent itself does not
exist (a completely new nested directory), `EvalSymlinks(parent)` fails →
`"resolve parent symlinks"` — under workspace-write mode, `write_file` to any new nested
directory was falsely rejected (danger mode short-circuits validation, so existing tests did
not expose it).

The fix uses the same `resolve → MkdirAll → Validate` order already applied and documented
in download.go (`internal/runtime/download.go:100`):

1. Run `resolve()` first: `safeWorkspacePath` performs component-by-component Lstat, rejects
   symlink components, and confines the path to the workspace before any directory creation
   (the same reasoning documented in download.go).
2. During `CreateParents`, run MkdirAll and then resolve again.
3. Move sandbox validation to after the parent directory exists and before atomicWrite.

Behavioral boundaries (deliberate):

- A same-content no-op write (`bytes.Equal` early return) no longer undergoes sandbox
  validation — zero bytes are written and there is zero change; blocking a no-op in read-only
  mode is not a security property.
- When `CreateParents=false` and the parent directory is missing, the error remains a sandbox
  resolution error (consistent with before the fix; atomicWrite would then fail as well).
- Symlink escape protection is unchanged: resolve() rejects symlink components with
  component-by-component Lstat before directory creation, so MkdirAll cannot use them to
  escape; `ValidatePathWithMode`'s EvalSymlinks remains the second line of defense.

## Deliberately not done

- Do not add a semantic switch to `ValidatePathWithMode` to "allow a missing parent
  directory": that would affect every caller, and the download.go precedent already shows
  that changing the call-site order is sufficient.
- PrepareWriteFile (proposal construction) has no sandbox validation by design (the proposal
  has zero changes and is revalidated at execution), so it remains unchanged.

## Acceptance criteria

- Under workspace-write mode, `write_file` to a completely new nested directory is written
  successfully (it was falsely rejected before the fix).
- Writes in read-only mode are still explicitly rejected (ErrSandboxDenied).
- danger mode behavior is unchanged (short-circuited).
