# VC-3 slice 4: 编辑后诊断回填（diagnostics backfill）

Date: 2026-09-01. Lane: `feat/vc1a-bash-tool` (worktree `agent-vivy-vc0`).

## What changed

The D4 open design point「插件诊断与内核 write/patch 结果的回填衔接（待定观察者契约设计）」is
now resolved and implemented. After the kernel's `write_file` / `patch` /
`multiedit` tools change a file, the mutation result carries lint/type
findings from tool-world plugins, so the model sees errors without a
separate `lsp_diagnostics` call (the mechanism the crush research flagged as
the key coding-quality lever).

Observer contract (four layers, all additive):

- `sdk/plugin`: optional `DiagnosticObserver` interface (`Plugin` +
  `ObserveWrite(ctx, env, paths) []string`). Tool-world only; lines are
  pre-formatted text; nil/empty means "nothing to report". Grants are
  enforced by the `Env` the kernel hands in — the observer stays sandboxed
  exactly like a tool call.
- `internal/tools`: optional `WriteDiagnosticsSource` extension of the ops
  surface; `FileMutationResult` gains a `diagnostics` string field
  (`omitempty`, so default-EXE results are byte-identical).
  `attachWriteDiagnostics` runs after a successful mutation and only when
  `Changed`; a failing source can never fail the mutation (write already
  succeeded — diagnostics are best-effort).
- `internal/runtime`: `EinoFilesystemBackend.SetWriteDiagnostics` +
  forwarding `WriteDiagnostics` — the backend is the mount point, it owns
  no plugin knowledge.
- `internal/pluginhost`: `DiagnosticBridge` implements the kernel contract
  by fanning out to every registered tool-world plugin implementing
  `DiagnosticObserver`, in registration order, each with its own granted
  Env. Channel-seam plugins and non-observers are skipped. The composition
  root (`internal/app`) wires the bridge right after the workspace lookup
  is built.

`plugins/lsp` implements the observer: per path, unknown language → skip
(no server, zero latency); servers are reused across calls; the
diagnostics wait is 2s (deliberately shorter than the interactive 3s
default) and each file contributes at most 30 lines. Silence means
"nothing to report", never "success".

## Boundaries (deliberate)

- `bash`-written files are not backfilled — the backfill rides the file
  mutation tool results; the model can call `lsp_diagnostics` explicitly
  for shell-side changes. Crush's backfill covers its edit tools likewise.
- The eino-native middleware write path (`Backend.Write/Edit`) has no
  result channel for the model, so backfill only rides Vivy's own tool
  results (`write_file`/`patch`/`multiedit`).
- The approval flow is untouched: the backfill runs inside the same tool
  run after approval, post-mutation, read-only.
- No UI change, no new config knobs (wait/cap are constants in the plugin).

## Crush alignment

Behavior/protocol alignment only; zero code copied (Crush is FSL-1.1-MIT).
Crush attaches LSP diagnostics to edit results; Vivy reaches the same model
experience through an optional plugin observer contract, which is the D4
plugin architecture, not a port.

## Explicitly not done

- `positionEncoding` negotiation (stays on the VC-3 余 list).
- No kernel-side dedup of identical diagnostics across successive writes —
  each mutation reports its own fresh round.
