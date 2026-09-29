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

# S11-F reusable definitions and host actions

Core Storage now owns reusable INOFY workflow definitions: session-author
draft rows (`workflow_definition_drafts`) under CAS/ETag and organism-visible
immutable published rows (`workflow_definition_revisions`) allocated
monotonically with artifact/catalog digests — migration 035 on both drivers,
one contract (`storage.WorkflowDefinitionStore`), one conformance suite.
`workflow_revisions` gains `input_json` plus the definition binding
(`definition_id`, `definition_revision`) so an admitted product run is
faithful to its source and restart recovery re-executes the exact canonical
input; schema-2 admission requires the input, legacy rows may not carry the
binding.

The host surface implements `definitions.Repository` on the store (artifact
round-trip, digest recomputation, error mapping onto `inofy.Error` codes)
and a product service on `*runtime.Service`: capabilities (honest
`supports_wait`/`supports_resume` false — the trusted catalog has no
wait-capable node), nodeTypes, saveDraft, validate (catalog diagnostics plus
one `host_admission` capability diagnostic checked against the full workflow
tool universe), publish (host admission gate first so no unstartable
revision becomes immutable), getRevision, listWorkflows, startRun
(revision-bound or draft-etag snapshot, routed through the S11-B/C/D
admission path — never a second engine), listRuns/getRun scoped to the
caller's session (foreign ids answer not-found), Journal-backed event
paging, committed node-output lookup, governed cancel, and unsupported
resume.

The RPC layer exposes the literal `inofy.*` method names the editor bridge
calls (`internal/rpc/inofy_product.go`): session resolution binds the peer
identity through `session_id` validated by the SessionStore (fail-closed);
errors carry `data.code` for the editor's ApiError surface; connections map
the existing provider registry read-only (kind/base_url/model/has_secret —
credentials never cross) and put/delete answer `unsupported_feature`. Live
event subscription reuses the existing `run/subscribe` machinery.

# S11-G embedded INOFY editor in the vivy/workflow-ui Module

The product editor ships as the selected `vivy/workflow-ui` Module
(`std/ui-extension@v1`): the default Recipe picks it, stage-ui emits
`vivy.workflow-ui.sidebar`, and a grouped sidebar entry routes `/workflows`
into the module page — no standalone Studio shell.

`src/studio/` vendors the INOFY shared editor tree at the same pinned
revision as `go.mod` (`6acfcc6b1a51`), byte-verbatim except the
`statusFor` transport-error adaptation (`vivy-transport.ts`) documented in
`VENDORED.md`; the graph unit suite runs unmodified from the staged copy.
`WorkflowPage` mounts the editor inside a Shadow DOM with textually
rescoped vendored CSS (`:root`→`:host`, `body`→`.studio-shell`) plus
xyflow styles, so host and editor styles never leak either way.

`FaceBridge` is the single host authorization path: every `inofy.*` call
carries `session_id` from the committed host store (fail-closed — the key
is omitted when no session is active), `startRun` binds `parent_run_id`
to the current run and a fresh ≤128-byte `operation_id`, and
`inofy.events` multiplexes onto `run/subscribe`/`run/event` notifications
with Journal `type`→engine `kind` normalization, cursor dedup, terminal
close, and resubscribe-from-cursor reconnect — journal stays the event
authority.

Honest surface only: capabilities come from `inofy.capabilities`,
TransportError codes map onto the editor's 412/409/401/422/501/503
branches, node/Run status, protected output, and cancel all derive from
committed host facts. Module chrome carries en/zh catalog keys; the
vendored editor keeps its own upstream zh table, so the i18n gate
skips directories marked `VENDORED.md` rather than pretending vendored
copy is Module-authored.
