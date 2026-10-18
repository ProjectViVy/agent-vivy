# Issue #81 E1-BML closeout — summary

Batch **L1 BML** of issue
[#81](https://github.com/ProjectViVy/agent-vivy/issues/81), package E1, Stories
E1-BML-01 through E1-BML-04, executed against the plan package on
`plan/issue81-bml-closeout` (`080186ae`) without merging that branch.

## Delivered

Four test-only changes, one commit per Story, on lane
`.worktrees/issue81-bml/agent-vivy`, branch `feat/issue81-bml`, based on
`origin/main` @ `e9609296`:

| Story | Commit | File(s) |
|---|---|---|
| E1-BML-01 | `cf8f9bc6` | `internal/modules/memory/persistence_acceptance_test.go` |
| E1-BML-03 | `25915b32` | `bml/replay_persistence_test.go`, `internal/modules/memory/replay_persistence_test.go` |
| E1-BML-02 | `64cd48bf` | `internal/app/bml_closeout_test.go` |
| E1-BML-04 | `88200c8b` | `ui/playwright.memory.config.ts`, `ui/e2e/memory-backend.ts`, `ui/e2e/memory.spec.ts` |

No product code, schema, runtime API, recipe, generated Assembly, or UI source
changed. `git diff --stat e9609296..HEAD -- internal/ bml/ plugins/ ui/src/
recipes/` shows only new `_test.go` files.

## Coverage map

| Requirement | Evidence |
|---|---|
| BML-R1 | `TestBMLCloseoutPublicDiskLifecycle` opens one SQLite root through the public `memory.Open` path, writes through module actions, closes all handles, reopens, and reads the same ID/content/revision. |
| BML-R2 | `TestBMLCloseoutActualModelConsumption` composes the ordinary default App, closes and reopens it at the same root, opens a fresh session, and proves a real loopback `chat.completions` request carries the BML context block into the model input. |
| BML-R3 | Same test: corrected body reaches the request at revision 2 and the superseded body does not; removed and absent records produce no BML block and an honest `BML_UNKNOWN` reply from the request-derived fixture. Story 01 covers the public CAS/removal outcomes. |
| BML-R4 | `TestBMLCloseoutStableEventReopenReplay` (receipt identity and single candidate across two restarts) and `TestBMLCloseoutTombstoneReplayAcrossReopen` (FTS exclusion survives reopen, insert-only replay of a tombstoned record is rejected as `memory_revision_conflict`, explicit rewrite at the observed revision is allowed). |
| BML-R5 | `ui/e2e/memory.spec.ts` drives the real page at the split Vite origin against a packed backend: create, process restart, search, two-tab stale CAS, delete with reason, restart again, outage. |
| BML-R6 | Verified candidate and limitations are recorded in `verification.md`; full `just ci` was not available on this macOS host (see limitations). |

## Explicitly not done

- No push, PR, merge, or change to the plan branch; delivery is local commits on
  `feat/issue81-bml` only, as agreed before execution.
- No product behavior was added or fixed. These tests characterize capabilities
  already on `origin/main`.
- No new memory capability is claimed: there is still no operator-visible
  history delete, no `memory_add` operation ID / exactly-once write, and no
  physical-erasure claim. Story 02's deterministic provider proves the context
  plumbing, not live-model answer quality.
- `docs/TODO.md` was not edited. The pre-existing UI unit-suite failures observed
  during verification belong to another track and are recorded here rather than
  as a new board row.

## Lane facts worth keeping

- Lane base is `e9609296` (`origin/main`). Local `main` in the shared root is
  `16808b5a`, one unpushed docs commit ahead; nothing from that commit is in this
  lane.
- The lane has its own `laputa` checkout and its own `.workspace/`, so no daily
  data, journal, or shared `laputa` was read or written.
- Iteration evidence lives in this directory; runtime scratch lives under the
  lane's gitignored `.workspace/issue81-bml-e2e/`.
