# Session Continuity and Deliverables Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans for native execution, or superpowers:subagent-driven-development if the user selects delegated execution. Follow the tasks in dependency order; checkboxes track implementation, not this planning delivery.

**Goal:** Deliver Issue #51's history -> explicit reference -> coding -> explicit file delivery loop through both model tools and a complete retained GUI.

**Architecture:** Three narrow host-owned operations reuse Service.Run, the existing Journal, policy/approval and WorkspaceManager. Eino runs existing tools/messages; first-party storage adds bounded queries and atomic continuity operations. Frontend attaches explicit snapshots without implicitly granting further source access and renders additive delivery groups.

**Tech Stack:** Existing Go, SQLite/PostgreSQL, Eino v0.9.13, React 19, TypeScript, Zustand, Radix UI, Vitest and Playwright; no new production dependency.

**Planning package:** SC-P5, 2026-09-22.

**Spec:** [SC-D4 specification](../specs/2026-09-21-session-continuity-design.md), including §13 complete frontend interaction design.

## Global Constraints

- Product naming: Lite means the reduced-capability coding product with retained GUI; Headless means operation without a UI. The planned product recipe is recipes/lite.vivy.yml. Existing recipes/minimal.vivy.yml is a baseline path, not the product name.

- Baseline: a8d361b0244a1c40be513622bbdaebb5c9d40014 (current main). This SC-P5 refresh changes no approved product semantics.
- One Service.Run path, Journal, policy/approval pipeline and workspace owner. Eino imports remain confined to runtime/provider.
- Read repository AGENTS.md, ui/AGENTS.md, vivy-eino, vivy-plugin and vivy-kernel-ci instructions before their respective implementation tasks. Do not touch tenant data/ directories.
- Default: current session only, intersected with policy. Attaching selected excerpts grants access only to those captured excerpts.
- No automatic retrieval, long-term memory, persona, channels, SSH, image offload, publishing, new engine or external search service.
- New delivery groups are additive; delivery does not imply run completion or passed verification. Preserve saved excerpt when its source is deleted; disclose that copy semantics.
- Effective bounds are the minimum of SC-D4 ceilings and existing runtime limits. One backend-owned limits projection; no independent UI constants.
- No new public Port, hand-edited generated Assembly, separate permissions database, transcript database, filesystem mirror or permanent artifact archive.
- Documentation/commits are English; UI copy supports en/zh. Use verified human git attribution per AGENTS.md; do not inherit a generic AI identity.
- Each task uses fail -> minimal implementation -> pass -> focused commit. Required final gate is just ci plus real split-GUI smoke; focused tests are not substitutes.

## Review Focus

1. Excerpt attachment with the further-reading checkbox off must not expose adjacent source messages (T1, T5, T7).
2. Concurrent append/backfilled projections sharing timestamps must not destabilize history pagination (T2, T3).
3. A queued submission or lost start response must not lose references or create duplicate tasks (T4, T7).
4. Source deletion, rewind and compaction are different from corrupted saved data; display and feed must distinguish them (T6, T8).
5. Same-path additive delivery, file replacement, connection loss and active HTML must not counterfeit a delivered version or execute content (T9-T11).

## Execution entry point

Use the [Story index](session-continuity/index.md) for the only status/dependency table, execution waves and T0–T12 plans. This file owns shared contracts and constraints; each Story owns its implementation checklist. The approved SC-D4 remains the sole product design.

## Shared contract ledger

These names are the shared contract; later tasks must not create alternative DTOs. Go types live in the three domain files named by T1; JSON tags use snake_case. Existing SourceRef, HistoryItem, ContextReference and Deliverable fields are specified in SC-D4 §5.

| Type | Fields / invariant |
| --- | --- |
| HistoryScope | session_ids []SessionID, workspace bool; caller selection only, default empty means current session |
| AcceptedHistoryScope | destination_session_id, source_session_ids, scope_hash; host-resolved at task admission |
| HistorySelection | source_session_id; exactly one of refs []SourceRef (max100) or run range {run_id,from_seq,to_seq}; kinds restricted to source records |
| ReferenceSelection | selection HistorySelection, expected_digest string; no browser content |
| HistorySearchRequest | query, session_ids, from/to, kinds, artifact_id, task_id, cursor, limit |
| HistoryReadRequest | selection, optional reference_id, cursor, limit; selection/reference_id mutually exclusive |
| HistoryTraceRequest | exactly one of source_ref, reference_id, deliverable_id |
| HistoryPage | status, items []HistoryItem, next_cursor, truncated, redacted, warnings, reason, optional selection_digest for a complete attachable selection |
| ReferencePreview | selection, items, digest, captured_at, byte_count, source_status |
| ContinuityInput | request_id, references []ReferenceSelection, history_scope HistoryScope |
| PresentRequest | files []{path,description}, optional title |
| DeliverySet | id, session_id, run_id, tool_call_id, created_at, title, items []Deliverable, failures []{path,reason}, status |
| DeliveryReadRequest | item_id, expected_digest, transfer_id?, offset, length |
| DeliveryChunk | transfer_id, item_id, digest, offset, data_base64, eof, expires_at |
| ContinuityReceipt | session_id, run_id, operation, request_id, input_hash, event_seq; event contains exact operation result |

Host operations (interfaces consumed by tools, implemented by runtime). Context carries trusted actor/authority from existing adapters; these signatures do not let the model set it:

```go
type HistoryOperations interface {
    Search(context.Context, domain.HistorySearchRequest) (domain.HistoryPage, error)
    Read(context.Context, domain.HistoryReadRequest) (domain.HistoryPage, error)
    Trace(context.Context, domain.HistoryTraceRequest) (domain.HistoryPage, error)
}
type ReferenceOperations interface {
    Preview(context.Context, domain.HistorySelection) (domain.ReferencePreview, error)
    Attach(context.Context, domain.ReferenceSelection) (domain.ContextReference, error)
}
type DeliverableOperations interface {
    Present(context.Context, domain.PresentRequest) (domain.DeliverySet, error)
}
```

Preview is used by trusted RPC composition; model Attach still requires its scope/policy. GUI task attachment goes through admission, not a forged model Attach context. Reference read by ID reads the destination-owned captured copy and does not broaden source scope.

Runtime-only service methods: HistoryService implements Search/Read/Trace; ReferenceService implements Preview/Attach; DeliverableService implements Present, List(ctx,sessionID,cursor,limit), Get(ctx,setID), Read(ctx,DeliveryReadRequest), CloseTransfer(ctx,transferID). List returns a paged set envelope; Get returns one set; CloseTransfer returns error. All use trusted context and authorized owner IDs. Reference projection responses additionally expose source_status and feed_status independently of saved snapshot content.

### SQL ownership and atomic contract

No table duplicates transcript or saved snapshots. Add:

- messages.position BIGINT, unique(session_id,position); sessions.next_message_position BIGINT. Backfill positions by (created_at,id) within each session; historical order is deterministic but marked legacy where source ordering is ambiguous. Allocate new positions under the same session write lock as message insertion, including projected messages, edit and fork.
- continuity_receipts: session_id, operation, request_id, input_hash, run_id, event_seq; unique(session_id,operation,request_id). Tool request_id includes stable tool_call_id under the owning run. Turn admission uses caller-stable request_id within the session. Receipt is a lookup projection; authoritative result and request fingerprint are in the pointed event.
- Event indexes for run/type/seq and run/session lookups only when not already present. Delivery/reference lookup may use bounded domain-event type queries first; no separate mutable delivery table.

Add storage.HistoryQueryStore with CaptureHistoryCut(ctx, authorizedIDs, filter) and QueryHistoryPage(ctx, cut, position, limits). HistoryCut contains up to100 session message ceilings and256 run seq ceilings; reject wider cut. HistoryPosition selects stream kind plus stable session/run key and local position. HistoryCandidates contains bounded raw records, next position and scan exhaustion; stays host-internal. Use SQL byte-length metadata before allocating large bodies. Oversized serialized payloads return an unavailable/truncated record without exposing a partial raw JSON prefix; source selectors can narrow to safe typed fields only where the existing schema permits it. Redaction/matching occurs in runtime before public output.

Add storage.ContinuityStore with CommitContinuityRun(ctx, admission), CommitContinuityOperation(ctx, mutation), FindContinuityReceipt(ctx, sessionID, operation, requestID). ContinuityAdmission contains complete ordinary user Message, Run, ordered RunEvents (started + references), accepted scope, exact source expectations and receipt. ContinuityMutation contains owner RunID, typed domain event, input fingerprint and receipt. Result returns assigned events plus original result on duplicate. Source expectations contain IDs/digests from server-validated projection; transaction rechecks current rows/visibility.

Admission must lock source and destination sessions in sorted-ID order, validate source deletion/truncation revisions, then insert all rows atomically. Postgres uses row locks; SQLite uses its existing serialized writer transaction with bounded busy handling. Terminal appends and continuity operations share run serialization. Source deletion uses the same session locking order. Do not rely on process mutex alone for backend integrity.

## Requirement-to-task coverage

| Spec | Implemented by tasks |
| --- | --- |
| §1–3 ownership, reuse, Eino | T0,T1,T3,T6,T10,T12 |
| §4 authorization and no implicit source grant | T1,T3,T5,T7 |
| §5 typed schema, limits, provenance | T1,T2,T3,T5,T9 |
| §6 search/read/trace and GUI inspection | T2,T3,T7,T8 |
| §7 explicit references and lifecycle | T4,T5,T6,T7,T8 |
| §8 presentation and authorized bytes | T9,T10,T11 |
| §9 recovery/deletion/partial failures | T2,T4,T6,T9,T10,T12 |
| §10 code ownership | Per-task exact paths above |
| §11 full coding/Lite evidence | T12 |
| §13 complete frontend state/interaction | T7,T8,T11,T12 |

## Handoff and completion rules

Design is confirmed; the restructured Story package requires user review before execution per Superpowers. Recommend native implementation because contract/storage/app/UI seams are shared, with one whole-branch review after implementation. Delegated execution is available if selected, with isolated worktrees and no concurrent writes to common files.

Known prerequisites: accepted predecessor evidence and usable just/Go/pnpm/Postgres/browser environment. PR #45 is already merged: `internal/storage/migrations` owns paired embedded SQLite/PostgreSQL migrations through `manifest.go`/`runner.go`; highest is `023_workspace_path.sql` and `024` is only the next available logical number. These are explicit execution gates, not unimplemented product semantics. No time or performance estimate is claimed. On completion report implemented/verified/unverified separately, retain iteration evidence, and push/create PR only under user authorization.
