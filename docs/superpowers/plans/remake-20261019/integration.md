# Integration, worktrees and cross-computer handoff

## Native structure

Future branch names: `remake/<lane>`, shared candidate `remake/integration`, maintenance `remake/misc`; optional #93 branches `remake/nudge`, `remake/warming`, `remake/steering`. No branches/worktrees were created in this delivery.

Per-lane directory layout:

| Path | Ownership |
|---|---|
| `<lanes-root>/<lane>/agent-vivy-remake-<lane>` | The branch's own Git worktree, scratch/home/assets/ports |
| `<lanes-root>/<lane>/laputa` | That lane's exact pinned dependency; never shared mutably with another lane |

The repository's donor `scripts/lane-new.mjs` hardcodes `feat/<slug>` and defaults its base to HEAD. It does not support `remake/<lane>` unchanged. Use native Git worktree operations with an explicit accepted B; do not silently create a feat branch or add a wrapper merely to satisfy naming.

Example for an authorized future execution, in the repository with actual shell-safe absolute paths and B substituted:

```bash
git worktree add /ABS/LANES/notebook/agent-vivy-remake-notebook -b remake/notebook ACCEPTED_CORE_SHA
```

`ACCEPTED_CORE_SHA` and `/ABS/LANES` are explanatory placeholders, not runnable values. After creation, bootstrap from that worktree using its actual supported dependency procedure so `../laputa` belongs to this lane. Reports may start from the accepted Notebook output because it consumes that capability; record the exact predecessor commit. Do not chain unrelated features just to make integration easier. Existing unrelated branches must never be reset or overwritten.

Git worktrees are local directories. To resume on the other computer, publish authorized branches or provide an explicit Git transfer separately, fetch them there and recreate worktrees from exact commits. This plan archive contains documentation/inventory, not Git objects or an implementation branch. No publication is performed now.

## One batch cycle

1. Pin current accepted core/candidate and actual prerequisites; inspect changes since the lane's original base.
2. Release one bounded sub-batch after hunk/API/data/default-selection refinement.
3. Feature author supplies the smallest change and focused evidence; shared pin/migration/Assembly changes are sequenced through the integrating writer.
4. Generate projections from source, never copy checked-in generated donor outputs.
5. Assemble exact candidate, review conflicts and run applicable final gates. A merge changing relevant code invalidates earlier evidence; an unchanged tested source receipt can be reused only with explicit source/pin equivalence.
6. Record accepted feature behavior and remaining limits, then choose the next batch. Deferred scope stays visible and is not marked passed.

Independent review is applied at risky boundaries and exact integration candidates; this package selects no agent topology and creates no automatic task. Keep active write lanes within review capacity; multiple directories do not require simultaneous implementation.

## Shared file serialization

| Surface | Integrating rule |
|---|---|
| go.mod/go.sum/Laputa lock | One compatible closure per candidate; no competing repins |
| Service/App and common RPC/DTO | One integrating writer for overlapping hunks; existing authority remains single |
| Paired migrations/format conversion | Allocate after B; preserve released bytes and finite source support |
| Recipe/generator/Assembly/UI projection | Edit authoritative source first, regenerate once per selected combined candidate |
| ChatInput/i18n | Sequence Provider, quick-approval and queue UI changes; no stale whole-file overwrite |
| Test homes, DBs and ports | Per lane; serialize known process-global fixtures and shared default ports |

## Receipt contract (proposed planning data)

Each released batch records: `batch_id,lane,donor_shas,base_sha,head_sha,accepted_prerequisites,retained_hunks,omitted_dispositions,dependency_pins,data_sources_and_target,commands_and_results,pending_gates,review_receipt,rollback_boundary`. The receipt is a small document in the existing issue/PR or iteration log, not a new runtime evidence/hash subsystem. #83 coordinates; #81/original issues consume capability acceptance; #91 and #93 remain independent owners.
