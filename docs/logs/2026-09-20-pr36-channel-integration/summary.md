# CH-INT-36 integration summary

Date: 2026-09-20
Worktree: `feat/channel-pr36-integration` at `/workspace/scratch/1b883f7e5915/vivy-channel-pr36-integration`
Status: Local Tasks 1–6 complete and independently reviewed; Task 7 local
closeout is in progress. The real merge remains uncommitted until final scope
staging and review.

## Outcome

The locked PR #36 source was verified against the aggregate target and the
ancestry-preserving merge was left open for Tasks 2–7. The target, source, and
merge base are recorded below. The source contains 69 commits and changes 114
paths from the locked base. The aggregate target has only one descendant after
the planned code baseline, and that descendant is planning-only.

No conflict was resolved, no conflict resolution was staged, and no merge
commit was created in Task 1. Git's normal merge operation has staged cleanly
merged paths; the eight listed paths remain unmerged for the next task.

## Frozen inputs

| Input | Exact value | Evidence meaning |
|---|---|---|
| Current integration target (`HEAD`) | `c054a9b5b85a7405d071e33e3fc756114f26e2da` | Current aggregate head, `feat/channel-modularization` |
| PR #36 source (`MERGE_HEAD`) | `eb8fee3c0996746e93def297d700c70fb65b8540` | Locked `feat/channel-tier1` source head |
| Source tracking ref | `eb8fee3c0996746e93def297d700c70fb65b8540` | `origin/feat/channel-tier1` matches the lock |
| Target tracking ref | `c054a9b5b85a7405d071e33e3fc756114f26e2da` | `origin/feat/channel-modularization` matches `HEAD` |
| Merge base | `fe60b1869a166490da72a07eef8f1d754caa2e3e` | Exact planned PR #36 base |
| Source range | `69` commits | `merge-base..source` ancestry count |
| Source file count | `114` paths | `git diff --name-only merge-base..source` |

The plan's code baseline is
`0c47eb96c733a29be25cace9cd048be0c367ca10` with tree
`27d50ea95dd58fecdc0a5cdd5f1e2ede6ffd0d9a`. Its only descendant to the
current target is:

| Descendant | Classification | Changed paths |
|---|---|---|
| `c054a9b5b85a7405d071e33e3fc756114f26e2da` — `docs(channel): plan PR 36 aggregate integration` | Planning-only | `docs/superpowers/plans/channel-modularization/ch-p0-3-backend-omission.md`; `docs/superpowers/plans/channel-modularization/index.md`; `docs/superpowers/plans/channel-modularization/pr36-integration.md` |

No product-code descendant was found after the planned code baseline.

## Merge result

The prescribed planning merge was run with the frozen target and source:

```text
git merge-tree --write-tree --name-only c054a9b5b85a7405d071e33e3fc756114f26e2da eb8fee3c0996746e93def297d700c70fb65b8540
```

It exited `1`, produced computed merge tree
`aef901140178144e8e22db0cd7157216b8908b74`, and reported exactly these eight
conflict paths:

1. `docs/COMPLETE.MD`
2. `internal/app/app.go`
3. `internal/app/channels_test.go`
4. `internal/modules/channel/binding.go`
5. `internal/rpc/control.go`
6. `internal/rpc/control_test.go`
7. `sdk/internal/assembly/conformance_results.json`
8. `sdk/internal/conformance/reproduction_test.go`

The open real merge reports the same eight paths through
`git diff --name-only --diff-filter=U`. This is the required handoff state.

## Handoff scope

Tasks 2–7 may continue only within the approved integration plan:

- Preserve one real `--no-ff --no-commit` merge and the source branch's
  ancestry. Do not squash, cherry-pick, use `-Xours`/`-Xtheirs`, or blanket
  select one side.
- Keep `internal/app` on the `channelcontract.Owned` boundary and keep
  `internal/rpc` generic. `internal/modules/channel` remains the sole owner of
  Provider binding, Channel Host lifecycle, settings projection, inspection,
  failed-delivery management, and the five Channel RPC bindings.
- Keep `Service.RunWithOptions` as the only inbound run path and
  `Service.DecideApprovalAsActor` as the only approval-decision path.
- Keep `std/channel@v1` and existing platform Module descriptors compatible.
  Do not begin reduced Recipes, conditional backend imports, Channel UI Module
  packaging/generated UI selection, release matrix work, or rollback work;
  those belong to CH-P0-3 through CH-P0-5.
- Treat conformance hashes as final-tree outputs. Regenerate them from the
  integrated sources rather than choosing either conflict side's digest.

Task 1 stops and returns to the supervisor if the locked source changes, a
product-code target descendant appears, the conflict set expands, preserving
the behavior requires a public `std/channel@v1` break, app/RPC would import the
concrete Channel Host or create a second dispatcher/runtime/settings path, a
storage migration number collides, PR behavior conflicts with the normative
#42 architecture, or acceptance would require starting CH-P0-3, CH-P0-4, or
CH-P0-5.

## Local closeout status

The eight-path merge is now resolved with no unmerged index paths. Tasks 2–6
preserved canonical Channel ownership, integrated durable delivery/recovery and
UI/API behavior, adapted the P0-2 capability test to
`internal/modules/channel.BindProviders`, and regenerated exact conformance
evidence. The five Provider hashes use the canonical self-describing fixed
points from their manifests/descriptors; Pack/Inspect parity shows five
`std/channel@v1` providers with 15 matching conformance results each.

Independent Task 5 and Task 6 reviews are approved with no Critical or
Important findings. Local focused/full Go, SDK, plugin/face, UI, i18n, scope,
and Pack/Inspect gates pass. Final acceptance is still pending literal `just
ci`, live PostgreSQL evidence, cloud-browser split acceptance, and external
branch CI; none is claimed by this local log.
