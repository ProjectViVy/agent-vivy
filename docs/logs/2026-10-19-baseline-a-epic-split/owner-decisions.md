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
