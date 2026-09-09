# 2026-08-26 — AGENTS.md parallel worktree isolation and per-delivery commit rules

## What changed

`AGENTS.md` (the repository governance document) added two rules and one
explanatory section, based on a comparison with agent-diva’s repository rules
(LOCK.md mutex, atomic commits, and worktree isolation) and the user’s decision:

1. **`parallel-worktree-isolation` (hard requirement)** + new section
   “Parallel lanes (worktree isolation, hard requirement)”: a shared root
   worktree may carry at most one active write lane. A second concurrent lane
   (another agent session, Studio session, or human edit), or a new feature
   started while the root tree is dirty, must be developed in a separate
   `git worktree` + branch and returned through a branch merge/PR. Editing the
   root tree while another lane is active and stacking unrelated topics in the
   root tree are forbidden. `LOCK.md` is not introduced; structural isolation
   replaces protocol coordination.
2. **`commit-one-concern-per-deliverable`**: each completed delivery is committed
   at completion as a single-concern commit; stage only the delivery’s explicit
   paths, remove temporary artifacts, and do not include unrelated dirty changes.
   Push still requires explicit user authorization. (A weakened port of agent-diva’s
   per-update auto-commit: commits follow deliverables rather than every update.)
3. The “Not ported from agent-diva on purpose” section was rewritten:
   `LOCK.md` mutex, root `TODOLIST.md`, `/new-command`, per-update auto-commit, and
   the reply prefix remain unported, with concurrency and commit hygiene delegated
   to the two native rules above.

## Scope

- `AGENTS.md`: new section + two Rulebook rules + rewritten not-ported section.
- `docs/TODO.md` §0.1: new `PROC-COMMIT` entry (see below).
- This iteration log.

## Decision origin

After comparing the agent-diva rules, the user decided that worktree isolation is
a **hard requirement** (shared root trees had already caused problems across
multiple lanes); atomic commits are ported in the recommended weakened form;
the LOCK.md mechanism is not being introduced for now.

## Explicitly not done

- No `LOCK.md` lock file was introduced (structural isolation replaces it; reassess
  if it proves insufficient in the future).
- No per-update auto-commit or `[I strictly follow the rules]` reply prefix was introduced.
- The three completed but uncommitted topics currently in the root tree (Evolution
  page / Welcome Wizard / Chat Message Actions) were not split or committed in
  this delivery. They are recorded in `docs/TODO.md` §0.1 `PROC-COMMIT` for
  splitting under the new rules or merging after human confirmation.
- No kernel / UI code was changed.
