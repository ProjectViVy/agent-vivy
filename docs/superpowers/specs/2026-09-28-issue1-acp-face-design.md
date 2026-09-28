# Issue #1: Local ACP v1 Face architecture

Status: **design reviewed by owner; G0 not accepted; G1 not scheduled**. Baseline:
`main` at `3c4ed66` (2026-09-28). Product authority: [Issue #1](https://github.com/ProjectViVy/agent-vivy/issues/1),
the v1 Module/Port/Assembly contracts, and the existing Control RPC. This
document details that issue without changing its UNSCHEDULED status. It is the
design input to its G0 contract freeze, not an implementation authorization.

## Outcome and scope

| ID | Required observable outcome |
| --- | --- |
| A1 | A selected local ACP v1 Face is launched by an IDE over stdio, and one real client completes a prompt. |
| A2 | Each ACP session corresponds to one real Vivy session; prompts, events, cancellation, approval, and Ask User use the existing Service.Run, Journal, Policy, and HITL path. |
| A3 | Only committed Journal events are projected to ACP; stdout stays parseable ACP NDJSON for the entire process lifetime. |
| A4 | Recipe selection and Inspect prove `projectvivy/acp` is the sole Face and identify its source/hash/Grant; omission physically excludes ACP, and its selection excludes Web/TUI Face code. |
| A5 | Unsupported client features, untrusted input, expiration, disconnect, and output failure fail closed without expanding the local tool authority. |

This release is local stdio, one Face per Generation, one client connection per
process, potentially several sessions on that connection, one active prompt per
session. No cross-launch `session/load`, remote listener, client filesystem or
terminal delegation, dynamic Module loading, new Host or Port, alternate loop,
or A2A. ACP is a transport-facing human Face; its compiled Module ID is
`projectvivy/acp`, T2, providing `std/face@v1` through `core/face-host@v1`
with effective `rpc.client` only. The Recipe uses the *actual v1 syntax*
`exclusive: {std/face@v1: projectvivy/acp}`, not a new `face: acp` key.

**Design decision.** Reuse the generated exclusive Face, FaceHost's in-process
authenticated Control RPC, and the existing private coding runtime. Add one
generic input stream, one pre-logging selected-Face startup path, and an ACP
adapter Module. A separate ControlHost duplicates authority; multiple Faces
require control and HITL arbitration; invoking a CLI child creates a second
runtime boundary. None serves the first release's required behavior.

## Existing facts and required seams

| Inspected source | Fact / consequence |
| --- | --- |
| `sdk/port/face/face.go` | `Options` has `Out` and `Err` but no `In` or prepared project-root field; add `In io.Reader` and `ProjectRoot string`, preserving all existing invocations. The Port stays ACP-independent. The launcher supplies the canonical root, not client input. |
| `internal/app/facehost.go` | `RunFaceProviderWithAppOptions` composes an App without gateway/ears and `DialControl` exposes `Call` and `OnEvent` through a private JSONL pipe. No new RPC transport or direct runtime access is needed. |
| `cmd/vivy/main.go`, `cmd/vivy/run.go`, `cmd/vivy/tui.go` | The default path installs a stdout logger before selecting a generated Face; `vivy run` is a separate headless command. `tui.go` always imports `internal/codeface` and the TUI live/view packages even in a packed ACP Generation. Branch before stdout logging; replace the TUI CLI source in the ACP build overlay with an explicit unavailable-command stub, preserving the normal/TUI build. Preserve non-ACP `run`; reject `vivy run` explicitly in an ACP Generation before invoking its stdio provider. |
| `internal/codeface/launch.go` | `Prepare` isolates a private SQLite Journal, configures a local project world, and retains the shared settings document; its package imports `sdk/tui/face`. Move the reusable preparation/options behind a package with **no TUI import**, then let VIVY CODE consume it. Importing `codeface` from ACP would break physical omission proof. |
| `internal/domain/face.go`, `internal/runtime/prompt_state.go` | `FaceCode` is the existing per-Run coding semantics and selects the code prompt. The Generation's Face identity is `projectvivy/acp`; ACP turns explicitly set `face: code`. Do not create `FaceACP` merely to name the transport or silently default turns to `web`. |
| `internal/rpc/control.go` | `turn/start` returns a `run_id`; `run/subscribe` replays by `after_seq`; `run/event` carries committed events; `approval/respond`, `question/respond`, and `run/cancel` own mutations. `review/respond` can cancel a question. |
| `internal/domain/event.go`, `internal/runtime/payloads.go` | Named model/tool/gate/terminal events and review IDs already exist. ACP must project an allowlisted subset of their bounded payloads, not relay raw event JSON. |
| `docs/plans/plugin-platform/README.md` | PLG-P9 is recorded complete on main; this satisfies Issue #1's prerequisite, but does not schedule Issue #1. |
| `sdk/internal/frontend_v1.go` | Pack already overlays the generated Go Assembly and embedded manifest, but unconditionally builds, embeds, and stages `ui/dist` in every artifact. Add an ACP-selected overlay for `cmd/vivy/tui.go` so Go's import graph drops the built-in TUI implementation. The UI asset tree is still present under the current Generation packaging contract; its removal is not part of the Issue's Face-code omission criterion. Record this boundary in G0 and verify package inclusion and artifact contents separately. |

Proposed code ownership: `cmd/vivy` selects the compiled Face before logging;
an internal Face-instance preparation package owns the private project/runtime
config and shared settings option; `internal/app` remains the single FaceHost;
`plugins/acp` owns protocol types, connection-local session/run/review state,
allowlisted event projection, and stdio. The generated Assembly owns Module
selection. The packer owns the selected ACP CLI source overlay so the normal
TUI CLI cannot bring TUI Face code into the ACP executable. No adapter writes
Journal or changes Policy.

## Protocol and connection contract

Target ACP **stable v1**, kickoff schema `schema-v1.21.0`, local JSON-RPC
2.0 NDJSON stdio. ACP v2 and unstable methods are not advertised. The adapter
rejects incompatible `protocolVersion`; it records client capabilities at
`initialize` and advertises only capabilities actually implemented in this
Generation. The baseline prompt types are Text and ResourceLink. Do not
advertise image/audio/embedded context, `loadSession`, modes, client FS,
terminal, or URL elicitation in this cut. Accept text as user input. For a
ResourceLink, retain its URI/name in bounded **user** content without fetching
it or treating it as instructions; an in-project file URI may be converted to
an existing validated `context_paths` input only after canonical containment
checks. Reject invalid or oversized links before starting a Run. Never fetch
arbitrary URIs. Nonempty `mcpServers` and `additionalDirectories` at
`session/new` are rejected explicitly, rather than being silently ignored.

`session/new.cwd` must designate the single canonical project root selected
at process launch; compare physical directory identity after resolving links.
No client request may alter the process cwd, selected world, credentials, or
shared settings. Return the Vivy `session/create` ID directly. The adapter
accepts only IDs created in this ACP connection, even though Control RPC can
address other sessions in the private process. It keeps an in-memory map of
owned sessions, their active Run IDs, subscription IDs, last delivered seq,
and outstanding review IDs; Journal and existing review stores remain the
durable authorities. No cross-launch recovery is promised.

| ACP direction / operation | Host operation and rule |
| --- | --- |
| Client `initialize` | Intersect static adapter support with negotiated client support; return only truthful capabilities. No Run. |
| Client `session/new` | Validate cwd/MCP/directory input, then `session/create` with the prepared `workspace_path`; return its real ID and mark it connection-owned. |
| Client `session/prompt` | Reject unknown session or overlapping prompt; validate content; `turn/start` with `session_id`, `text`, and `face: code`; record real `run_id`; `run/subscribe` from seq 0; wait for committed terminal event before replying. |
| Agent `session/update` | Convert sequenced, committed `run/event` for the active run to ACP text chunks and tool call/update variants. Emit in seq order, deduplicate by `(run_id, seq)`. Unknown event types are not sent as raw JSON. |
| Client `session/cancel` (notification) | Look up *this session's* active Run, call `run/cancel`, then finish the pending prompt with `stopReason: cancelled` once terminal is durably observed; it cannot cancel another session. |
| Agent `session/request_permission` | On committed `tool.approval_required`, send one reverse request bound to `(session_id, run_id, approval_id, tool_call_id)`. Offer only allow-once and deny options that map to current Policy. A selected option calls `approval/respond`; ignore late responses rejected by Vivy. |
| Agent `elicitation/create` | On committed `user.question_required`, request a form with one bounded `answer` string, only if client advertised `elicitation.form`. An accepted answer calls `question/respond`; decline/cancel/error cancels the question or Run via an existing authoritative route. Never send URL mode. |

For approval UI failure, unsupported form UI, timeout, or transport failure,
do not grant approval or synthesize an answer. Cancel the owning Run through
Control RPC using a cleanup context while available; let durable expiration
remain authoritative if cancellation itself fails. Once client cancel or
expiration has won, late reverse responses are ignored. Reverse requests
execute independently of the ACP read loop so a client may send `session/cancel`
while a permission/form request is outstanding; the same bounded connection
writer serializes notifications, reverse requests, and responses.
Cancellation is best-effort with respect to a tool effect already authorized
and started by Vivy; it is not a transaction that rolls that effect back.

After `run.completed`, finish the prompt with `end_turn`; after
`run.cancelled`, finish with `cancelled` (required when the client cancelled);
after `run.failed`, return a sanitized JSON-RPC failure on the outstanding
prompt, preserving the durable failed outcome, with no fabricated success
stop reason. A pre-Run validation/Host failure returns a sanitized JSON-RPC
error immediately. A `run/stream_error`, sequence gap that cannot be
replayed, or loss of the output stream is an adapter failure: attempt to
cancel the owned Run and never report success. Translate expected validation,
not found, unsupported, and busy errors to stable client-visible codes;
internal diagnostic strings and raw Journal payloads stay off the wire.

| Error condition | ACP wire result |
| --- | --- |
| Wrong protocol version, invalid cwd/prompt, unknown or unowned session | JSON-RPC `-32602` with a fixed, non-sensitive message. |
| Optional operation deliberately unimplemented, including `session/load` | JSON-RPC `-32601`; corresponding capability absent in `initialize`. |
| Overlapping prompt or bounded-capacity exhaustion | JSON-RPC `-32001` (server busy); no second Run is created. |
| Vivy Run failure after acceptance or adapter/internal failure | JSON-RPC `-32603` with a sanitized category; preserve failure in Vivy's Journal. |
| Client `session/cancel` after Run acceptance | Complete the pending prompt with `stopReason: cancelled`, following committed terminal state. |

An already-completed prompt is not retroactively cancelled. If a cancel
notification races with terminal commit, the persisted Vivy terminal state
settles the result; tests must exercise both orderings.

Event projection needs an explicit G0 field allowlist: `model.delta` ->
`agent_message_chunk`; `tool.requested` -> `tool_call`; `tool.started` /
`tool.finished` -> `tool_call_update`; approval and question required events
drive reverse requests; `run.completed` / `run.failed` / `run.cancelled` end
the pending prompt. Reasoning, arguments, previews, and tool results are
separately bounded and redacted before inclusion. Never present a raw
`model.reasoning_delta`, approval payload, secret, internal instance directory,
or private Journal field. ACP requires absolute paths where a protocol path
is used: a client-supplied workspace path already known to that client may
be returned when that particular ACP field requires it; internal instance,
credential, and unrequested host paths must never be projected.

## Lifecycle, limits, and failure model

1. Detect a selected ACP Face from the generated Assembly before normal
   stdout logging; bootstrap and file logging write only to stderr/file.
   Prepare one private instance for the process, set the canonical project
   root, open App/FaceHost, and start the ACP stdio connection using
   `Options{In: os.Stdin, Out: os.Stdout, Err: os.Stderr,
   ProjectRoot: prepared.Config.Runtime.WorkspaceRoot}`.
2. One ACP connection may own multiple sessions; each session has at most
   one active prompt, with as many pending review requests as the existing
   Run actually emits, up to a bounded connection limit. Do not serialize
   the entire connection on a long prompt or reverse request. Each session's
   events maintain their own monotonic sequence and stable tool call IDs.
3. Register `Host.OnEvent` before starting/subscribing to a Run. After
   `turn/start`, subscribe from seq 0; replay covers events committed before
   the subscription. Deliver an update only after its committed seq, and
   finish the prompt only after all prior projected updates are written.
4. On EOF, broken pipe, signal, or write failure, stop accepting prompts,
   cancel all connection-owned active Runs with a bounded cleanup context,
   close outstanding ACP calls/subscriptions, then close App. Do not delete
   the private Journal as part of cleanup. Durable Run cancellation may fail;
   record that fact in stderr/file and report incomplete cleanup honestly.
5. Bound inbound frame size, content length, number of sessions, pending
   reviews, outgoing queue, reverse-request lifetime, and shutdown time. The
   candidate stdio transport defaults to a 10 MiB inbound frame and a 30 s
   write timeout; FaceHost's in-process RPC has a 32 MiB default frame and a
   64-frame outgoing queue. G0 decides whether these defaults are suitable
   and freezes the remaining limits with tests for exceeding them. A slow
   client must cause backpressure and then
   connection termination/cancellation, never unbounded buffering or silent
   event loss. There is no new polling or persistence cache in the hot path.

The child process has its parent's local OS authority. `rpc.client` is a T2
governance grant, not an OS sandbox. Treat all client prompt/URI/review input
as untrusted; keep canonical path checks and ToolHost/Policy approval in
Vivy. No network listener or ACP-controlled subprocess is added. Logs must
not reproduce ACP frames or credentials; the pinned SDK's default Debug
payload logging requires a filtered logger before first protocol traffic.

## Dependency compatibility: G0 investigation, not yet a pass

The pinned candidate `github.com/eino-contrib/acp@v0.0.4` exposes protocol
types, `conn.NewAgentConnectionFromTransport`, and `transport/stdio.NewTransport`.
Its generated Go client interface names the method
`UnstableCreateElicitation`, but its **wire method is `elicitation/create`**.
It also has form capability and request/response shapes resembling the
stable schema. The old Go name is not by itself a wire incompatibility.
Static inspection has **not** established full structural compatibility with
the official stable `schema-v1.21.0` nor validated its outbound frame order,
backpressure, disconnect propagation, or form response with a real client.
Do not mark the dependency accepted on the basis of this inspection.

G0 must record a field-by-field stable-subset comparison and an executable
wire fixture for initialize, new, prompt, updates, cancel, permission, and
form elicitation, including missing client capabilities. Also measure whether
the SDK's root/conn/stdio imports or logger pull prohibited transports or
unredacted payloads into the selected executable. If it cannot produce the
stable subset without an incompatible patch, stop G0 for a revised dependency
decision; do not invent a parallel ACP schema or leak Eino types into the Port.

Upstream primary references: [ACP v1 overview](https://agentclientprotocol.com/protocol/v1/overview),
[prompt turn](https://agentclientprotocol.com/protocol/v1/prompt-turn),
[elicitation](https://agentclientprotocol.com/protocol/v1/elicitation),
[schema release](https://github.com/agentclientprotocol/agent-client-protocol/tree/schema-v1.21.0/schema/v1),
[candidate SDK at v0.0.4](https://github.com/eino-contrib/acp/tree/v0.0.4).

## Verification and delivery boundary

G0 changes the existing normative documents, especially
`ACP-REMOTE-CONTROL-PROPOSAL.md` (local ACP Face vs deferred remote control),
`VIVY-FACE-PACK.md` (historical non-Face ACP wording), and `docs/TODO.md`
(old WONT-DO). Freeze the exact projection and error tables, stop reasons,
limits, dependency result, packaging boundary, and threat model there or in
one linked normative contract; do not create conflicting sources of truth.
The implementation plans may be prepared now but remain blocked until G0 is
reviewed and G1 is scheduled by the Issue owner.

G1 acceptance follows A1-A5: a real IDE/client subprocess prompt; ordered
tool-bearing committed updates; approval approve/deny/unsupported; Ask User
answer/cancel/expiry; cancellation isolated to the addressed session;
stdout NDJSON throughout startup/shutdown; unchanged gateway/TUI/headless;
selected and omitted Recipe pack/Inspect with physical dependency evidence;
focused adapter/FaceHost tests, `just ci`, and the repository iteration log.
As the change touches Module/Port/Host/Compiler, implementation uses the
repository `vivy-plugin` and `vivy-kernel-ci` workflows. No G1 code or
implementation completion is claimed by this design.
