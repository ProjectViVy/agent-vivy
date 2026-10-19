# Owner decisions on the baseline-A plan (2026-10-19, after the branch was pushed)

These decisions refine the planning delivery. They are recorded in the plan,
the Story package and the epic packages; the branch is updated accordingly.

1. **Loop hard stop moves with E-NUDGE.** The count-6 `loop_detected` stop and
   the ten-step window are one mechanism in `internal/runtime/loopguard.go`.
   Owner: split it too, put it in the package. D04 keeps the exact-tuple guard
   as the interim core implementation; E-NUDGE N00 adds `NudgeDecision.Stop`,
   N01 returns it, N02 maps it once to the terminal, and N03 deletes the core
   body. Unselected species rely on `MaxToolTurns`/Eino `MaxIterations`; no
   second core window exists at any point.
2. **Warming is opt-in.** Owner: warming becomes a package but is not loaded by
   default. The shipped default-family Recipes (`default`, `headless`, `diva`,
   `vivy-code`, `scx`) and the lite/masks Recipes do not select
   `vivy/exp-warming`; only `recipes/exp-warming.vivy.yml` and the combined
   `recipes/exp-loop-features.vivy.yml` select it. The default species therefore
   loses warming behavior (no warm request, no `cache.warmed` event), an
   owner-approved change from baseline A.
3. **D07 is retained as an explanatory step, not withdrawn.** Baseline A has no
   bundle/toolkit subsystem, so D07 has no deletion work here. It stays as the
   checkpoint for re-intake: the discarded 2026-10-08..10-18 batch that carries
   bundle/toolkit must resolve content-addressed identity, acquisition/consent
   digests and toolkit action hashes against C5 (hash-free owned releases,
   opaque `ArtifactID`, release-sequence consent) before merge, or leave the
   subsystem out. Do not merge it first and fix hashes later.
4. **Post-baseline re-intake is a later gated track.** The 168 commits and 30
   first-parent merges from 2026-10-08..10-18 remain unverified; when they are
   re-introduced, each batch must pass the post-#91 contract (no custom hashes,
   one execution path, no second runner/queue, CI and benchmark non-regression)
   and D07's bundle/toolkit checkpoint applies to the batch that carries it.
5. **D07 file/task text is not executable at baseline A.** The paths and
   commands in D07 describe the re-intake work; the Story has no baseline-A
   deletion action and must not be implemented against the 10-07 tree.
6. **Nudge is not installed for now (provisional).** Owner: do not add
   `vivy/exp-nudge` to the shipped default-family Recipes or the lite/masks
   Recipes; only `recipes/exp-nudge.vivy.yml` and the combined
   `recipes/exp-loop-features.vivy.yml` select it. The default species therefore
   has no coaching and no per-tuple loop stop, relying on the existing
   `MaxToolTurns`/Eino `MaxIterations` cap. This is a deliberately deferred
   product choice to revisit later, not an omission; the epic records it as a
   decision and the default artifact's none-case evidence must show no nudge
   import, constructor or `tool.nudge` producer.
7. **The current planning head is `d47a343` and this round implements Phase A
   only.** The phase split in
   [the handoff supplement](../../plans/2026-10-19-stable-loop-reintake-handoff.md)
   governs delivery: Phase A is the #91 base cut (D00–D14), Phase B is the three
   EXP epics, Phase C is reviewed re-intake under #83 with #81 as backlog. The
   three EXP epics are tracked in one issue, #93; Phase A acceptance neither
   requires their packages nor claims final C11 physical omission.
8. **Execution validation contract for the base cut.** Story iteration runs
   focused checks only: L0 documentation scope uses `git diff --check` plus a
   relative-link scan; L1 package scope uses `just fmt-check` and direct
   `go vet`/`go test -timeout 35m` on the affected packages; L2 shared-contract
   scope (shared runtime, generated Assembly, storage/migrations, plugins or
   faces) uses the full module test suite plus `just vet`, and `just plugin-ci`
   when plugin or face modules change. `ensure-laputa`, `bootstrap-test`,
   `ui-ci`, `headless-compile` and `test-postgres` are skipped during
   iteration. The final base candidate still runs the repository `just ci`
   once. Postgres, real-provider, BML and real-path acceptance move to the
   PR/upstream stage; a missing environment stays pending and is never recorded
   as a passed gate. The independent non-author audits are not reduced.
9. **Implementation branch and commit identity.** Implementation is cut from
   `feat/agent-loop-deletion` at `d47a343`; commit attribution stays with the
   human owner, `mastwet`, and no AI tool appears as author, committer,
   co-author, PR author or contributor.
10. **One unified epic issue.** #93 covers nudge, warming and steering
    extraction together; per-epic issues are not created.
11. **Infrastructure alignment with main is approved for this round.** E1
    (Eino `v0.9.13` -> `v0.9.21` with the pending-run restart policy) and E2
    (the vendored Claude stream-race patch) land as a Phase-A preamble before
    D04/D05. They are dependency/recovery alignment, not capability increments,
    so they do not wait for the re-intake batches. E1 re-cites the plan's Eino
    references at v0.9.21 in the same change. The optional provider bug-fix set
    (E3) stays a separate triage: self-contained fixes only, never commits that
    cite post-baseline runtime code. Assessment:
    [provider/Eino alignment](../../research/2026-10-19-provider-eino-alignment-backport.md).
