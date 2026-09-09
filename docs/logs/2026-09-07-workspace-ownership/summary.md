# TUI-WORKSPACE-OWNERSHIP — workspace RPC ownership validation

## Summary

The `/files` run-workspace RPC (`workspace/list`, `workspace/read`) previously
called `WorkspaceFiles.Ensure` directly for a format-valid but unknown run id,
creating a directory on disk—the control plane had never proved that the run
belonged to this instance's Journal. This closes the surface fail closed:

- **Control-plane precheck (`internal/rpc/control.go`)**: added
  `workspaceRunOwner`; before touching the filesystem, both handlers call
  `Runs.GetRun` to prove that the run exists in this Journal. An unknown run
  returns `run not found` (404 semantics, consistent with `run/get`); it is also
  rejected when the Run store is not wired.
- **Zero runtime side effects (`internal/runtime/workspace_files.go`)**:
  `WorkspaceFiles` switches from `manager.Ensure` (creates a directory) to
  `manager.Existing` (never creates one). A run whose workspace does not yet
  exist returns the new sentinel error `ErrWorkspaceNotFound`, which the
  control plane maps to `run workspace not found` 404. The UI accessor's
  contract is now "never create filesystem state."
- **Read race hardening**: `Read` opens the file, calls `Stat` on the handle,
  and then reads from that same handle—even if the path is replaced with a
  symlink between Lstat and open, the bytes leaving the function come from the
  validated regular-file handle. Reads now use `io.LimitReader` bounded by the
  byte cap (the old implementation read everything into memory before
  truncating).
- **Tests**: the RPC layer adds two tests for unknown-run fail-closed behavior
  (asserting zero accessor calls) and workspace-not-found→404 mapping; the
  runtime layer adds two tests for unknown/malformed run ids (control
  characters `\x1b`/`\x00`, `..`, and path separators) failing closed with zero
  directory creation, and for rejecting a final-component symlink.

The control plane uses trusted loopback transport and has no per-client
identity, so "run belongs to this Journal" existence is the ownership fact; no
new authentication concept was introduced (to avoid scope expansion).

## Explicitly not done

- Did not add secret-filename filtering for `workspace/list` (the
  project-context surface already has this pattern; it belongs to another
  surface).
- Did not add per-client RPC authentication (the protocol has no such concept;
  the loopback trust model is unchanged).
- The residual symlink-follow window at the instant `Read` opens a file cannot
  be completely closed with a portable API under POSIX semantics; it is
  mitigated by the Lstat + handle Stat + workspace symlink-free invariant.

## Filing

- Board: the `docs/TODO.md` §0.1 TUI-WORKSPACE-OWNERSHIP row → §10 completion log.
- Related: `docs/plans/2026-09-07-mcp-stdio-upstream.md` is unrelated; this was an independent delivery.
