# CMP-3 — Session compaction summary retrieval endpoint (session/compactions + compaction history panel)

## What changed

The `session_compactions` table previously supported only writes and "get the latest"
(`LatestSessionCompaction`, used for feed assembly), with no retrieval surface: operators
could not review a session's historical compaction records. This slice completes the
three-stage retrieval path from storage → RPC → UI, with no new table (reusing the existing
`session_compactions` table).

### Storage layer

- `internal/storage/contracts.go`: `CompactionStore` adds
  `ListSessionCompactions(ctx, sessionID, limit)` — returns records for the session,
  newest-first (`ORDER BY created_at DESC, run_id DESC`, with the same tie-breaking order
  as `LatestSessionCompaction`); `limit <= 0` returns empty (negative `limit` means
  "unbounded" in sqlite and must be blocked).
- `internal/storage/sqlite/compaction.go`, `internal/storage/postgres/compaction.go`:
  same-shape implementations (`?` vs `$n` dialects; postgres goes through `b.db.SQL`).
- `internal/storage/conformance/suite.go`: CN-20 "compactions listed by session"
  (guard 19→20): three records across two sessions, asserting ordering (same
  `created_at` ties are decided by `run_id DESC`), row-field round-trip, limit truncation,
  an empty set for an unknown session, and an empty set for `limit=0`.

### RPC layer

- `internal/rpc/control.go`: `ControlDeps.Compactions` (`nil` → MethodNotFound,
  shaped like SkillRevisions/Todos); new `session/compactions` method:
  - `session_id` is required (missing → InvalidParams); unknown session → CodeNotFound;
  - `limit` is optional, defaults to 50, and is capped at 200;
  - returns `{compactions: [{run_id, created_at, tail_from, dropped_count, summary}]}`;
  - `summary` is untrusted generated content (it passed through the model feed); the
    server passes it through verbatim, and the UI's React text node handles injection
    protection.
- `internal/app/app.go`: assembly-layer `Compactions: backend` wiring.
- `internal/rpc/control_test.go`: `TestControlHandlerListsSessionCompactions` asserts
  everything — empty `[]` (not null), newest-to-oldest ordering, field round-trip,
  `limit=1` truncation, unknown-session NotFound, missing-parameter InvalidParams, and
  unwired MethodNotFound.

### UI layer

- `ui/src/lib/api.ts`: register `session/compactions` in `RPC_METHODS`;
  `SessionCompactionRecord` type; `listSessionCompactions(sessionId, limit=50)`.
- `ui/src/components/settings/CompactionSettingsCard.tsx`: adds a "Compaction history"
  block below the usage panel — no session → guidance copy; session with no records →
  empty state (`data-testid="compaction-history-empty"`); records → entry list
  (run badge + local time + folded-message count + summary line-clamp-3,
  `data-testid="compaction-history"`). Session changes load automatically; a successful
  "Compact now" refreshes automatically; the "Refresh" button refreshes history too. The
  summary renders as a React text node (automatic escaping).
- `ui/src/i18n/en.ts`, `zh.ts`: six bilingual keys:
  `settings.compaction.historyTitle/historyLoading/historyEmpty/historyOpenSessionHint/
  historyRun/historyDropped`.
- `ui/e2e/compaction-setting.spec.ts`: adds a "Compaction history panel" spec — title
  visible in zh/en, empty state or guidance visible, and a regression check for raw i18n
  keys.

## What was explicitly not done

- Expansion/full-text viewing UI for the summary body (currently folded with line-clamp-3)
  — open a later slice if full-text retrieval is needed.
- A cross-session compaction-record aggregation/management page (CMP-3 is a
  single-session retrieval endpoint).
- A live CN-20 run against the postgres backend: the postgres conformance suite requires
  an external DSN to start (existing convention); this slice is fully green on sqlite, and
  the postgres implementation has the same shape and test semantics as sqlite.

## Board

`docs/TODO.md` CMP-3 → DONE (§10 same-day record).
