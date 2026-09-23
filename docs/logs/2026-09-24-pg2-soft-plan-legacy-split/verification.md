# Verification

- TDD RED: `go test ./internal/runtime -run 'Test(SoftPlan|PlanGuidanceUsesEmbedded|LegacyPlanShellResume)' -count=1` failed as expected: the embedded `plan.md` asset was empty, and legacy shell recovery returned `plan/full_auto` instead of `plan/plan`.
- Focused GREEN: the same command passed (`ok agent-vivy/internal/runtime`, 1.154s; final post-adjustment rerun 0.944s).
- Plan suite: `go test ./internal/runtime -run 'Test(Plan|LegacyPlan)' -count=1` passed (`ok agent-vivy/internal/runtime`, 4.567s).
- Derived conformance identity: `go run ./sdk/internal/cmd/source-hash internal ''` returned `b77db0b4c0f9b71afd475eec20b70de5836a29dca2a266410466b0acc9070859`; all five `internal` source entries in `sdk/internal/assembly/conformance_results.json` were updated.
- Conformance reproduction: `go test ./sdk/internal/conformance -run '^TestCheckedInProviderConformanceMatchesExecutedSuites$' -count=1` passed (`ok agent-vivy/sdk/internal/conformance`, 182.840s).
- Final repository gate: `$env:PATH = 'C:\Program Files\Go\bin;' + $env:PATH; just ci` passed with exit 0. UI typecheck, 400 UI tests, production build, localization checks, `go vet ./...`, `go test -timeout 20m ./...`, headless compile, and plugin CI passed. Full output: `%TEMP%\issue47-pg2-task1-just-ci.log` (`C:\Users\Administrator\AppData\Local\Temp\1\issue47-pg2-task1-just-ci.log`).
- The first full CI run exited 1 because the conformance artifact still carried the prior internal source digest. The artifact was refreshed from the repository source-hash tool; the conformance test and final full CI then passed.
- `git diff --check` passed. Existing `ui/src/generated/assembly.ts` and `ui/src/routeTree.gen.ts` worktree changes were not included.

## Post-review correction: remove guidance on Plan exit

- TDD RED: `go test ./internal/runtime -run '^TestPlanGuidanceRemovedAfterExitOnScriptedServicePath$' -count=1` failed on the second real model request: guidance count was 1, expected 0. The scripted run began in persisted Plan state, made an initial guidance-bearing request, exited Plan through a tool call, and then made the second request.
- GREEN: the same scripted Service/Engine test passed (`ok agent-vivy/internal/runtime`, 1.830s) after inactive reconciliation removed embedded Plan guidance from combined system messages.
- Focused Plan/legacy suite: `go test ./internal/runtime -run 'Test(Plan|LegacyPlan)' -count=1` passed (`ok agent-vivy/internal/runtime`, 5.354s).
- Internal source hash was refreshed with `go run ./sdk/internal/cmd/source-hash internal ''`: `b2d08e7fee9fa8ee64c88cb7e5f607bf193deea52c49a1d9e3b845d313b5e687`. The five matching entries in `sdk/internal/assembly/conformance_results.json` were updated.
- Final full gate: `$env:PATH = 'C:\Program Files\Go\bin;' + $env:PATH; just ci` passed with exit 0. It passed UI typecheck, 400 UI tests, UI build and localization checks; `go vet ./...`; `go test -timeout 20m ./...` (including `internal/runtime` in 486.498s, `sdk/internal` in 854.301s, and `sdk/internal/conformance` in 307.298s); headless compile; and plugin CI. Complete output: `C:\Users\Administrator\AppData\Local\Temp\1\issue47-pg2-task1-review-fix-ci.log`.
