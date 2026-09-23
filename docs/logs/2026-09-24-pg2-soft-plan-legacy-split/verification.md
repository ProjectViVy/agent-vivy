# Verification

- TDD RED: `go test ./internal/runtime -run 'Test(SoftPlan|PlanGuidanceUsesEmbedded|LegacyPlanShellResume)' -count=1` failed as expected: the embedded `plan.md` asset was empty, and legacy shell recovery returned `plan/full_auto` instead of `plan/plan`.
- Focused GREEN: the same command passed (`ok agent-vivy/internal/runtime`, 1.154s; final post-adjustment rerun 0.944s).
- Plan suite: `go test ./internal/runtime -run 'Test(Plan|LegacyPlan)' -count=1` passed (`ok agent-vivy/internal/runtime`, 4.567s).
- Derived conformance identity: `go run ./sdk/internal/cmd/source-hash internal ''` returned `b77db0b4c0f9b71afd475eec20b70de5836a29dca2a266410466b0acc9070859`; all five `internal` source entries in `sdk/internal/assembly/conformance_results.json` were updated.
- Conformance reproduction: `go test ./sdk/internal/conformance -run '^TestCheckedInProviderConformanceMatchesExecutedSuites$' -count=1` passed (`ok agent-vivy/sdk/internal/conformance`, 182.840s).
- Final repository gate: `$env:PATH = 'C:\Program Files\Go\bin;' + $env:PATH; just ci` passed with exit 0. UI typecheck, 400 UI tests, production build, localization checks, `go vet ./...`, `go test -timeout 20m ./...`, headless compile, and plugin CI passed. Full output: `%TEMP%\issue47-pg2-task1-just-ci.log` (`C:\Users\Administrator\AppData\Local\Temp\1\issue47-pg2-task1-just-ci.log`).
- The first full CI run exited 1 because the conformance artifact still carried the prior internal source digest. The artifact was refreshed from the repository source-hash tool; the conformance test and final full CI then passed.
- `git diff --check` passed. Existing `ui/src/generated/assembly.ts` and `ui/src/routeTree.gen.ts` worktree changes were not included.
