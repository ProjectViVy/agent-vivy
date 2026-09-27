# MEM-1C iteration log — memory UI management surface

Date: 2026-09-27. Branch: `feat/memory`. Executed serially in the lead session
(user-directed: no sub-agents for this story).

## What changed

- `plugins/vivy-memory/ui/vivy-memory/src/memory-client.ts` — client now covers
  all nine `vivy.memory.*` actions (list/search/get/add/update/remove/
  rules.read/rules.write/status). Outcome types mirror the Go
  `MemoryCrudOutcome` contract: `applied | listed | proposal_created | failed`
  plus `reason`, `entry`, `entries`, `evidence_advisory`, `proposal_id`.
  `MEMORY_REASONS` holds the snake_case reason vocabulary.
- `plugins/vivy-memory/ui/vivy-memory/src/dialogs.tsx` — new add/edit dialog
  (`MemoryContentDialog`) and delete dialog (`MemoryDeleteDialog`, required
  reason, destructive confirm). Both send `base_revision` for CAS. A
  `memory_revision_conflict` keeps the dialog open with a refresh affordance
  (`onRebase` → `vivy.memory.get` swaps in the freshest record via a keyed
  remount); other `failed` outcomes render the backend reason verbatim. No
  optimistic mutation, no fabricated success.
- `plugins/vivy-memory/ui/vivy-memory/src/rules.tsx` — `MemoryRulesPanel`:
  read-only view with source badge (`default`/`file`), edit → monospace
  textarea → `rules.write` with the `rules.read` digest token as
  `base_revision`. Conflict clears the draft and reloads.
- `plugins/vivy-memory/ui/vivy-memory/src/view.tsx` — Records/RULES tabs,
  status strip from `vivy.memory.status` (startup revision + no-database
  notice), New-memory button, detail pane with provenance, evidence refs, id,
  created/updated timestamps and revision, Edit/Delete actions.
- `plugins/vivy-memory/i18n/catalog.json` — 15 new units, en + zh both
  COMPLETE.
- Tests: 9 cases in `view.test.tsx` (status fetch, add payload shape, update
  CAS + conflict + rebase, delete reason gating, rules read/write CAS).

## Decisions

- UI-invoked writes execute through `module.action.invoke` with operator
  authority; the module defines no RequiredGrants/RequiresApproval, matching
  the control-plane authority model for `/memory` (a management surface), not
  the agent-side `memory_*` approval path from MEM-1B.
- No pagination UI: the wire contract has no `next_cursor`, so the list is a
  single page by contract.
- No `kind` selector on add: the backend only accepts `long_term` for
  operator UI writes today.

## Incident: source-hash fixed point

`vivy-module.yaml` `source.sha256` and `plugins/vivy-memory/module.go`
`module.Source.SHA256` pin the same tree digest, and `internal/sourcehash.Tree`
zeroes every occurrence of the declared digest before hashing. Re-pinning only
one site produced a non-converging digest (stage-ui failed twice). Procedure
now verified: write zeros into both sites, run
`go run ./sdk/internal/cmd/source-hash plugins/vivy-memory <zeros>` to get the
candidate, write it into both sites, confirm
`source-hash <dir> <candidate> == candidate`.
