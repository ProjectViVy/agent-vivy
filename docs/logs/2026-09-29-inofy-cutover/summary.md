# INOFY sole-path cutover (S11-E)

The workflow route now runs exclusively on the embedded INOFY engine.
`StartINOFYWorkflow` admits the normalized `inofy.workflow/v1` Definition
(canonical bytes + ProgramMeta), commits a schema-2 revision atomically,
and launches `Program.Run` with the S11-C atomic RunStore and the S11-D
governed child NodeExecutor. The INOFY terminal commit is the only graph
native terminal owner — the old `launchWorkflow` terminal emitter and the
`compose.NewWorkflow` execution path are deleted; the host never writes a
fabricated terminal for a workflow run.

Durable identity carries the full engine identity chain (ProgramDigest,
CatalogDigest, CompilerVersion, EinoBuild, InputDigest, EffectiveLimits,
HostBindingID). Duplicate starts join the same revision only when the
whole identity matches; a non-local active run or a stale running
projection fences the caller with `ErrWorkflowRecoveryRequired` — the
same-fresh-writer rule classifies it rather than silently replaying.
Discriminator-1 (legacy descriptor) rows stay historical: skipped by
auto-recovery, rejected by inspection (`ErrWorkflowLegacyFormat`) and
excluded from `workflow/list`. Old `{descriptor:…}` first-party payloads
fail at the RPC edge with `-32602`.

Inspection is backed by the committed projection plus journal replay:
`workflow/get` returns `definition` (canonical INOFY JSON), `engine_status`
(inofy projection status incl. `recovery_required`), per-node status with
deterministically derived `child_run_id`s, and `outputs` resolved from
committed `workflow_results` blobs through the declared output bindings.
The Run Inspector renders the same projection — nodes, revision digest,
engine status, declared outputs — inside the existing Children tab.

Cancellation propagates to graph-owned children. INOFY classifies a run
cancelled mid-effect as `recovery_required` — the honest outcome — and the
app worker stops polling instead of fabricating `cancelled`. Restart
recovery re-verifies authority (policy hash, sandbox, tool ceiling) and
definition identity before touching a run: admitted-only runs resume at
the next writer epoch and complete; running projections let the engine
commit `recovery_required` itself; terminal/waiting projections are left
settled.

Eino compose imports that remain in `internal/runtime` serve the chat
agent loop (ToolsNode, GetToolCallID, interrupt-rerun), not a graph
compiler — `nativeOrchestration*` is reachable only from conformance test
fixtures; no production path persists a `native-orchestration:` resume
target, so `compose.NewWorkflow` is production-unreachable.
