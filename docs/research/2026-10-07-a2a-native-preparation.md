# A2A native candidate preparation — write attribution and cleanup contract

Date: 2026-10-07. Branch: A2A. Task: A2A-00.2.
Scope: what a candidate session writes before commit, which existing native
symbol cleans up each resource, and what is missing. Evidence is the merged
tree on this branch and pinned laputa `ff3936f44ff8cf08c12af2cf698c194cfe474fd3`.

## 1. Write-attribution map

A candidate session for `ChannelTaskAdmission.NewSession` may write in three
places before the SQL commit:

| Resource | Writer | When | Belongs to |
|---|---|---|---|
| `<workspaceRoot>/<runID>` private dir (0700) | `runtime.AdmissionWorkspaceAllocator.EnsureForAdmission` | pre-commit | filesystem |
| `frozen_core_sessions` row in `garden.db` | laputa `personactx.Store.Capture` via `agentapi` Bootstrap inside `divacognitive.bundle.Prepare` | pre-commit (prompt authority) | laputa store, not core SQL |
| `sessions`/`messages`/`runs`/`run_events`/`continuity_receipts`/`channel_tasks*` rows | `CommitChannelTask` transaction | commit | core SQL (one tx) |

Nothing else writes session-scoped state during Prepare:
`agentapi.BindHumanSession` is a pure binding (no store write),
`PersonaStatus` only reads the profile persona directory, ingest/evolution
writes happen at run-end `Capture`, i.e. only for a committed run.

## 2. What blocks a missing Session today

Traced on `internal/runtime/service.go` admission (~940–1300):

1. `sessionSandbox` (service.go:5128) — `GetSession` failure already falls
   back to product defaults. No change needed for a candidate.
2. `MaskResolver.Capture` → `ReadMaskCapture` (sqlite/masks.go:211,
   postgres/masks.go:207) — `COUNT(sessions)=0` → `mask.CodeNotFound`.
   Candidate mode must synthesize an empty revision-zero capture without the
   store read (design already says this; verified the type is
   `mask.Capture{Selection: maskcontract.Selection{SessionID, Revision:0}}`).
3. `selectedWorkspace` (isolation.go:244) — `sessions.GetSession` →
   `ErrNotFound` propagates through `EnsureForAdmission`. Candidate mode must
   resolve as "no selected workspace" without the lookup. This tolerance must
   be candidate-scoped: for an existing-context admission a missing row must
   still fail (do not hide a deleted caller-supplied context).
4. `sqliteLockContinuitySessions` / `postgresLockContinuitySessions`
   (sqlite/continuity.go:~360, postgres/continuity.go:36) — missing
   destination session → `ErrNotFound`. For `NewSession` the new backend path
   inserts the Session row inside the commit transaction instead of locking a
   pre-existing one.
5. Receipt pre-check `FindContinuityReceipt` is continuity-scoped; the A2A
   `FindChannelTaskReceipt` has no existing-session dependency.
6. Prompt snapshot: `ContinuityAdmission` carries no prompt field; the proven
   tx-local helper is `sqliteInsertAdmissionPrompt` (run_admission.go:227),
   already used by `CommitChildSessionAdmission` (child_sessions.go:60-130),
   which is the existing in-tree template for "session row + message + run +
   prompt + started event in one transaction".

## 3. Existing loser-cleanup verification (bounded experiment)

`go test ./internal/runtime -run 'TestEnsureForAdmission|TestDiscardNewPrivateAdmission|TestAdmissionAllocator' -count=1 -v`:

```text
--- PASS: TestEnsureForAdmissionPrivateLifecycle          (create→discard→idempotent)
--- PASS: TestDiscardNewPrivateAdmissionPreservesNonEmptyDir (non-empty dir kept)
--- PASS: TestAdmissionAllocatorNeverRemovesUserDirectories (local+selected untouched)
```

`DiscardNewPrivateAdmission` removes only an *empty* `runID`-named private
dir (`os.Remove` refuses non-empty), no-ops for local/selected workspaces, and
the accepted winner's resources are never reachable through it. The service
call site `discardAdmissionWorkspace` (service.go:1443) already applies it on
definite-loss paths and preserves it on `ErrCommitUncertain`.

**Negative finding (frozen rows):** pinned laputa `personactx.Store` exposes
only `Capture` (`INSERT OR IGNORE` + re-read) and `Get`/`ErrSessionNotFound`;
no delete/discard exists anywhere under `garden/internal/personactx` or
`garden/agentapi`. A candidate FrozenCore row for a never-committed session
ID is unreachable but permanent — each failed `NewSession` attempt mints a
new session ID, so orphans accumulate without bound.

## 4. Crash-outcome reconciliation

- Pre-commit failure / definite loser: empty workspace dir removed by the
  existing discard path; frozen row orphans (gap, §5).
- Ambiguous commit (`ErrCommitUncertain`): workspace dir and frozen row are
  preserved; retry resolves through the channel-task receipt before any
  cleanup — same discipline as the existing continuity path.
- Process restart: committed runs settle via native restart recovery;
  never-committed candidates leave an empty dir and a frozen row, both
  invisible (the session ID was never returned). A native sweep fixture is
  required to keep accumulation bounded.

## 5. Smallest native extension (cross-store — owner review)

The workspace leg needs no new API: reuse `EnsureForAdmission` /
`DiscardNewPrivateAdmission` plus a candidate-scoped "no session lookup"
resolution inside `selectedWorkspace`, and a startup sweep in the runtime
that removes `<root>/<runID>` dirs that are empty, older than the maximum
admission window, and have no `runs` row.

The persona leg has no existing symbol. Minimal contract proposed:

- Pinned laputa gains `personactx.Store.DiscardSession(ctx, sessionID) error`
  (single `DELETE FROM frozen_core_sessions WHERE session_id = ?`) surfaced
  through `agentapi` as a session-scoped discard, and exposed on the
  cognitive bundle as an optional `DiscardFrozenSession` capability used by
  `discardAdmissionPersona` next to `discardAdmissionWorkspace`.
- Restart sweep enumerates `frozen_core_sessions` rows older than the
  admission grace window whose session ID has no core `sessions` row, and
  discards them. Enumeration needs a `ListFrozenSessions(ctx, capturedBefore)`
  companion, or the discard accepts a keep predicate supplied by the host.

Rejected alternative: opening `garden.db` from the host and issuing raw
`DELETE FROM frozen_core_sessions` — couples the host to a sealed vendor
schema and duplicates the store handle. Kept as fallback only if the owner
declines a laputa pin bump.

## 6. Outcome checklist vs task

- Failed candidates inaccessible: yes — session ID never returned pre-commit;
  core rows never visible without commit.
- No eager permanent Session: satisfied by minting `sess_<16hex>`
  (`newPrefixedID` discipline) with no row until `CommitChannelTask`.
- No lost prompt/Mask snapshot: prompt snapshot is a tx-local insert in the
  commit transaction; candidate Mask is synthesized revision-zero, asserted
  at commit.
- Definite losers cleaned: workspace yes (existing symbol); frozen row no —
  needs the §5 extension.
- Ambiguous outcome resolved by receipt before cleanup: yes, existing
  semantics + channel-task receipt.
- No unbounded orphan accumulation: workspace yes (discard + sweep); frozen
  no — needs the §5 extension + sweep.
