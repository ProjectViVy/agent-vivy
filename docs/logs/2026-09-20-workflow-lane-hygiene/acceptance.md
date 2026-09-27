# Acceptance: how a human can tell the root tree is clean again

Product/user view: `main` is once more only `main`. The workflow branch keeps
its own work, and `main`'s gates and shipped identity describe `main`.

## 1 — No lane leakage in the root tree

```text
git status --porcelain
```

No `?? internal/workflow/` and no
`?? internal/domain/workflow_test_support.go`. Nothing about this shows up in
`git log`, because deleting untracked files leaves no commit; this log entry
plus the SHA-256 values in `summary.md` are the record.

## 2 — The root gate looks at everything

```text
just vet        # or: go vet ./...
```

Passes with plain `./...` and no package filtered out. `justfile`'s `test` and
`vet` recipes contain no `Where-Object` exclusion and no carve-out comment.

## 3 — The shipped identity matches the tree

```text
cd sdk
go run ./internal/cmd/source-hash ../internal ""
```

Prints `724b0f9a037f38c96a3cf03289560bb0cb46fe9eb8729d0bdc6fa3d1ce0d7e8d` —
the value carried by all five `internal` suite rows of
`sdk/internal/assembly/conformance_results.json`, and the value the provider
conformance producer gate recomputes.

## 4 — WF-1 is intact in its own venue

```text
git log --oneline -1 feat/workflow-wf1
git -C .worktrees/workflow-wf1 status --porcelain
git log --oneline main..feat/workflow-wf1
```

The branch still tips at `08fa3d2`, the worktree still holds the whole slice,
and `main..feat/workflow-wf1` still shows exactly that one commit. Hygiene
removed the leakage, not the work.

## 5 — The policy is where the next lane will read it

`docs/plans/workflow-system/README.md` §0 states the lane rules, and the
`docs/TODO.md` WF-1 row states the branch, its tip, and that landing on `main`
is a separate decision that may never be taken.