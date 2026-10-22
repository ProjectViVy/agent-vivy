# D14 runtime identity residue cleanup

This isolated delivery removes five remaining active custom digest uses and
repairs the current shell preview replay recognizer. It starts at
`a12748a7114bb54fd0bdf663bfb1f01fcec90802`.

- Steering compares the exact delivered text stored on the existing Eino
  message marker. Repeated delivery stays idempotent; changed text is inserted
  before the pending assistant tool-call tail.
- The resolving provider cache compares a private construction tuple, including
  the exact credential. It retains current readiness and adapter admission
  checks. The tuple is neither serialized nor added to diagnostics.
- A selected workspace uses `selected:<sessionID>` from its authoritative
  durable Run/session owner. Its canonical filesystem root remains private in
  `Workspace.Path`; confinement and release checks remain authoritative.
- The TUI assembly cache compares an owned message snapshot, including nested
  tool cards and attachment/file metadata. A local rebuild counter replaces the
  content-derived stamp used by the existing paused viewport anchor.
- MCP command names reversibly encode the existing server/prompt tuple in
  lowercase hex. This preserves case-sensitive identity through case-insensitive
  command lookup. The existing 256-byte component limits bound the name to 1029
  bytes. RPC permits this length only after decoding the existing prompt ID and
  matching its exact derived name; ordinary skill names retain their 96-byte
  limit. No alias registry or second identity owner is introduced.
- Shell message projection recognizes the currently emitted
  `bash script [redacted bytes=N]` preview without restoring any hash generator.

## Ownership and Eino capability check

The simplest viable change is direct equality at each existing owner, plus a
local TUI rebuild version. A separate identity registry, hash replacement or
second runtime would add ownership without satisfying the deletion contract.
The retained `Service.Run`, durable queue, ToolHost and Eino execution paths
remain the only execution owners.

Inspected repository-pinned Eino `v0.9.21`:
`adk.TypedChatModelAgentMiddleware.BeforeModelRewriteState` and
`TypedChatModelAgentState[*schema.AgenticMessage]` in `adk/handler.go`,
`adk.WithHistoryModifier` in `adk/chatmodel.go`, and the existing
`schema.AgenticMessage.Extra` marker. The typed middleware hook persists its
returned state and is the existing agentic steering seam. This change reuses
that hook; it adds no orchestration primitive or substitute loop.

Inspected pinned EinoExt `agenticopenai v0.2.4` `ChatConfig.APIKey` and
`NewChatModel` in `chat_model.go`. Provider construction already owns the
credential. Cache selection is private Vivy construction state, not an Eino
transport capability. Exact tuple comparison replaces only its digest key.

Selected workspace ownership was checked against
`internal/storage/sqlite/sessions.go` (workspace update requires no Runs),
`internal/storage/postgres/sessions.go` (shared session-row lock and Run check),
and `Service.SetSessionWorkspace` (same first-Run admission lock). No inspected
consumer required different sessions selecting the same path to share a public
workspace ID; filesystem authority continues to use the actual selected Path.

## Boundaries

No persistent channel/observer/memory namespace, migration, finite converter,
production Journal, release package, dependency pin, or Studio code is changed.
Historical stored workspace IDs are not rewritten by this runtime patch. New
selected IDs identify session ownership rather than equality of host paths;
the integrated D14 conversion/acceptance owner must retain historical records
and verify its supported copied-store transition. This delivery does not claim
D14 completion or product/release acceptance.
