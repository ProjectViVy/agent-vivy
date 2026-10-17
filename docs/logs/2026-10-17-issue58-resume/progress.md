# SDD ledger — plan: docs/superpowers/plans/issue-58/index.md

Ruling: execute serially and self-review; explicit user prohibits subagents. Existing remote implementation is retained, not rewritten for TDD ritual.
Pre-flight: S3/S4 consume native C1/C2; S5 Task1a adapter exists, Task1b/2 unfinished. Use retained authority and avoid ProfileID-to-subject inference.
Ruling: isolate both repositories under /workspace/issue58 so checked-in relative replacements resolve exact lock without modifying the original Laputa checkout.
Current: merge main into recovery candidate; resolve TODO with both current rows; regenerate UI assembly via SDK.
Next: stage UI, commit integration, merge PR71 and inspect conflicts/pins.

Integration: main committed at 598c48f6. PR71 merged preserving both histories. Laputa remains 249; Windows-only 8c5328d is a retained pending dependency correction, not silently discarded.
Ruling: merge tooladapter publishRefusal extraction but keep durable operation admission only at InvokableRun; proposal failure already has an admitted operation. Automatic text merge compiled with missing locals; fixed and focused tests pending.
UI merge RED: unavailable shared context rendered 0%; restored unknown display plus passive accessibility semantics. Same 22 compact/thinking/approval tests GREEN. UI typecheck and build passed.
Bootstrap baseline: real native BML negative-budget regression passed (8 events, zero skip).
Next: finish focused Go tests; reproduce SDK minimal pack failure; fix optional inventory binding without mandatory absent fields.

Canonical active checkout moved to /workspace/issue58-candidate/agent-vivy (full independent clone); sibling Laputa remains exact 249. Earlier worktree is inactive, patch saved at /workspace/issue58/assembly-fix.patch. Go 1.26.4 vcs.RootNames recognizes directory .git only; direct build -x selected invalid /workspace .git. Full clone resolves this without -buildvcs=false.
SDK optional inventories: full clone unmodified test TestV1PackAndInspectProveRecipeRemoval RED missing ContextSources/RunObservers; generated optional pointer inventory accessors + copy-before-binding GREEN 23.432s. Minimal capabilities remain physically omitted. No Eino API change: this is assembly ownership, not model orchestration.

Recap first-load race: activeSessionId preceded RPC peer binding. Original real-browser spec failed with module action not authorized. A diagnostic refresh proved the ordering defect. RecapClient now awaits session/get on the same RPC transport before action invocation; failed binding never dispatches a write. Server authorization unchanged. Two regression tests RED before fix; 14 client/workspace tests GREEN; unchanged original Playwright spec GREEN (1 test, 11.8s) with system Chromium on Linux.
CI first attempt: UI 733 tests passed; studiocore fake SDK build failed because /tmp/.git is an empty sandbox mount. Same test passed with TMPDIR inside the valid candidate clone, without disabling VCS checks. Remaining first-attempt Go SDK suite is still running; this mixed working-tree run is not fixed-candidate acceptance.
Laputa local merge 82d15017224f77a20a79b72251be5bb7d3a86c27 retains 249 native implementation and 8c5328d owner-lock snapshot correction. Root dependency lock still 249 pending deliberate pin update and revalidation. No remote publication.
Next: finish current CI result, pin bundled Laputa candidate, rerun with repository-local TMPDIR, then Windows harness corrections and S5 Task1b/Task2. S6 not accepted.

Pin update: go.mod and laputa-source.lock.json now name local merged Laputa 82d15017224f77a20a79b72251be5bb7d3a86c27. Reproduce using BOTH /workspace/issue58-handoff/*.bundle in sibling full clones. This unpublished dependency cannot be fetched from GitHub; do not publish the Vivy candidate alone. No remote state changed.

Harness correction: six original BML Home fixtures now register immediate error-reporting cleanup without changing their assertions. Native owner snapshot now records presence/type of only the exact root .owner.lock; nested same-named data remains byte-checked. Laputa candidate advanced to 27da7b9e2d991857650b9e373627b65119a73eec (Linux Issue58 suite executed; Windows still unavailable).
CI first attempt also caught source/lock changes during SDK builds caused by this session proceeding with Recap and pin edits before the long SDK suite finished. These source-hash failures are invalidated execution evidence, not product defects. Next full run must freeze BOTH source trees throughout; do not reinterpret it as passing.

Fixed CI concluded EXIT1 at DIVA overlay compile, after standard root tests/SDK pack PASS. Repair test helper via the generated optional inventory accessor; retain every test and assertion. No platform-specific excuse or test exclusion. Library upload of the 120784272-byte handoff archive failed before preparation: hosted apps tools/list network error; no persistent file ID returned. Local archive and full bundles remain available.
