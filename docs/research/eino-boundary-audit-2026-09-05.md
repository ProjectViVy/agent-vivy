# Audit of the boundary between Vivy in-house work and native Eino capabilities

- Date: 2026-09-05
- Nature: current source-code fact record; not implementation authorization
- Baseline: `github.com/cloudwego/eino v0.9.13`, `github.com/cloudwego/eino-ext/components/tool/mcp v0.0.9`,
  `github.com/mark3labs/mcp-go v1.0.0`, using `go.mod` and the local module cache as the source of truth
- Decision order: Vivy architectural consistency first, native Eino/EinoExt reuse second, in-house exceptions third

## 1. Conclusion

The impact is a localized refactor, not a rewrite of the Vivy runtime.

Vivy's main loop already uses native Eino `adk.ChatModelAgent`, `adk.Runner`,
checkpoint/resume, schema/stream, and the skill, agentsmd, reduction, and
summarization middleware. Most of the current in-house code carries Vivy's
product-authority semantics and cannot be directly replaced by Eino: Journal,
Policy, HITL, budget, sessions, recovery, RPC, sandbox, worker-process
isolation, and product-event projection.

Candidate surfaces for reevaluation or cleanup amount to approximately **1.3–1.6k
lines of production code plus tests/assembly**, concentrated mainly in dynamic
tool discovery, Sequential Thinking, and the plantask compatibility surface that
has not entered production middleware. The MCP adapter converged on an
Eino-native approach on 2026-09-06; the streaming-observation wrapper was also
judged worth retaining and is no longer included in this upper bound. This number
is an audit upper bound, not the number of lines that can be deleted directly.

## 2. Current code size and accounting basis

Current non-test Go code is approximately:

| Area | Production code | Audit interpretation |
|---|---:|---|
| `internal/runtime` | 17.1k lines | Contains both Eino wiring and Vivy product governance; it cannot be treated wholesale as an "in-house loop" |
| `internal/provider` | 0.8k lines | EinoExt model construction plus Vivy provider catalog/secret/model metadata |
| `internal/tools` | 4.0k lines | Vivy's stable tool contract; handed to Eino through adapters |
| `internal/domain` | 1.1k lines | Product types outside the Eino isolation wall |
| `internal/storage` | 8.6k lines | Journal and product persistence, outside Eino's responsibilities |

Therefore, the total line count of `internal/runtime` cannot be used to measure the size of "duplicated Eino".

## 3. Native Eino capabilities already used correctly

| Capability | Current wiring | Disposition |
|---|---|---|
| Main ReAct loop | `engine.go`: `adk.NewChatModelAgent` | Native, retain |
| Execution and recovery | `adk.NewRunner` + `ResumeWithParams` | Native, retain |
| Checkpoint interface | `EinoCheckpointAdapter` | Thin adapter, retain |
| OpenAI / Claude | EinoExt model components | Native, retain |
| Skills | `middlewares/skill.NewMiddleware` | Native middleware + Vivy Backend, retain |
| AGENTS.md | `middlewares/agentsmd` | Native middleware + workspace Backend, retain |
| Context compaction | `reduction` + `summarization` | Native middleware + Vivy event/budget bridge, retain |
| Messages, tools, streams | Eino schema/components interfaces | Native protocols + Vivy adapters, retain |

## 4. Implementations Vivy must own

### 4.1 Journal and product events

Eino `AgentEvent` is a runtime event, not Vivy's durable product contract. Vivy
must own persist-before-fanout, run terminal state, message projection, restart
recovery, deletion fences, event versions, token/cost, and RPC compatibility. The
main locations are `service.go`, `mapper.go`, `message_projector.go`,
`internal/domain/event.go`, and the storage implementation.

Disposition: **normal in-house work; do not outsource to Eino.**

### 4.2 Policy, HITL, and tool governance

`tooladapter.go` performs parameter safety validation, Policy profiles, Plan Mode,
approval interruption/recovery, hooks, proposal/precondition checks, redaction,
output limits, and tool-mount auditing outside the Eino Tool interface. Eino
provides Tool and interrupt primitives, but does not own Vivy's governance contract.

Disposition: **normal in-house work; Eino primitives may be reused, but the governance gate cannot be removed.**

### 4.3 Checkpoint durability wrapper

Eino provides `CheckPointStore`; Vivy's `VersionedCheckpointStore` adds engine
version, checksum, BlobStore, and fail-closed handling for incompatibility.
`EinoCheckpointAdapter` is responsible only for interface conversion.

Disposition: **a correct thin adapter, not duplicated checkpoint functionality.**

### 4.4 Journal → Eino context projection

`context.go` projects durable messages, tool calls/results, images, and project
files into Eino messages, and enforces the history window and product budget.
`pairToolTurns` discards incomplete call pairs inside a truncated window; Eino
`patchtoolcalls` instead adds placeholder results. Their semantics differ, so one
cannot mechanically replace the other.

Disposition: **normal in-house work; a prompt template can reduce formatting code, but cannot replace projection semantics.**

### 4.5 Workspace, files, and encoding governance

`EinoFilesystemBackend` serves both the Eino Backend interface and Vivy typed
operations. Workspace containment, symlink escape prevention, sandboxing, atomic
writes, diff, stale-read, file versioning, LSP diagnostics, images, and bounded
search are all Vivy product/security semantics.

Disposition: **normal in-house Backend; Eino tool-registration components may be reused, but the security authority remains Vivy.**

### 4.6 Same-binary child worker

The small model–tool loop in `internal/worker` has surface overlap with Eino
`NewAgentTool`/prebuilt agents, but it implements an independent process failure
domain: the parent process brokers the model, tools, approvals, budget, and
PolicySnapshot; the child owns no Journal and cannot relax the parent's
permissions. Eino prebuilt agents are in-process orchestration, so direct
replacement would create a parallel governance track.

Disposition: **normal in-house work after architectural tradeoffs; not currently in the deletion scope.** If a lightweight in-process child is approved in the future, `NewAgentTool` can be an optional backend without replacing the existing worker.

### 4.7 RPC, Faces, Channels, Plugins, and UI

These are Vivy product surfaces and host boundaries, outside Eino orchestration responsibilities.

Disposition: **normal in-house work.**

## 5. Overlaps or conceptual debt requiring review

### 5.1 MCP transport — MCP slice complete as of 2026-09-06

`internal/runtime/mcp_backend.go` no longer owns a JSON-RPC/HTTP/SSE/session/request-id
protocol stack. The official `client.NewStreamableHttpClient` provides Streamable
HTTP, modern `server/discover` (preferring `mcp.LATEST_PROTOCOL_VERSION` and
falling back to the legacy initialize flow according to the official
implementation), typed tools/resources/prompts, and session lifecycle; tool
discovery and schema conversion are handled by `GetTools` from
`github.com/cloudwego/eino-ext/components/tool/mcp v0.0.9`.
The Eino MCP component is Apache-2.0 licensed; mcp-go is MIT licensed.

This is an intentional thin boundary: the Eino component currently covers only
tools and converts `CallToolResult.IsError` to a Go error; Vivy must retain
`isError`, resources, prompts, and typed lifecycle semantics, so those paths still
use the same mcp-go client. Eino tools are used only for catalog/schema
projection and are never mounted directly on the model; actual calls still use
the `mcp_list_tools`/`mcp_call` and `PrepareMCPCall` approval paths. Vivy continues
to own configuration hot reload, provenance, untrusted/fail-closed projection,
the 8s operation timeout, the 512KiB raw-response guard, the 256KiB
content/catalog budget, 32-page and duplicate-cursor bounds, browser-use
filtering, and client close on removal/replacement/application shutdown.

The explicit gap and future removal boundary is as follows: once the Eino MCP
component covers resources/prompts/lifecycle while retaining `IsError` semantics,
the corresponding mcp-go typed plumbing can be removed from this adapter; before
then, an Eino tool cannot directly replace the Vivy domain contract. stdio, OAuth,
and continuous listening have not been introduced.

Impact: **moderate and localized; Journal, Policy, HITL, RPC/TUI, and the governance architecture are unchanged.**

### 5.2 In-house `tool_search` and visible-surface middleware

`internal/tools/toolsearch.go`, `toolselection_middleware.go`, and
`toolselection.go` total approximately 260 lines. Eino v0.9.13 already has
`middlewares/dynamictool/toolsearch`, which likewise handles initial hiding,
search, and subsequent exposure of dynamic tools.

Recommended boundary: let Eino manage model visibility; the Vivy adapter continues
to enforce the allowlist, Skill mount, Policy, and a second validation at actual
call time. First compare behavior; do not merely delete the execution gate.

Impact: **small to moderate.**

### 5.3 Sequential Thinking

`SequentialThinkingBackend` (the misleading `Eino` prefix was removed on
2026-09-06) has no Eino import; it is an approximately 160-line in-memory state
tool that is lost on restart. The capability check / replacement with EinoExt
Sequential Thinking remains to be done.

Impact: **small.**

### 5.4 Todo / plantask: "compatibility does not mean adoption"

`EinoTodoBackend` implements `plantask.Backend`, but production tools use Vivy's
own `task_create/get/update/list` typed operations; the current production
assembly does not call `plantask.New(...)`. Existing research described
"implementing a compatible interface" as "using native plantask," which is
inaccurate.

Do not attach the plantask middleware merely for the sake of using a native
component: if middleware-injected write tools do not pass through Vivy's
`toolAdapter`, they bypass Policy/HITL. Candidate actions are to delete the
unused compatibility half, or to prove that the native tools can pass through
the governance gate in full before integrating them.

Impact: **small, mainly conceptual cleanup and removal of dead compatibility code.**

### 5.5 stream observer and Eino callbacks — KEEP (2026-09-06)

`model_stream_observer.go` wraps ChatModel and uses
`schema.Pipe[*schema.Message](8)` to tee raw chunks on the producer's
`Recv → persist → Send` path. This is the seam for Journal real-time delta,
reasoning/text ordering, budget, backpressure, fail-closed behavior, and
interrupt/resume; it is not a deletable observation layer.

Review of pinned Eino v0.9.13: `callbacks.OnEndWithStreamOutput`,
`schema.StreamReader.Copy` / `copyStreamReaders`, and
`compose.genericOnEndWithStreamOutput`. The callback performs a sibling `Copy`
after inner `Stream()` returns; the handler may run synchronously, but it is still
not on the producer path and cannot:

1. Put persist in `Recv → persist → Send` (persist on the independent copy is not
   on the graph Recv path)
2. Propagate persist backpressure to provider `Recv` through a bounded `Pipe(8)`
3. Fail-close persist errors into the graph stream (failure on the independent
   copy does not fail downstream)
4. Execute `waitForToolsSettled` on the producer path

This wrapper's `Begin` must still be called before handing the tee to Eino, to
avoid a pump/eager-forwarding race; that is an implementation constraint of this
wrapper, not evidence that a callback cannot perform Begin.

Therefore, **retain the wrapper**. Vivy does not currently register an
`eino/callbacks` handler. Revisit this if Eino later provides a producer `Recv`
hook with fail-closed behavior.

Impact: **closed; not a deletable surface.** Record:
`docs/logs/2026-09-06-eino-boundary-stream-observer-naming/`.

### 5.6 `Eino*Backend` naming debt — DONE (2026-09-06)

Names that are accurate and do consume Eino interfaces—**retain the `Eino` prefix**:
`EinoCheckpointAdapter`, `EinoFilesystemBackend`, and `EinoSkillBackend`;
`EinoTodoBackend` implements `plantask.Backend`, but production does not attach the middleware (see §5.4).

On 2026-09-06, the misleading prefix was removed from items that only implement Vivy `tools.*Operations`:

| Former name | Current name | Notes |
|---|---|---|
| `EinoCommandBackend` | `CommandBackend` | Vivy sandbox/policy adapter |
| `EinoHTTPBackend` | `HTTPBackend` | Vivy SSRF/policy adapter |
| `EinoMCPBackend` | `MCPBackend` | Vivy governance adapter; Eino `GetTools` is used only for tools/schema |
| `EinoSequentialThinkingBackend` | `SequentialThinkingBackend` | EinoExt component not yet adopted |
| `EinoWebFetchBackend` | `WebFetchBackend` | Vivy bounded fetch adapter |
| `EinoDownloadBackend` | `DownloadBackend` | Vivy workspace/sandbox adapter |

The name `MCPBackend` no longer implies that Eino types leak into the product
boundary; its Eino-native tools/schema projection and mcp-go typed lifecycle
boundary are described in §5.1. The remaining implementation is still Vivy's own
governance shell (SSRF, sandbox, approval, etc.), and the name is no longer
evidence that Eino has been wired in.

## 6. Corrections to existing research

The overall judgment in the existing `eino-reuse-inventory-2026-08-31.md` remains
valid: Vivy should own governance, Journal, Policy, and worker, while about 60%
of candidate capabilities can be reduced through Eino/upstream reuse.

Four points need correction:

1. "Having an equivalent capability" does not mean "currently used": in-house
   `tool_search` remains the production implementation.
2. "Implementing an Eino Backend" does not mean "the middleware is wired":
   plantask is not currently registered as production middleware.
3. `MCPBackend` (formerly `EinoMCPBackend`) is a governance adapter inside the
   runtime: Eino `GetTools` handles tools/schema, while the mcp-go typed client
   handles resources/prompts/lifecycle and Vivy's `isError` semantics; it is not a
   second protocol stack.
4. License accounting: the Eino MCP component is Apache-2.0 and mcp-go is MIT;
   neither should be recorded as having an unknown license or as using a mixed
   license.

## 7. Impact-scope assessment

- Approximately **85%** of runtime/product code: necessary in-house work, retain.
- Approximately **5–8%**: clearly worth migrating, reducing, or cleaning up,
  concentrated in tool_search, Sequential Thinking replacement, and the plantask
  compatibility surface; MCP is now a thin adapter with a defined removal boundary.
- The stream observer/callbacks previously listed as **2–3% PoC**: the wrapper was
  judged worth retaining on 2026-09-06 and is no longer a reduction candidate.
- Although the child worker contains a substantial amount of code, it is a product
  architecture choice and is not included in the immediate deletion scope.

These percentages indicate audit scale, not a precise deletion commitment.

## 8. Follow-up discipline

This record does not authorize implementation. When executing items individually, you must:

1. First verify the actual API and behavior of pinned Eino/EinoExt;
2. State Vivy invariants and whether the native component can satisfy them;
3. Prefer replacing protocol/orchestration mechanics while retaining the Vivy governance shell;
4. Maintain the domain firewall; Eino imports must not cross beyond runtime/provider;
5. Test and deliver each item independently; do not rewrite the runtime wholesale.

## 9. 2026-09-06 superseded note

The implementation decision for the tool-search candidate is now closed on
the dedicated `refactor/eino-toolsearch` worktree. The pinned Eino v0.9.13
core `adk/middlewares/dynamictool/toolsearch` middleware replaces the custom
search and visibility surface. Vivy retains the fixed allowlist, governed
business-tool adapters, policy/HITL/audit checks, and the final hidden
Skill-mount projection; config/settings normalize the retired legacy name at
their input/write boundary. The MCP, observer, naming, and tool-search slices
are independently evidenced; Sequential Thinking and plantask remain open.
