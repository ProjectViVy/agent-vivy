# Summary — child runs share their root run's workspace

A delegated sub-agent could not see anything its parent had written. Under the
sandbox world the manager handed a child `<root>/child_<id>/`, a sibling of the
parent's `<root>/run_<id>/`, so a file the parent produced one turn earlier did
not exist as far as the child was concerned. Every child run now resolves to its
**root** run's directory instead of getting one of its own.

## What changed

1. **The redirect lives inside `WorkspaceManager`** (`internal/runtime/isolation.go`).
   `workspaceRunOwner` performs the durable `GetRun` once per request and returns
   both the run row and its session, so the existing session-selection lookup and
   the new ownership question share one read; `selectedWorkspace` now takes the
   resolved session id rather than a run id. `workspaceOwnerRunID` answers "whose
   directory is this": a durable `domain.RunKindChild` row with a `RootID`
   resolves to that root, anything else resolves to itself. The redirect is
   inserted after the selected-project and `local` branches, so those two worlds
   are byte-for-byte unchanged.

   `RootID`, not `ParentID`: child depth is bounded at 4, and keying on the root
   keeps a whole descendant tree in one directory instead of chaining a fresh one
   per level.

2. **A context bridge covers the pre-persist gap** (`internal/runtime/policy_context.go`).
   Child admission allocates the workspace *before* `CreateRun` persists the
   child row, so the durable lookup cannot answer yet. `withWorkspaceOwnerRun`
   carries the already-computed root id across exactly that gap, the way
   `withSessionID` already does; it is set at the two pre-persist allocation
   sites (`child_oneshot.go`, `child_sessions.go`). Once the row exists it is
   authoritative and the context value is ignored — a durable row beats a stale
   context, which one of the new tests pins.

3. **A shared directory is never reclaimed by a child's lifecycle.** `Workspace`
   gained an unexported `redirected` flag, set when the owner is not the run that
   asked. `Release` returns before its shape check (a redirected path *does*
   match `root/<ID>`, so the existing guard would have passed and deleted the
   parent's directory on an admission rollback); `EnsureForAdmission` reports
   `created=false` even when this call really did `mkdir`, because a directory
   created inside the crash window is still the root's to keep; and
   `DiscardNewPrivateAdmission` resolves the owner and returns before any
   `os.Remove`.

4. **The Journal records the resolved workspace.** `child.started` and the
   `run.*` terminals carry `workspace_id`, and that value is now the root run's
   directory name for a child. This is a value change only: the field was already
   there and already optional, so no schema, payload or consumer changed
   (`ChildRunDetails`, the run inspector projection and the UI all read the same
   key). No second field was added — a `child_workspace_id` beside it would be a
   second source of truth.

## The contract sentence

A host-requested one-shot child whose `tool_names` include a writable tool (D11)
now writes into the **parent's** workspace rather than a private one. The gate in
front of that write is unchanged: the child inherits `sandboxMode` and the
approval policy from the parent session, so an effectful call still stops at an
approval. `TestEffectfulOneShotChildSharesRootWorkspaceUnderApproval` pins both
halves of that sentence — shared workspace id, no per-child directory, no effect
before the decision, effect after it.

Stale-read tracking keys on `(sessionID, root, displayPath)`. Sharing the root is
the intended behaviour there: a one-shot child runs inside the parent session, so
the guard now spans parent and child on the same files. A continuable child has
its own child session id, so its read state stays separate from the parent's even
though the directory is shared.

## Explicitly not done

- No change to `world: local` or to sessions with a selected project root. Both
  already gave every run in the session the same directory; the defect was
  sandbox-world specific.
- `domain.RunKindWorkflow` runs keep a directory of their own. The redirect keys
  on `RunKindChild` only; widening it to workflow node runs is a separate
  decision with its own output-collection semantics.
- No automatic cleanup of the `root/<childID>` directories earlier builds already
  created. They are inert scratch and are boarded as
  `WORKSPACE-ORPHAN-CHILD-DIRS` rather than deleted behind the owner's back.
- No cache for the ownership read, because there is no extra read to cache: the
  `GetRun` that `selectedWorkspace` used to perform is now performed once by
  `workspaceRunOwner` and shared. Managers built without a run lookup
  (`NewWorkspaceManager`, `NewLocalWorkspaceManager`) do no store access at all,
  exactly as before.
- No change to path validation, symlink rejection, workspace-name validation, or
  the approval/policy path. A child's writes are still confined to the resolved
  root, and `ValidateRunPath` resolves through the same redirect.
