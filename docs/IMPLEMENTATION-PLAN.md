# AGENT-VIVY V0 — Implementation Plan

> Status: authoritative translation of the design dossier into an executable
> architecture. Implementation code lives in this repo; the contracts below
> are what that code must honor.
> Source dossier: `../` (`diva-go/`) — PRD v0.5, Assembly Options, Reference
> Index, Go/No-Go Preflight.
> Updated: 2026-08-07

---

## 1. Verified technical facts

These were checked against real sources before this plan was written; they
override any assumption in the dossier.

| Fact | Evidence |
|---|---|
| Eino ADK lives in the main module `github.com/cloudwego/eino/adk` | vendored ref `../.workspace/eino/go.mod`; module path confirmed |
| ADK exposes `Runner` / `TypedRunner`, `NewRunner`, `Query`, `Run`, `Resume`, `ResumeWithParams` | `.workspace/eino/adk/runner.go` |
| `CheckPointStore` = `Get(ctx,id) ([]byte,bool,error)` + `Set(ctx,id,[]byte) error`; optional `CheckPointDeleter.Delete` | `.workspace/eino/internal/core/interrupt.go` |
| `ChatModelAgent` built via `NewChatModelAgent(ctx, *ChatModelAgentConfig)`; full ReAct loop for `*schema.Message` | `.workspace/eino/adk/chatmodel.go` |
| Interrupt / resume is first-class at Runner level (`InterruptInfo`, `ResumeParams.Targets`) | `.workspace/eino/adk/{runner.go,interrupt.go}` |
| OpenAI model component is online `github.com/cloudwego/eino-ext/components/model/openai` (v0.1.13 resolves via goproxy.cn) | `go list -m ...@latest` |
| There is **no** official Eino Anthropic model component (`.../components/model/anthropic` and `eino-contrib/anthropic` both 404) | `go list -m ...@latest` → "no matching versions"; git ls-remote → repo not found |
| `proxy.golang.org` is unreachable from this network; `https://goproxy.cn` works | live dial test |
| Go 1.26.4 installed at `C:\Program Files\Go\bin` (not on PATH) | `go version` |
| `modernc.org/sqlite` (pure Go, no CGO) resolves to v1.56.0 | `go list -m` |

Consequences:

- Eino is consumed **exclusively as online module dependencies**. The
  vendored `../.workspace/eino/` tree is a read-only API reference for
  verification (task A1) — it is never imported, vendored, or `replace`d.
- The Anthropic path is a **Vivy-owned thin adapter** over the Anthropic
  Messages API, hidden behind `ProviderRef`. It is not an Eino component.
- `GOPROXY` must be `https://goproxy.cn,direct` (already persisted via
  `go env -w`; closes SR-7 / P1-1).

## 2. Module and package layout

Single Go module `agent-vivy` (D-006). No multi-module workspace, no
imitation of Diva's 17-crate graph.

```text
agent-vivy/                      module agent-vivy
  cmd/vivy/                      process entrypoint (signals, health, wiring)
  internal/app/                  composition + lifecycle (start/stop order)
  internal/config/               typed config load/validate; secret boundary
  internal/domain/               Vivy-owned contract types (zero ext deps)
  internal/runtime/              Run service + Eino adapter (ONLY Eino door)
  internal/provider/             openai / anthropic; YAML bundles
  internal/tools/                ToolSpec registry + approval policy
  internal/storage/              Journal/Snapshot/Blob/Lease + SQLite backend
  internal/events/               SSE fan-out + after_seq replay cursor
  internal/httpapi/              UI-facing command/query/SSE API
  schemas/                       JSON Schema / OpenAPI public contract
  fixtures/                      provider / event / recovery fixtures
  ui/                            Vite browser shell (API-only)
  docs/                          this plan + TODO board
```

### 2.1 Dependency direction

```text
httpapi ─┐
events  ─┼─> runtime ──> provider ──> (Eino components)
app     ─┘      │              │
                └─> storage <──┘   (storage is engine-agnostic)
                        │
                    domain   (everyone speaks domain; domain imports nothing)
tools ─────────────> domain
```

Rules:

- `internal/domain` imports **nothing external** — no Eino, no SQLite, no
  net/http. It is the anti-leak firewall (D-007).
- Eino types may be imported **only** by `internal/runtime` and
  `internal/provider`. Everywhere else is a lint/grep violation.
- `internal/storage` never exposes SQLite-specific surfaces to domain
  (no raw tx types, partial indexes, `AUTOINCREMENT`, FTS, WAL knobs —
  D-027). It speaks the four Vivy contracts.
- `internal/httpapi` and `ui` speak only the JSON contract in `schemas/`.

### 2.2 Eino type-leak gate (D-007, RK-1)

CI grep gate (task D4 adds it to the pipeline):

```text
FAIL if any file outside internal/runtime/** and internal/provider/**
      imports  github.com/cloudwego/eino*
FAIL if any file imports or references  agent-diva-  (anti-clone, RK-1)
```

## 3. Vivy-owned domain contract

Defined in `internal/domain`. Go skeletons below are the target shape
(implemented in task B3), not final field-for-field code.

```go
package domain

type SessionID string
type RunID string
type EventSeq int64

type Session struct {
    ID        SessionID
    Title     string
    CreatedAt int64 // unix milli
}

type Role string // "user" | "assistant" | "tool"

type Message struct {
    ID        string
    SessionID SessionID
    RunID     RunID // empty for user-authored messages
    Role      Role
    CreatedAt int64
    // content is append-only; no silent mutation (FR-2)
}

type RunStatus string

const (
    RunAccepted   RunStatus = "accepted"
    RunQueued     RunStatus = "queued"
    RunActive     RunStatus = "active"
    RunCompleted  RunStatus = "completed"
    RunFailed     RunStatus = "failed"
    RunCancelled  RunStatus = "cancelled"
)

// Terminal returns true for completed/failed/cancelled.
func (s RunStatus) Terminal() bool {
    switch s {
    case RunCompleted, RunFailed, RunCancelled:
        return true
    }
    return false
}

type Run struct {
    ID        RunID
    SessionID SessionID
    Status    RunStatus
    CreatedAt int64
}

type EventType string

const (
    EventRunStarted          EventType = "run.started"
    EventModelDelta          EventType = "model.delta"
    EventModelCompleted      EventType = "model.completed"
    EventToolRequested       EventType = "tool.requested"
    EventToolApprovalRequired EventType = "tool.approval_required"
    EventToolStarted         EventType = "tool.started"
    EventToolFinished        EventType = "tool.finished"
    EventRunCompleted        EventType = "run.completed"
    EventRunFailed           EventType = "run.failed"
    EventRunCancelled        EventType = "run.cancelled"
)

type RunEvent struct {
    RunID          RunID
    Seq            EventSeq // monotonic per run
    Type           EventType
    CreatedAt      int64
    PayloadVersion int
    Payload        []byte // JSON, schema-validated (schemas/)
}

type ToolCall struct {
    ID    string
    RunID RunID
    Spec  ToolSpec
    Args  []byte
}

type Approval struct {
    ID         string
    RunID      RunID
    ToolCallID string
    Decision   string // "approved" | "denied"
    ExpiresAt  int64
}
```

### 3.1 Run state machine and the terminal invariant (D-008)

```text
accepted ──> queued ──> active ──> completed
   │                        ├──> failed
   └────────────────────────┴──> cancelled
```

- A run transitions to `accepted` the instant the API accepts it (FR-4).
- It moves to `active` only when Eino actually begins; to `queued` if another
  run holds the session lock.
- **Exactly one terminal event** (`run.completed` | `run.failed` |
  `run.cancelled`) per run. The event store rejects a second terminal write
  for the same `run_id` (D-008, AS-5, AS-6). This invariant is the backbone
  of restart recovery.

## 4. Provider boundary

### 4.1 `ProviderRef` interface

```go
package domain

import "context"

// ChatModel is the only shape the runtime needs. It is satisfied by Eino's
// model.ChatModel for real providers. Tests inject deterministic model
// doubles at the runtime seam.
// Defined as an interface here so domain never imports Eino.
type ChatModel interface {
    Stream(ctx context.Context, input []*Message) (Stream[*Message], error)
}

type Stream[T any] interface {
    Recv() (T, error) // io.EOF at end
}
```

`internal/runtime` adapts `domain.ChatModel` to Eino's `model.ChatModel`
when constructing the `ChatModelAgent`; `internal/provider` returns concrete
implementations. Call sites never change when swapping a deterministic test
double for a real provider (FR-3).

> Note: the exact adapter seam is finalized in task A1 once the online Eino
> version's `model.ChatModel` signature is confirmed against v0.9.13.

### 4.2 Provider catalog and YAML bundles (D-018, D-022..D-025)

- V0 ships **exactly two** provider bundles: `openai` (OpenAI-compatible) and
  `anthropic`. Diva's other 21 provider entries are **not** carried (D-023).
- Bundle schema adapts Diva's `providers.yaml` field vocabulary (D-024):
  `name, api_type, keywords, env_key, display_name, default_model,
  gateway_prefix, skip_prefixes, env_extras, is_gateway, is_local,
  detect_by_key_prefix, detect_by_base_keyword, default_api_base,
  strip_model_prefix, supports_prompt_caching, models, model_overrides`.
- Entries are **re-derived** into Vivy's owned schema with a `provenance`
  field citing the Diva source path and entry name (D-025). No verbatim copy
  of Diva YAML text; no Diva Rust source (D-005).
- `openai` bundle is backed by the online
  `eino-ext/components/model/openai` component.
- `anthropic` bundle is backed by a Vivy-owned thin adapter (no official Eino
  component exists). It implements `domain.ChatModel` over the Messages API.
- Secrets are read from env (`env_key`) at request time and **never**
  persisted (D-010). Bundles describe shape + defaults, not credentials.

## 5. Storage architecture (D-026..D-033)

### 5.1 The four Vivy-owned contracts

```go
package storage

// Journal is the durable, append-only, ordered event log. Product history
// lives here, never in engine checkpoint bytes (D-028).
type Journal interface {
    Append(ctx context.Context, commit Commit) (EventSeq, error)
    Replay(ctx context.Context, after EventSeq) (Iterator[Entry], error)
}

// SnapshotStore holds the latest consistent domain state per key.
type SnapshotStore interface {
    Get(ctx context.Context, key string) ([]byte, int64, error) // value,version
    Put(ctx context.Context, key string, value []byte, expectVersion int64) error
}

// BlobStore stores opaque blobs (e.g. Eino checkpoint bytes) by id.
type BlobStore interface {
    Get(ctx context.Context, id string) ([]byte, bool, error)
    Put(ctx context.Context, id string, data []byte) error
    Delete(ctx context.Context, id string) error
}

// LeaseStore serializes exclusive work (session run lock, recovery).
type LeaseStore interface {
    Acquire(ctx context.Context, key, owner string, ttl time.Duration) (bool, error)
    Release(ctx context.Context, key, owner string) error
}
```

SQLite is the **V0 reference backend** behind these contracts, not an
architectural invariant (D-026, D-031). The filesystem-journal backend is a
V1+ probe, out of scope here.

### 5.2 V0 SQLite table sketch

```sql
sessions(id TEXT PRIMARY KEY, title TEXT, created_at INTEGER);
messages(id TEXT PRIMARY KEY, session_id TEXT, run_id TEXT, role TEXT,
         created_at INTEGER, content BLOB,
         FOREIGN KEY(session_id) REFERENCES sessions(id));
runs(id TEXT PRIMARY KEY, session_id TEXT, status TEXT, created_at INTEGER);
run_events(run_id TEXT, seq INTEGER, type TEXT, created_at INTEGER,
           payload_version INTEGER, payload BLOB,
           PRIMARY KEY(run_id, seq));
approvals(id TEXT PRIMARY KEY, run_id TEXT, tool_call_id TEXT,
          decision TEXT, expires_at INTEGER);
checkpoints(id TEXT PRIMARY KEY, generation INTEGER, checksum TEXT,
            created_at INTEGER);          -- pointer row
checkpoint_generations(id TEXT, generation INTEGER, blob BLOB,
                       PRIMARY KEY(id, generation)); -- D-030: no in-place overwrite
leases(key TEXT PRIMARY KEY, owner TEXT, expires_at INTEGER);
schema_migrations(version INTEGER PRIMARY KEY, applied_at INTEGER);
```

- `run_events` enforces monotonic `seq` per `run_id`; a trigger/transactional
  guard rejects a second terminal event for the same `run_id` (D-008).
- Checkpoint overwrite is generation-based (D-030): the `checkpoints` pointer
  row atomically flips to a new `checkpoint_generations` row; same-id writes
  never mutate an existing generation in place.
- Versioned migrations apply on startup; failure aborts startup (FR-8).

### 5.3 Two-layer Eino checkpoint bridge (D-028..D-030)

```text
EinoCheckpointAdapter        implements Eino CheckPointStore {Get,Set,Delete}
        │
        ▼
VersionedCheckpointStore     Vivy-owned: atomic replacement, generations,
        │                    checksum, metadata, listing, orphan detection,
        │                    engine-version validation
        ▼
BlobStore                    (SQLite backend in V0)
```

Checkpoint bytes never define product history. Product events may reference
only an already-durable checkpoint.

### 5.4 Approval write-ordering invariant (D-029)

For approval / resumable-cancellation, the durable write order is fixed:

```text
1. Vivy allocates checkpoint id
2. Eino Set + adapter durability
3. Eino emits interrupt
4. Adapter verifies checksum + generation
5. Vivy appends ONE journal commit containing run.suspended + approval.requested
6. UI sees the approval only after that commit is durable
```

Reverse order is a safety violation (closes the "UI sees approval before
checkpoint durable" crash window).

### 5.5 Backend conformance suite (D-032)

Before domain code trusts the SQLite backend, it must pass ≥16 cases:
atomic append, monotonic sequence, expected-version conflict, idempotent
replay, idempotency payload mismatch, exactly-one-terminal, first-writer-wins
approval, restart repair, torn final write, malformed complete data, orphan
checkpoint recovery, cancellation/approval recovery after kill, secret
redaction, Windows process-lock, monotonic replay under concurrent writers,
replay-after-disconnect. (Full 18-case suite is the V1+ target.)

## 6. Runtime: Run service + Eino adapter

`internal/runtime` is the only Eino door. Responsibilities:

1. Build a `ChatModelAgent` from a `domain.ChatModel` (via the provider) and
   a `Runner` (`adk.NewRunner` with `EnableStreaming=true` and the
   `EinoCheckpointAdapter` as `CheckPointStore`).
2. Drive a run: `runner.Query(ctx, userText)` → iterate the returned
   `AsyncIterator[*AgentEvent]`.
3. **Map** each Eino `AgentEvent` to a `domain.RunEvent` and persist it
   through the Journal before fanning out to SSE (task C4). Mapping table:

   | Eino signal | Vivy RunEvent |
   |---|---|
   | run start | `run.started` |
   | stream text delta | `model.delta` |
   | stream end / final message | `model.completed` |
   | tool call requested | `tool.requested` |
   | tool requires approval | `tool.approval_required` |
   | tool begin | `tool.started` |
   | tool result | `tool.finished` |
   | normal completion | `run.completed` |
   | provider/internal error | `run.failed` |
   | cancellation | `run.cancelled` |

4. **Cancellation**: cancel the run `context.Context`; Eino's cancel support
   (`.workspace/eino/adk/cancel.go`) produces a cancel signal that maps to
   exactly one `run.cancelled` terminal event (task E1, AS-5).
5. **Approval flow** (effectful tool):
   - tool emits interrupt → Eino persists checkpoint via adapter;
   - runtime verifies checksum/generation, appends one journal commit
     (`run.suspended` + `approval.requested`) per §5.4;
   - on approval decision, resume with `runner.ResumeWithParams(ctx,
     checkpointID, &adk.ResumeParams{Targets: map[address]approvalResult})`
     feeding the Approval back to the interrupted tool;
   - on denial, resume with a denial result so the model can continue
     (FR-6, AS-3).
6. **Restart recovery** (task E2, FR-8, AS-6): on startup, enumerate runs in
   `accepted`/`queued`/`active`; either resume (via checkpoint) or mark
   `failed` with a clear reason. Never write a duplicate terminal event.

## 7. HTTP API + events + UI

### 7.1 Endpoints (implemented D1/D2; contract in `schemas/api.openapi.yaml`)

```text
POST   /api/sessions                    create session
GET    /api/sessions                    list sessions
GET    /api/sessions/{id}               load session + messages
PATCH  /api/sessions/{id}               rename
DELETE /api/sessions/{id}               delete (tx: messages+runs+events)

POST   /api/sessions/{id}/messages      send message -> returns run (accepted)
GET    /api/runs/{id}                   run status
POST   /api/runs/{id}/cancel            cancel -> exactly one run.cancelled

GET    /api/approvals                   list pending approvals
POST   /api/approvals/{id}/decision     approve/deny (server-side enforced)

GET    /api/runs/{id}/events?after_seq=N   SSE stream (replay then live)
GET    /healthz                         liveness
```

- Every event carries `run_id`, monotonic `seq`, `created_at`, `type`,
  `payload_version` (FR-5). Reconnects resume via `after_seq` with no gaps or
  duplicates (AS-7).
- Approval authority is **server-side only** (D-009). The decision endpoint
  validates `run_id` + `tool_call_id` binding and expiration; a UI-only check
  is never authoritative, and tests must prove the server rejects a bypass.

### 7.2 UI shell (D3/D4)

- Vite-based, talks only to `/api/*` and the SSE endpoint. Never imports Go /
  Eino / reference types (D-007). No mock domain records (RK-5).
- Views: session list, chat with streaming, approval prompt, run status,
  run-detail / event-log view (reads from journal — D-017), error display
  with structured cause category, refresh-without-state-loss.
- Smoke-tested with Playwright against the **real Go process** (D4, FR-9).

## 8. Observability and errors (FR-11, D-017)

- Structured JSON logs (`log/slog`) carry `run_id`, `session_id`, `seq`.
- Provider errors surface as `run.failed` with a structured cause category
  (`provider_error` | `tool_error` | `internal_error` | `cancelled`), never
  silent failure.
- Stack traces at debug level only; user-visible errors do not leak internals.
- The run-detail UI view reads history from the persisted journal, not from
  in-memory state.

## 9. Anti-clone and scope gates (from the dossier)

- No `agent-diva-*` code import, schema inheritance, Tauri command surface,
  or 134-command port (D-001, D-005, D-013, D-021).
- Diva capabilities re-enter only through a separate capability proposal.
- No production-path mocks (PRD §6.2).
- Single Go module, single storage backend, exactly two providers — scope
  creep triggers a capability proposal, not a silent addition (RK-2, RK-4).

## 10. What this plan deliberately defers

Multi-provider breadth, MCP/plugins, multi-channel gateway, sandbox/shell,
context compaction, scheduler, memory/RAG, multi-agent Team/graph Workflow,
desktop pet/VRM/voice, SystemV/Rust migration, and the filesystem-journal
storage backend. All are `Deferred`/`Drop` per Assembly Options §5 and return
only via capability proposals.
