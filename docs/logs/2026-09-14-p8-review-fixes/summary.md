# P8 review hardening (post PR #24 / PR #25 audit)

Date: 2026-09-14
Branch: `fix/p8-review-fixes` (from `main` = `cf98076`)

## What changed

Follow-up to the code review of PR #24 (PLG-P8 Gate A) and PR #25 (Gates B
and C). Three review risks and two nits were fixed directly instead of being
tracked on the board:

1. **Observer delivery retry backoff** (`internal/observerhost/host.go`):
   a persistently failing Run Observer no longer spins at the base retry
   interval forever. The delay doubles per consecutive failure and caps at
   `MaxRetryDelay` (default 60s, config-exposed). Delivery is never
   abandoned, so the at-least-once receipt contract is unchanged. Attempts
   reset on success.
2. **Required/reserved expired context fails closed**
   (`internal/contexthost/host.go`): an expired candidate with strict
   treatment now returns the new `ErrRequiredContextExpired` instead of
   being silently dropped, matching the fail-closed behavior already used
   for oversize, budget, and invalid-provenance paths. Competitive expired
   candidates are still omitted (`DroppedExpired`).
3. **Context View recovery cache + iterator error handling**
   (`internal/runtime/service.go`): `contextViewForRun` memoizes the
   committed Context View per live run (`Service.contextViews`, evicted in
   `cleanupRunState`), eliminating the full-Journal replay on every tool
   resume (O(n^2) over a long run). `drive()` seeds the cache from the
   fresh preparation. Journal iterator errors are now observed (`it.Err()`)
   and logged as a warning; recovery degrades to no View instead of
   silently depending on a partial scan.
4. **Dead allowlist entry removed** (`sdk/internal/assembly/runtime_generate.go`):
   the generated Run Observer projection no longer allows the `"result"`
   payload field, which no terminal payload defines.
5. **Docs table header** (`docs/architecture/SCX-PLUGIN-INTEGRATION.md`):
   the pinned-Eino decision table now says "Decision for Gates A–C" and the
   retrieval row no longer references the already-passed Gate B as future.

## Scope explicitly not done

- Dotted-path payload projection and `terminalRunIDs` startup scan remain
  as-is (future needs, no current consumer).
- `time.Sleep`-based synchronization in observer conformance tests was not
  converted to channels.
- `scx-reference` capacity semantics (`DeliveryFailed` receipt) unchanged;
  the backoff cap contains its retry cost.
- No `docs/TODO.md` entries: every finding in this iteration was fixed in
  the same iteration, at the owner's request.

## Round 2 — plugin v1 foundation audit fixes

A second review pass covered the P1-P7 foundation the P8 work sits on
(`compiler.go`, `graph.go`, `grants.go`, `source.go`, `source_verify_v1.go`,
`sourcehash/tree.go`, `frontend_v1.go`, `port/catalog.go`) at
`origin/main` (`cf98076`). Three risks and one nit were fixed on the same
branch, in an isolated worktree (`../agent-vivy-p8-fixes`) because the root
checkout was occupied by another lane:

1. **Source firewall widened and re-scoped as advisory**
   (`sdk/internal/source_verify_v1.go`): the forbidden import list now also
   rejects `plugin`, `syscall`, `golang.org/x/sys`, and `unsafe`.
   `VIVY-MODULE-STANDARD.md` §5 gains a "Source firewall scope" note: the
   scan is defense-in-depth over direct imports and package-level selectors;
   the authoritative capability boundary stays with Host interfaces and
   Grant enforcement.
2. **T2 sources without a verified root are rejected**
   (`sdk/internal/assembly`): a T2 `SourceRecord` with an empty `Root` now
   fails Compile with "lacks a verified source root" unless it explicitly
   carries the new `RootlessFixture` flag (test fixtures only; production
   catalogs never set it).
3. **Orphan grant approvals are rejected** (`compiler.go`): a
   `GrantApprovals` entry naming an unselected module now produces a
   diagnostic, symmetric with the existing orphan source-pin check.
4. **Single repository Module table** (`frontend_v1.go`):
   `snapshotSourceDirs` and `sourceRecords` now derive from one
   package-level `repoSourceDirs` table; the scx-reference
   required-context special case became a table column instead of a magic
   path comparison.

Foundation findings deliberately not changed: `sourcehash`'s single
generated-file exclusion (no second generated file exists) and the empty-ID
requirement matching in `compileRequirement` (no consumer supplies empty
IDs today).

## Verification

See `verification.md`. Acceptance view in `acceptance.md`.
