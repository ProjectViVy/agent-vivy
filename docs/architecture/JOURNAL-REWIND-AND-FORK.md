# JOURNAL-REWIND-AND-FORK — In-Session Message Edit / Rewind / Fork Design Proposal

> **Status:** proposal (design note; implementation not started)
> **Date:** 2026-09-02
> **Closes toward:** TODO §0.1 item UI-CHAT-ACT (`MessageBubble.tsx` has three disabled edit/rewind/fork placeholders)
> **Terrain map:** This proposal is based on a file-by-file field inventory of `internal/storage`, `internal/runtime`, and `internal/rpc` conducted on 2026-09-02 (all cited file:line references reflect facts as of that date).
> **Related:** `VIVY-ASSEMBLY.md` (event contract), `hitl-review-center.md`, and RB-1 research (file-level rewind—**a separate axis**, see §1.3).

## 1. Problem and Semantics

### 1.1 User Story

When a user is dissatisfied with their Nth message in a chat stream:

| Action | Semantics | Old messages after branching |
|---|---|---|
| **Edit (edit)** | Modify item N and rerun from it; everything after N (including N) leaves the context | Archived but hidden |
| **Rewind (rewind)** | Invalidate everything from item N onward (including N), then resend/regenerate from the end of item N-1 | Archived but hidden |
| **Fork (fork)** | Create a new session from the history "through item N" and preserve the original session unchanged | The entire original session remains visible |

All three share the same kernel primitive: **"Invalidate the history after message M."** Edit/rewind = primitive + resend; fork = a branch form of the primitive.

### 1.2 Hard Constraints (Inventory Conclusions)

1. **Journal is an append-only source of truth** (`Journal.Append` + `ErrRunClosed`, contracts.go:57-64): the only bulk deletion in the repository is the all-session transaction in `DeleteSession` (sqlite/sessions.go:107-114). The messages table has no deletes; run_events has no deletes.
2. **Model context reads the messages table**, not the event stream (`runMessages` → `ListMessages`, service.go:1151-1179); the event stream serves subscription/recovery/checkpoint. **messages table = model source of truth**, run_events = audit/subscription source of truth—both axes must be respected.
3. **Compaction establishes a "fold-on-read" precedent**: `CompactSession` deletes nothing, writes a `SessionCompaction{TailFrom}` row (compaction_service.go:168-178) plus a `context.compacted` event, and folds on read (`foldSessionHistory`, service.go:1186-1217). Truncation naturally belongs to the same family of mechanisms.
4. **There is no per-session active-run gate** (`s.active` is keyed by runID, service.go:380; `RunWithOptions` starts unconditionally)—truncation must include its own busy check.
5. Child-run semantics (`domain.Run.Kind/ParentID/RootID/Depth`, domain/run.go:85-94) exist, but **child runs are not re-executed after restart** (service.go:629-631)—if fork reused child semantics, it would inherit this recovery asymmetry, so it is **not adopted** (see §3.4).

### 1.3 Non-goals

- **File rewind:** RB-1 decided that the file-level version chain uses VC-3 storage; this proposal does not touch it.
- **Physical deletion:** No truncation implementation using DELETE FROM messages/run_events (the audit axis cannot be broken).
- **Multi-session batch operations / cross-session reordering:** Only single-point truncation within one session.
- **Retry/regeneration itself:** Already delivered (regeneration = resend the previous user input in a new turn, 2026-08-26).

## 2. Core Primitive: Logical Truncation Marker (No Row Deletion)

### 2.1 Storage (migration 020, same shape across sqlite + postgres + conformance)

```sql
CREATE TABLE session_truncations (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,   -- pg: BIGSERIAL
    session_id    TEXT NOT NULL,
    cutoff_message_id TEXT NOT NULL,   -- start of invalidated interval (inclusive)
    tail_message_id   TEXT NOT NULL DEFAULT '',  -- id of last message in session at marker time (inclusive)
    reason        TEXT NOT NULL,       -- 'rewind' | 'edit' | 'fork'
    fork_session_id TEXT,              -- points to new session when reason='fork'
    created_at    INTEGER NOT NULL
);
CREATE INDEX idx_session_truncations_session ON session_truncations(session_id, id);
```

- **Effective truncation = the union of all rewind/edit markers for the session** (R2 correction, caught by e2e replay: an edit flow = rewind + new turn + possibly another rewind; if "latest marker wins," the second rewind can resurrect an interval invalidated by the first; the view = the stored list minus the union of each closed interval `[cutoff, tail]`. fork/forked-from anchor rows do not participate in view folding and must not override rewind markers—view reads use `ListViewTruncations` (rewind/edit only), while audit reads use `LatestSessionTruncation` (latest for any reason). Old rows are kept forever—the invalidation itself is also a fact).
- Message table rows are **retained exactly as-is**: messages in truncated intervals are filtered at the read layer, not deleted. The audit axis remains complete: run_events, truncated messages, and truncation markers can all be replayed.
- **Tail anchor:** truncation hides the **closed interval `[cutoff, tail]`**—the last message that existed in the session at marker time. New turns appended after rewind (with IDs beyond tail) remain visible: an edit flow = rewind + an entirely new `turn/start`; without a tail anchor, folding would permanently hide the retry itself (a design flaw caught by R2 e2e offline replay; the initial open-interval semantics "invalidate everything after cutoff" are deprecated). Unknown cutoff/tail always fail-open (do not fold).
- Size budget: marker rows store IDs only, not content, so there is no 1MB-style limit; the number of markers per session is naturally bounded (one per manual action).
- The conformance suite adds a group (guard 20→21+): `latest` winner, cross-session isolation, zero markers for empty sessions, marker filtering effects on `ListMessages`/`runMessages`, and **visibility of turns appended after rewind**.

### 2.2 Read-Layer Folding (Single Filter Point)

The **runtime consumers** of `ListMessages` are uniformly changed to "take all rewind/edit markers for the session and remove the **union** of their closed intervals `[cutoff, tail]`" (see §2.1 for tail-anchor semantics):

- `runMessages` (model context, service.go:1151-1179)—after truncation, the history visible to the model is bounded by the cutoff;
- `session/messages` RPC (UI list, control.go:908)—the UI likewise sees the post-truncation view (old messages remain in the database, but the product view follows effective truncation);
- `trajectory/session` (trajectory projection)—the same filter keeps the three views consistent.

**Composition order with compaction:** truncation precedes compaction folding—messages after cutoff are removed first, so a `SessionCompaction.TailFrom` row that falls in the removed interval becomes ineffective (its summary's corresponding tail is no longer in context; rows are not deleted, and the folding function naturally skips it). Implement `foldSessionHistory` in two stages, `truncation → compaction`, which can be tested within one function.

**Readers that do not fold:** `run/log` (single-run event replay) and restart recovery (service.go:585-667) **do not filter**—the event stream of a truncated run remains fully auditable, and recovery continues to process its terminal state as before; recovery reconstructs "non-terminal runs," orthogonal to truncation (apart from the conflict surface in §2.4).

### 2.3 Events and Audit

The truncation action itself enters the event stream (reusing compaction's `RecordExternalRunEvent` pattern, compaction_service.go:779, with synthetic run ID `tr_<seq>`):

```json
{ "type": "session.truncated", "payload_version": 1,
  "payload": { "session_id": "...", "cutoff_message_id": "...",
               "reason": "rewind|edit|fork", "fork_session_id": "..." } }
```

**No approval:** truncation is a product action on the user's **own session view**; it triggers no tools or side effects and is at the same level as `session/delete` (which also requires no approval). D-010 semantics are unchanged (no sensitive payload is added).

### 2.4 Busy Checks and Interruption

At the entry points for `Service.RewindSession`/`ForkSession`, list active runs for the session (`ListActiveRuns` filtered by session); if the list is non-empty, return `ErrSessionBusy` (a new error, with `runtimeError` mapped to Conflict/InvalidParams—the mapping is defined in §4). On the UI side, `actionsDisabled` already disables buttons while a run is in progress (MessageBubble.tsx:138), providing defense in depth. **No "truncate and cancel" compound action** is provided—cancellation already has dedicated UI (`turn/interrupt`), keeping a single concern.

## 3. Implementation Shapes of the Three Actions

### 3.1 Rewind

`session/rewind` `{ session_id, message_id }`：

1. Busy check (§2.4);
2. Verify that the message belongs to the session and is not already inside an effective truncation interval;
3. Write a truncation marker (`reason='rewind'`) plus a `session.truncated` event;
4. Return the post-truncation view cursor. After receiving it, the UI pre-fills the input box with the rewound user message text (local behavior), and the user resends through the existing `turn/start`—the **kernel does not "automatically resend"**; both edit and rewind end with an explicit new turn.

### 3.2 Edit

= `session/rewind` (cutoff=message being edited) + existing `turn/start` (new content). The original message body leaves the context, and the new content is appended as a new user message. **No new RPC is needed in the UI**—this is a composition of two existing calls. Semantically, "old content is hidden after editing but retained," consistent with the table in 1.1.

### 3.3 Fork

`session/fork` `{ session_id, message_id, title? }`：

1. Busy check (the original session);
2. Create a new session (`SessionStore.CreateSession`, title defaults to `Original title · Fork`);
3. **Copy** the **effective-view** message rows up to and including `message_id` into the new session (including attachment references; attachment `data_url` is already inline in the message rows). Copy rather than reference: the two sessions evolve independently after the fork, avoiding cross-session read-through; the cost is one bounded INSERT (session history already has an MB-scale budget). R2 correction (caught by e2e replay): both cutoff location and copying use the **effective view** (rows after truncation-marker folding), not the raw stored list—rows already folded by rewind must not revive in the child session, and the child-session context equals the visible context at the fork point in the original session; requesting rewind/fork for a folded message returns `ErrInvalidCutoff`;
4. Write a truncation marker in the **original session**?—**No**. The fork leaves the original session unchanged! A marker is written in the original only to record the fork fact, but a `reason='fork'` marker **does not change the original session view** (`cutoff_message_id` records the fork point; filtering skips `reason='fork'`; it serves only as an audit and replay-protection anchor). Also write a reverse-provenance row with `fork_session_id` in the **new session** (`reason='forked-from'`, likewise not filtered). R2 addendum (caught by e2e replay): anchor rows **must not obscure** view rules either—view reads (`LatestViewTruncation`, latest rewind/edit only) and audit reads (`LatestSessionTruncation`, latest for any reason) are separate; if view reads also used the "latest row," a later fork anchor would override an earlier rewind marker and restore folded history;
5. Write a `session.forked` event in the new session (payload carries parent_session_id + fork point), return the new session_id, and have the UI navigate to the new session.

**Why child-run semantics are not adopted:** the product of a fork is a user-visible **session** (it can continue for multiple turns and has its own compaction, todos, and approvals), whereas a child run is a single-turn execution unit that is not re-executed after restart—their lifecycles are different.

### 3.4 After the Fork

- The new session is fully independent: independent compaction rows, truncation sequence, and token statistics;
- The original session's `session/compactions`, trajectory, and messages remain unchanged;
- `ListRunsBySession` (runs table) is not migrated—the forked historical runs remain in the original session, and turns in the new session begin with the first turn after the fork, so the trajectory panel naturally shows a "new session, new trajectory."

## 4. RPC Surface (Add 2 Methods, Dispatch Switch control.go:475-658)

| Method | Params | Return | Errors |
|---|---|---|---|
| `session/rewind` | `{session_id, message_id}` | `{cutoff_message_id, remaining_count}` | InvalidParams / NotFound (message does not belong to session) / Conflict (session busy / cutoff point already invalid) |
| `session/fork` | `{session_id, message_id, title?}` | `{session_id}` | Same as above |

`runtimeError` adds mappings for `ErrSessionBusy` and `ErrInvalidCutoff`. The capability list (control.go:477-502) registers both methods.

## 5. UI Wiring (Enable MessageBubble Placeholders)

- **Edit:** the bubble enters local edit mode (`textarea`) → after confirmation, `rewind(M)` + `startTurn(new text)`; cancel returns. While queued (`queuedMessages` is non-empty or `runBusy`), the button remains disabled (inherited from the current behavior).
- **Rewind:** confirmation dialog ("The N messages after this will leave the context (retained in the archive)") → `rewind(M)` → pre-fill the input box with the original text.
- **Fork:** confirmation dialog → `fork(M)` → navigate to the new session.
- Store: `rewindSession`/`forkSession` actions plus a `listMessages` reread after truncation (reuse the existing refresh path); after welcome, `session/messages` automatically receives the filtered view, with no extra frontend state.

## 6. Test Plan

1. **Storage conformance:** marker CRUD + latest-wins + filtering effects (both sqlite/postgres backends).
2. **Runtime:** after rewind, `runMessages` excludes messages after cutoff; compaction invalidation path after truncation; busy negative case (rewind with an active run → `ErrSessionBusy`); after fork, new-session context = copied history while the original session view remains unchanged; `session.truncated` event is recorded in the Journal.
3. **RPC:** positive/negative cases for both methods (NotFound/Conflict/InvalidParams); `session/messages` filtering takes effect.
4. **End-to-end:** e2e create session → two turns → edit the first message → assert the view contains only the post-edit chain + the new turn completes normally.
5. **Recovery orthogonality:** recovery behavior is unchanged when a truncated session contains a non-terminal run (§2.2 readers do not fold).

## 7. Implementation Split Recommendation (Not Scheduled, in Value Order)

- **R1:** migration 020 + `SessionTruncationStore` + folding + `session/rewind` RPC + busy gate (kernel only; UI enables only the "rewind" placeholder).
- **R2:** `session/fork` + UI edit/fork wiring + e2e.
- **R3 (optional):** polish trajectory/compaction composition details + close out the documentation.

Each slice independently passes `just ci` + conformance + one commit.

## 8. Open Questions (Must Be Decided Before Implementation)

1. ~~Whether `cutoff_message_id` should use the message ID or a `(created_at, id)` composite tie-breaker~~ **Decided (R2):** folding matches IDs by **list position** (`ListMessages` order is authoritative), not by created_at; add the **`tail_message_id` tail anchor** to bound the closed invalidation interval (§2.1). Position matching naturally avoids ties with the same created_at, and conformance CN-21 locks this down.
2. Whether large attachment messages copied by a fork should have a count limit—the preference is to reuse the existing session-history budget and add no new limit.
3. Whether the `session.truncated` event should enter the folded-summary hint in `session/context`—the preference is not to include it (truncation is a user action, not a context-budget event).
