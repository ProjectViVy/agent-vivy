# Verification

Linux amd64; Go 1.26.4, Node 24.19.0, pnpm 11.19.0, just 1.58.0. Tools loaded from `/workspace/.vivy-cloud/env.sh`. No tests claimed yet. Initial PATH `go` was an unrelated program, corrected before tests.

Root environment Laputa ff3936f did not match source lock. Isolated sibling worktree now pins exact 249a756e04c5f28af781e4dadea9e9086689bde0, preserving the original environment.

Merge working tree: pnpm install --frozen-lockfile PASS; SDK stage-ui initially failed before dependency installation, then PASS. pnpm typecheck PASS; pnpm build PASS (chunk-size advisory). Vitest compact/thinking/approval RED 1/22 (false 0%), then GREEN 22/22. Go focused attempt build-failed tooladapter undefined locals following automatic merge; correction is being tested. Historical incoming Markdown whitespace retained; git diff --cached --check excluding docs/logs passes.

Re-read CI 38829757533: backend and full UI pack both fail on optional RuntimeAssembly ContextSources/RunObservers accesses in assembly_memory.go; missing executable is downstream. Native Windows owners also failed; no new Windows result claimed.

Full-clone SDK test: go test -json -p=1 ./sdk/internal -run '^TestV1PackAndInspectProveRecipeRemoval$' -count=1 -timeout=8m. RED 27.975s missing optional assembly fields, GREEN 23.432s after fix. Worktree attempts before relocation failed on VCS inspection and are not RED/GREEN code evidence.

Recap client: pnpm --dir ui exec vitest run ../plugins/vivy-memory/ui/vivy-memory/src/recap-client.test.ts ../plugins/vivy-memory/ui/vivy-memory/src/recap.test.tsx — 14 PASS. Original browser: pnpm --dir ui exec playwright test --config ../.workspace/issue58-verification/playwright.config.ts — 1 PASS, real disposable backend, system /usr/bin/chromium. Config changes only browser executable and absolute fixture paths; original spec unchanged. Linux only. Logs archived in /workspace/issue58-handoff.

Laputa 27da7b9e2d991857650b9e373627b65119a73eec: go test -json ./actmem -run '^TestIssue58' -count=1: 93 test pass events, zero fail/skip, Linux. BML six Home fixture tests: PASS on Linux; this verifies retained assertions, not Windows cleanup behavior. Recap UI tsc --noEmit PASS.

Fixed integration pair ec12fe21a45a17b2e8d5521e2b807bb851b95eb8 / Laputa 27da7b9e2d991857650b9e373627b65119a73eec: just ci EXIT 1. UI 735 tests/97 files PASS; standard root Go suite PASS, runtime 130.418s and SDK/internal 303.842s; repository-local TMPDIR fixed studiocore synthetic build. DIVA packed and inspected successfully (generation a16e7a82553c704c75eee3ac8bf6966f05e2cc38442991123021eab3d1f5631b), but App overlay BUILDFAIL before test bodies: issue58_memory_ownership_test.go directly referenced absent RunObservers. Mission regression did not execute in this attempt. Standard go test output does not enumerate skip events; no zero-skip claim. Later just dependencies were not reached.

DIVA script at 3eafa2c2652f87f69ce9e25610b6e53e9ea0f110 / native27 EXIT1 after 776.255s actual App tests. Only reported failure: TestMemoryLoopActivityFailurePreservesSourceAndArchiveRetry expected DeadlineExceeded, but the newer native owner policy probe returns typed recap_unavailable immediately for the deliberately directory-shaped ACTMEM.MD. The source/primary assertions before this point passed. Mission/dream/actor fixtures executed (23:33:26, 23:33:47, 23:34:09 UTC) without a reported failure; this does not resolve Windows reproducibility. Tighten the archive test to require the specific native code and retain all durable-source/retry/capsule assertions, rather than accept arbitrary errors or change production error handling.

Archive regression targeted real DIVA overlay: TestMemoryLoopActivityFailurePreservesSourceAndArchiveRetry PASS 2.32s, package PASS 2.367s, zero skips (JSON result). The exact native recap_unavailable error is required, and original retention, identity stability, no extra primary requests, public retry and final capsule content all remain asserted. Full DIVA suite was not rerun after this test-only correction yet. Logs: /tmp/issue58-memory-loop-archive-green.log.

### S5 Task1b source reader (working candidate after efb981f7)

Linux, unchanged local Laputa 27da7b9. `go test -json ./internal/runtime -run '^TestIssue58RecapSources' -count=1 -timeout=3m`: RED two unavailable-body tests, GREEN all three (0.219s package). Covers exact historical bodies, restore provenance, revoked grant, historical correction subset, changed range/correction digest and absence of extra model requests.
`go test ./internal/modules/diva-cognitive ./internal/runtime -run 'Recap|Compaction' -count=1 -timeout=4m`: PASS (0.089s/2.308s). `go test ./internal/modules/diva-cognitive ./internal/app -run 'Issue58Recap(Sources|Authenticated)' -count=1 -timeout=4m`: PASS (0.038s/0.090s); the App test is existing transport authorization, not a new native-source end-to-end claim.
`pnpm --dir ui test recap`: 15 PASS, two files, 1.38s. New test preserves tool errors, original text, downloaded attachment bytes, copied reference text, saved file body and correction coordinates. No Windows or full-candidate acceptance implied.
`pnpm --dir ui exec tsc --noEmit`: PASS. `git diff --check`: PASS. Generated UI manifest refreshed by the normal stage-ui pretest hook; never hand edited.
