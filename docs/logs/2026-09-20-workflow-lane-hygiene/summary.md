# Root-tree lane hygiene: WF-1 stops living in `main`

Iteration: 2026-09-20. Scope: repository governance — the root working tree,
the `just` gate recipes, and the checked-in `internal` source digest. No
product behavior changed.

## What was wrong

Two untracked files from the WF-1 workflow lane were sitting in the root
working tree:

- `internal/workflow/workflow_test.go` (SHA-256
  `A1033B04EA4ABA8813B675931DD20F44765E123C5CD46D01E4F16CDA599B47C5`) — a
  test-only file whose package has no non-test files on `main`, so it did not
  compile: `go vet ./internal/workflow/` failed with
  `undefined: ValidationContext`.
- `internal/domain/workflow_test_support.go` (SHA-256
  `401E7EC59093443E34C24AB4C0BEEC4211F115E4B9E6C73A2B25094FFE2001B7`) —
  declares `errWorkflowExecutionUnavailableForTest`, which nothing in the
  repository references.

They cost more than tidiness:

1. `just test` and `just vet` carried a transient carve-out that filtered
   `agent-vivy/internal/workflow` out of `./...`, so the root gate passed by
   not looking. The recipe comment said to revert it "once that lane lands".
2. `internal/sourcehash.Tree` hashes **every file under `internal/`**, tracked
   or not. Both strays were therefore folded into `main`'s canonical source
   digest: the live digest and the checked-in
   `sdk/internal/assembly/conformance_results.json` both read
   `af2bcb75e3f80a04f22ee29c59226ad1cef8b4ae46737cce12be0813e65516f3` — an
   identity that included another lane's work-in-progress.

The lane itself is legitimate work: `feat/workflow-wf1` carries
`08fa3d2 feat(workflow): ship WF-1 workflow product slice (#40)` and is
pushed as `origin/feat/workflow-wf1`. What was wrong was the *venue*, not the
work. The branch is deliberately independent and may never merge.

## What changed

- Removed both strays from the root tree. Their only copies on `main` were
  these strays; the lane's own copies are tracked on `feat/workflow-wf1`, and
  the root `workflow_test.go` differed from the lane's copy by exactly an
  unused `agent-vivy/internal/domain` import.
- `justfile`: `test` and `vet` are back to plain `./...`; the carve-out and its
  comment are gone.
- `sdk/internal/assembly/conformance_results.json`: the five `internal` suite
  rows moved `af2bcb75…` → `724b0f9a037f38c96a3cf03289560bb0cb46fe9eb8729d0bdc6fa3d1ce0d7e8d`,
  computed from the cleaned tree with `sdk/internal/cmd/source-hash`.
- `docs/plans/workflow-system/README.md` gained §0 "Lane and landing policy":
  WF-1 is a long-lived independent branch that may never merge; no WF-1 file
  belongs in the root tree; a stray is a deletion task, not a synchronization
  task; a landing must move the digest in the same change.
- `docs/TODO.md`: the WF-1 row now records the implemented-but-unmerged branch
  and that landing is a separate product decision; the
  `PROVIDER-PROFILE-DIGEST-PIN` row records this closed instance of its own
  failure mode.

## What was explicitly not done

- `feat/workflow-wf1`, its worktree, and the workflow feature are untouched.
  The branch was not merged, rebased, or deleted, and no workflow file was
  moved into it.
- The digest remains a hand-maintained artifact. The regeneration-mode fix in
  `PROVIDER-PROFILE-DIGEST-PIN` (have the producer gate write
  `conformance_results.json` on success) is not implemented here.
- No change to `internal/sourcehash`'s decision to hash untracked files. That
  is deliberate: the digest is the identity of what is on disk, which is what
  ships, and a build has no `.git` to consult. The remedy is lane discipline,
  not a narrower hash.
- Nothing in the `studio/` submodule was staged, committed, or modified.