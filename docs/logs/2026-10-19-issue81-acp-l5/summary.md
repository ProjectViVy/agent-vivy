# Issue #81 E8 / L5 ACP batch — summary

Lane: `.worktrees/l5-acp/agent-vivy`, branch `feat/l5-acp` from `origin/main`
`e9609296`. Epic item: #81 E8 (restricted local ACP stdio Face), executed
through its three Stories E8-01 → E8-02 → E8-03. Work stays on the lane branch;
nothing was pushed, opened, merged, closed or released.

## What changed

**E8-01 — recovered and re-aligned.** The 16-commit ACP pilot on `origin/ACP`
(head `21c5b8bb`) was replayed onto current `main` by concern, not as one
blind squash: `plugins/acp` (the T2 stdio Face), `internal/faceprocess`
(private per-Face instance + selected launcher), the selected-face dispatch in
`cmd/vivy`, `RunSelectedFaceWithAppOptions`, the boot-config loader,
`RPCErrorCode`, session-root `turn/start.context_paths`, the pack-time overlay
that swaps `cmd/vivy/tui.go` for a pack-only stub, and the SDK probe module
(`sdk/internal/testdata/acp-compat`). The only merge conflict was
`internal/rpc/control.go`, resolved so main's newer `FileContexts` fallback and
the branch's project-context root both survive.

**E8-02 — human interaction through runtime authority.** `plugins/acp`
now answers the two committed interaction events instead of failing the pilot:
`tool.approval_required` becomes one `session/request_permission` with exactly
`allow-once`/`reject-once` and a bounded preview read from `review/get`
(`Action`/`Target`/`Preview`/`Risk`, never raw arguments);
`user.question_required` becomes one `elicitation/create` form with a single
required `answer` string (≤64 KiB UTF-8). Every decision writes the existing
durable path (`approval/respond`, `question/respond`, `review/respond`
`action: cancel`). Stale, foreign, expired, already-resolved or duplicate
replies have zero effect; unsupported clients, unusable previews and transport
failures deny rather than approve; 16 live dialogs per connection, then the owned
run is cancelled. Teardown withdraws live dialogs, cancels only the runs this
connection started (tracked across the prompt's lifetime) and waits for their
durable terminal state within 5 s, reporting — not hiding — an unsettled run.

**E8-03 — a packable, inspectable artifact.** Added `recipes/acp.vivy.yml`
(exclusive `std/face@v1: projectvivy/acp`, `rpc.client` grant, T2
`repo:plugins/acp` pin) and `plugins/acp/README.md`; regenerated the source
digest into both homes (`vivy-module.yaml`, `module.go`). The packer now carries
an external module's own `require`/version-`replace` directives into the
temporary build modfile (`mergeSourceModulePins`), because a dependency
module's `replace` is ignored by the Go toolchain — without that, the plugin's
SDK fork never reached the packed build. `resolveDependencyLocks` downloads the
full build list for the same reason. Added `TestACPRecipeSelection` (always) and
`TestACPPackedArtifactScriptedPeer` (opt-in), which packs the real Recipe,
inspects the artifact and drives a scripted ACP peer through
`initialize`/`session/new`/`session/prompt`/`session/cancel`/EOF.

New residual rows are recorded in `docs/TODO.md` (`ACP-CONFORMANCE-ROW`,
`ACP-ARTIFACT-E2E`, `ACP-WINDOWS-ACCEPTANCE`) and `ACP-SDK-UPSTREAM` carries
the verified fork commit.

## Scope kept

One Provider (`projectvivy/acp`, `std/face@v1`, kind `acp`) requiring
`core/face-host@v1` from `vivy/face-host` with the `rpc.client` grant; one local
ACP v1 stdio session → FaceHost Control RPC → the existing
Service/Journal/Policy/HITL authority. No second runtime, journal, storage or
policy path, no new Port, no UI change.

## Explicitly not done

- No push, PR, merge, issue closure, release or default-body activation.
- No remote ACP / HTTP / WebSocket control plane, no multi-Face coexistence, no
  ACP client filesystem/terminal methods.
- No provider-level conformance row (evidence is partial), no packed-artifact
  E2E in default CI, no Windows `.exe` acceptance; see
  `docs/logs/2026-10-19-issue81-acp-l5/verification.md`.
