# Acceptance — 2026-08-26 AGENTS.md parallel and commit rules

## Manual acceptance (product/user perspective)

1. Open the repository root `AGENTS.md`:
   - Before the “Validation” section, the new **“Parallel lanes (worktree
     isolation, hard requirement)”** section is visible. It specifies one active
     lane in the root tree, `git worktree add ../agent-vivy-<slug> -b feat/<slug>`,
     returning each lane through a branch/PR, and no LOCK.md.
   - Two rules were added at the end of the Rulebook:
     `parallel-worktree-isolation` (marked hard requirement) and
     `commit-one-concern-per-deliverable` (push still requires authorization).
   - The “Not ported from agent-diva on purpose” section was rewritten to explain
     that structural worktree isolation replaces the lock file and that commits
     follow deliverables rather than every update.
2. `docs/TODO.md` §0.1 contains `PROC-COMMIT` (the three uncommitted root-tree
   topics are to be split into separate deliveries).
3. Behavioral acceptance (effective for the next parallel run): when another
   agent or Studio session opens this repository, it should create and work in a
   worktree + branch rather than edit the root tree; every completed delivery in
   this repository should land as an independent single-concern commit (this
   delivery is the first example: it contains only `AGENTS.md` and this log,
   excludes other dirty root-tree changes, and was not pushed).
