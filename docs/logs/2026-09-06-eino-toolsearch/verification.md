# Verification

Commands run in the isolated `refactor/eino-toolsearch` worktree:

- `go test ./internal/runtime -count=1` — PASS.
- Targeted runtime Runner/mount tests — PASS (official search, hidden mount
  rehydration, no-dynamic path, and mount middleware cases).
- `go test ./internal/config ./internal/app/settings ./internal/tools -count=1` — PASS.
- `go test ./internal/config ./internal/app/settings ./internal/tools ./internal/app -count=1` — the config/settings/tools packages passed; `internal/app` setup was blocked by the checkout's missing `ui/dist` embed directory (`ui/embed.go: pattern all:dist: no matching files found`).
- `go test ./internal/runtime -run 'TestEngineAgentsMDInjection|TestServiceAgentsMDInjectionIsTransient|TestServiceAgentsMDInjectionSurvivesApprovalResume|TestEngineAlwaysSkillsInjection|TestServiceFeedsSessionHistory|TestServiceHistoryIsolatedAcrossSessions|TestServiceRunLeadsWithPreamble' -count=1 -v` — PASS after accounting for Eino's transient official deferred-tool reminder.
- Review targeted command `go test ./internal/runtime -run 'TestEngineRunnerUsesOfficialToolSearchForActiveDynamicTools|TestEngineRunnerMountsHiddenToolInfoAfterTheFirstModelCall|TestEngineRunnerWithoutDynamicToolsDoesNotInstallSearch|TestEngineRejectsReservedToolSearchName|TestMountedToolVisibilityMiddleware|TestFixedVisibleToolNamesAreExact' -count=1` — PASS (independently rerun by the delivery owner).
- Review boundary command `go test ./internal/runtime -run '^TestEngineRunnerUsesOfficialToolSearchWithoutFixedTools$' -count=1` — PASS (independently rerun by the delivery owner).
- Review package command `go test ./internal/config ./internal/app/settings ./internal/tools -count=1` — PASS (independently rerun by the delivery owner).
- `go test ./internal/runtime -count=1` — PASS (248.322s).

The custom-code exception is covered by the runtime Runner and mount tests:
the official middleware supplies the raw `tool_search` path and selected
deferred tools, while Vivy's mount test reproduces a persisted state with a
missing hidden `ToolInfo` and verifies ordered rehydration/idempotence. The
official middleware has no hook for that external mount state, and ordinary
adapters do not alter `state.ToolInfos`.

Full gate follow-up:

- First `just ci` — FAIL: existing runtime assertions treated the official
  `<available-deferred-tools>` reminder as a real user/preamble and one
  unrelated `TestCronAtJobDeletesAfterSuccessfulRun` timing canary missed its
  settle window. The runtime assertions were updated to ignore only that
  transient reminder; no product behavior was weakened.
- Second `just ci` — PASS: fmt-check, UI install/typecheck/201 tests/build,
  Go vet, full Go test, headless compile, and all plugin/face vet+test stages
  completed successfully.
- Final `just ci` after the zero-fixed/one-dynamic Runner boundary test and
  comment-only cleanup — PASS: UI 201 tests/build, Go vet, full Go test
  (including runtime), headless compile, and all plugin/face vet+test stages
  completed successfully.
