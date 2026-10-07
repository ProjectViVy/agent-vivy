# ACP-03 Sessions Prompts and Committed Projection Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans. Track the ordered steps with checkboxes; delegation is optional and requires the applicable authorization.

**Goal:** Construct the ACP module and execute owned prompts with ordered safe output and isolated cancellation.
**Architecture:** One SDK connection owns session and prompt generations; Control/Runtime remain authoritative. A bounded reducer orders committed events and hands interactions to ACP-04 without blocking.
**Tech Stack:** Go 1.26.4; ACP wire 1 / schema-v1.21.0; existing FaceHost and Control; SDK pin accepted by ACP-01.
**Spec:** [Reconciled detailed design](../../specs/2026-10-07-acp-stdio-face-design.md), executable baseline dd78fcf142f384d47ce5cfefb43738fdb9a7346d.
**State / dependencies:** [Single index](index.md#story-status-and-dependencies). Requires accepted ACP-02 interfaces, startup and isolation evidence.

## Global Constraints

Follow the [shared constraints](index.md#global-constraints), proposed limits in design section 10, and repository plugin/kernel workflows. This conditional plan does not approve its public interfaces or schedule G1. ACP-01 freezes them and revises downstream steps before execution. No product data access or generated-assembly hand edits.

## Review Focus

RF-1/2/3: URI forms and no-fetch references, session reservations, early subscribe callbacks, cancel before run ID and final response ordering.

## Files and interfaces

Proposed plugins/acp/{go.mod,go.sum,vivy-module.yaml,module.go,agent.go,control.go,transport.go,module_test.go,agent_test.go,control_test.go,transport_test.go,input_test.go}.
Read faces/headless/module_v1.go and go.mod for the existing provider/module pattern. The plugin module path is agent-vivy/plugins/acp; its local agent-vivy replacement points two levels up (replace agent-vivy => ../..).

Public producers:

- func New() module.Module.
- func NewProvider() face.FaceProvider; Definition returns ID projectvivy.acp, Kind acp.
- Provider.Construct(context.Context, face.Host) (face.Instance, error) requires face.TextPresentationHost.
- boundFace.Run(context.Context, face.Options) (face.Result, error), requiring nonnil input/output/error streams.

Private types/functions for the remaining tasks and ACP-04:

- agent embeds acp.BaseAgent and owns the Host, presentation facet, SDK connection, immutable negotiated capabilities and session map.
- sessionState stores canonical root, a mutex, next generation and the later active prompt slot.
- agent.Initialize(ctx, acp.InitializeRequest) (acp.InitializeResponse, error).
- agent.NewSession(ctx, acp.NewSessionRequest) (acp.NewSessionResponse, error).
- func normalizePrompt(root string, req acp.PromptRequest) (text string, contextPaths []string, err error).
- func (a *agent) call(ctx context.Context, method string, params any) (json.RawMessage, error), restricted to the design section 12.2 method allowlist and a 10-second child timeout.
- func safeRPCError(err error) *acp.RPCError, implementing the exact design section 12.7 mapping.
- func newConnection(a *agent, in io.Reader, out io.Writer) (*acpconn.AgentConnection, error), using only the accepted G0 public options. ACP-01 must replace this plan's dependency reference with the exact selected constructor/options before execution.

## Prompt and projection files and interfaces

Proposed plugins/acp/{prompt.go,projection.go,prompt_test.go,projection_test.go}; extend agent.go, control.go and their tests.
Read internal/rpc/control.go eventResult/streamRun/startTurn and schemas/events/payloads/{model.delta,model.request,model.completed,tool.requested,tool.started,tool.finished}.json.

Consumers: the preceding module/session tasks' agent/sessionState, call, normalizePrompt, safeRPCError, SDK connection and presentation facet.
Producers:

- func (a *agent) Prompt(ctx context.Context, req acp.PromptRequest) (acp.PromptResponse, error).
- func (a *agent) SessionCancel(ctx context.Context, req acp.CancelNotification) error.
- promptScope { SessionID string; RunID string; Generation uint64 } and promptState, owned by a single session slot until finalization.
- committedEvent { RunID string; Seq int64; Type string; PayloadVersion int; Payload json.RawMessage }, decoded from the existing event envelope with exact snake_case JSON tags.
- runProjection with constructor newRunProjection(scope promptScope, sanitize face.TextSanitizer) *runProjection.
- func (p *runProjection) Apply(event committedEvent) ([]acp.SessionNotification, error), called only by the ordered reducer; it returns no raw payload.
- func opaqueToolID(nonce []byte, scope promptScope, toolCallID string) string: SHA-256 of length-prefixed nonce/session/run/tool inputs, hex output.
- interactionJob { Scope promptScope; Event committedEvent; ToolID string }; ACP-04 consumes it asynchronously. Until that consumer exists, an interaction event cancels the run with an explicit unsupported-interaction reason; it must not wait forever or auto-approve.

## Task 1: Construct a pure module and bounded stdio connection

- [ ] Write TestModuleIsPure, TestProviderRequiresTextPresentation and TestACPTransportBoundary. Module construction performs no I/O; absent optional Host facet fails explicitly; nil streams fail; logging is disabled before any SDK worker starts.
- [ ] In plugins/acp run go test -run 'Test(ModuleIsPure|ProviderRequiresTextPresentation|ACPTransportBoundary)$' -count=1. Initial failure: new provider absent.
- [ ] Implement descriptor/manifest and lifecycle using the established typed module pattern. Request only rpc.client. Add the exact accepted dependency, root SDK dependency and local replacement; do not alter generated assembly.
- [ ] Implement transport configuration: 1 MiB inbound, 64 pending SDK frames and accepted timeout controls. The SDK keeps its serialized writer; the launcher retains final stream ownership. If the selected public API cannot enforce the accepted contract, stop and reopen G0 instead of adding a wrapper queue after allocation.
- [ ] Rerun tests, then go vet ./... in the nested module. Capture go list -deps ./... and verify no plugin import crosses the internal/Eino firewall or selects SDK remote transports.

## Task 2: Negotiate and create owned sessions

- [ ] Write TestInitializeRestrictedPilot, TestDuplicateInitializePreservesSessions and TestSessionAdmission. Core code_mode_available must be present; response protocolVersion=1, loadSession=false, image/audio/embeddedContext=false, http/sse=false, sessionCapabilities={}, authMethods=[].
- [ ] Test absent/null/nonempty mcpServers, nonabsolute/control-character cwd, nonempty additionalDirectories and the 32-session boundary including concurrent reservations. Assert these exact side-effect rules:

~~~text
invalid request -> session/create call count == 0
nonempty mcpServers -> -32602 and CLIENT_MCP_UNSUPPORTED; no echoed values
32 sessions or reservations -> next request returns -32001; count remains 32
successful create -> real sessionId and canonical root retained; wire returns only sessionId
same-looking ID from another connection -> -32002 before Control
~~~

- [ ] Run go test -run 'Test(InitializeRestrictedPilot|DuplicateInitialize|SessionAdmission)$' -count=1; observe the intended initial failures.
- [ ] Implement serialized initialization and atomic session reservations without holding locks across Control. Call initialize, derive actual artifact name/version from the sealed Generation/module identity and intersect supported capabilities. Validate the entire new-session request, call session/create {workspace_path: cwd}, and store only its real ID/canonical root.
- [ ] Release reservations after definite errors. Ambiguous session creation drains the private connection; never retry and allocate another durable session. Duplicate initialization returns a safe invalid-request result without changing connection/session state.

## Task 3: Pin input translation and safe Control failures

- [ ] Write TestPromptNormalization, TestResourceURIPlatformForms and TestControlErrorProjection. Cover text order, link-only prompt, deduplicated paths, 256 KiB text, Unicode boundaries, empty input, unsupported blocks, non-file URI references with zero fetch/context reads; malformed/credential-bearing URIs; file query/fragment/remote authority and encoded traversal/separators, native Windows drives and rejected UNC/device forms.
- [ ] Run go test -run 'Test(PromptNormalization|ResourceURIPlatformForms|ControlErrorProjection)$' -count=1. Implement normalization using URI/path operations only. Preserve valid non-file URI/name as bounded user text with no context_paths; convert only safe local file references. Do not read/open/fetch resources. Core remains responsible for canonical containment and file content limits.
- [ ] Implement the exact allowlist and used-field Control DTOs, with source-referenced fixtures from internal/rpc/control.go. Use errors.As with RPCErrorCode; never parse error strings. Construct acp.NewRPCError with fixed message/stable reason and no raw cause or stack. Unknown failures map to -32603.
- [ ] Check unsupported BaseAgent methods cannot grant capabilities or return unsafe errors. Until Tasks 4-6 implement Prompt/SessionCancel, their unsupported behavior must be explicit; this intermediate module is not a release.
- [ ] Run go test -race ./... -count=1 in plugins/acp, then just ci from root; record evidence and commit explicit module paths with: feat(acp): add bounded stdio module and session admission.

## Task 4: Reserve, start, subscribe and finalize exactly once

- [ ] Write TestPromptAdmissionAndCancelBeforeRunID, TestSessionCancellationIsolation, TestIdleCancelDoesNotAffectNextPrompt and TestTerminalCancelLinearization. Use a fake Host with barriers, recorded Control calls and ordered output observations.
- [ ] Run go test -run 'Test(PromptAdmission|SessionCancellation|IdleCancel|TerminalCancel)' -count=1 in plugins/acp; observe the missing Prompt lifecycle failure.
- [ ] Implement full-prompt validation before capacity reservation, then atomically reserve both limits. Send turn/start with real session_id, face=code, normalized text/context_paths. On ambiguous admission, drain the connection and private Runtime; never resubmit.
- [ ] Register the run event route before run/subscribe {run_id, after_seq:0}; create the sanitizer once the run ID exists. Buffer early matching callbacks until the response binds subscription_id; reject a mismatch. A latched cancel is applied as soon as the run ID is known.
- [ ] Keep locks off all RPC/write/user waits. Capture the generation in asynchronous completions; release the slot only after one final response decision and idempotent unsubscribe/cleanup.
- [ ] Pin precedence: unwritable transport emits no response; accepted client cancel wins; otherwise integrity failure is an error; otherwise durable terminal chooses end_turn, -32800 or failure. A cancel after response commitment is a no-op.

## Task 5: Reduce ordered committed events and bound memory

- [ ] Write TestProjectionOrderingReplayAndIntegrity. Feed seq 3,1,2,duplicate 2 plus an unprojected event; assert one contiguous projection, duplicate suppression and cursor advancement. Also cover immediate terminal before subscribe returns, wrong run/subscription, malformed required payload, gap expiry, ingress+reorder overflow and run/stream_error.
- [ ] Run go test -run TestProjectionOrderingReplayAndIntegrity -count=1; then implement one reducer loop per prompt and a shared bounded ingress budget. Callback delivery only enqueues; it never waits for user input.
- [ ] Make quota checks account for retained raw bytes and event count across both queues, not 256+256 separately. On gap/integrity/overflow, cancel only the affected run unless the transport itself is fatal.
- [ ] Map tool lifecycle to a single opaque ID, ensuring tool_call exists before updates. Content arrays replace previous arrays. Do not copy args/raw results/reasoning/provider metadata/child state; unprojected events still advance sequence.

## Task 6: Sanitize display units and verify original model bytes

- [ ] Write TestProjectionTextBoundary with split secret/root chunks, newline-free tail, Unicode, 64 KiB and 64 KiB+1 partial lines, consecutive model requests, digest mismatch and output JSON expansion.
- [ ] Run go test -run TestProjectionTextBoundary -count=1; implement this reducer algorithm:

~~~text
model.request: reset current model hash/length/partial-line state
model.delta: hash/count original UTF-8 bytes, then accumulate complete lines
complete line: sanitize the whole line; emit safe text split at rune boundaries
partial line > 64 KiB: emit one omission marker; suppress to newline/completion
model.completed v2: compare original byte_len and SHA-256, then sanitize/flush tail
digest/version failure: fail prompt; never emit end_turn
~~~

- [ ] Measure encoded notification size including JSON escaping/envelope overhead before sending; split safe text until every frame is <=256 KiB. Do not split arbitrary JSON or truncate an essential approval target. Output order follows the source event releasing the safe unit; partial text may wait for model completion.
- [ ] Await all prior SessionUpdate writes before committing Prompt's response. A write timeout is connection-fatal; never send a substitute success.
- [ ] Run go test -race -run 'Test(PromptAdmission|SessionCancellation|IdleCancel|TerminalCancel|Projection)' -count=1; rerun the inherited SDK admission-race fixture after dependency changes. Then run just ci, record evidence and commit explicit paths with: feat(acp): stream ordered prompts with safe cancellation.

**Acceptance:** Owned sessions, admission, prompt/cancel, original-byte integrity and ordered sanitized output have focused evidence. Non-file resource references create no adapter fetch; local context uses the durable session root. Interaction events remain explicitly unavailable until ACP-04; this intermediate module is not a released pilot.

**Handoff:** ACP-04 receives promptScope, interactionJob, stable tool IDs, live Control cleanup access and bounded transport/reducer state. Preserve generation correlation and keep all durable decisions in Runtime.
