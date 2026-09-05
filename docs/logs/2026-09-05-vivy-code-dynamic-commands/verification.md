# Verification

## Targeted checks

- `go test ./internal/rpc -run 'TestControlHandler(SkillsCatalog|MCPPromptCommands)$' -count=1` — passed.
- `go test ./internal/runtime -run 'TestEinoMCPBackend(ListsAndGetsPrompts|IgnoresNonUserPromptContent|SkipsServerWithoutPromptCapability)$' -count=1` — passed.
- `go test ./sdk/tui/view -run 'TestDynamic' -count=1` — passed.
- `go test ./sdk/tui/view ./sdk/tui/command ./internal/tui -count=1` — passed.
- From `faces/tui`: `go test ./... -run 'TestPackedLiveDynamicCommandsUseTypedCatalogAndExpansion' -count=1` — passed.

## Product gate

- First `just ci` — all changed packages passed; the suite failed only at unrelated existing cron timing canary `TestCronTriggerManualConflictAndDisabledJob` (`active run not cleared after settle`). Isolated `go test ./internal/runtime -run '^TestCronTriggerManualConflictAndDisabledJob$' -count=10` passed in 20.771s. Captured as `TFLAKE-CRON-MANUAL-SETTLE` in `docs/TODO.md`.
- Second `just ci` — passed in full before the final MCP capability/argument-dialog review fixes.
- Final `just ci` after scope freeze and all code edits — passed in full: fmt, UI typecheck, 201 UI tests, UI build, Go vet/test, headless compile, all plugin modules, and packed `faces/tui`.
- `just vivy-code` — passed; rebuilt `vivy-code.exe` with the release `vivy_headless` tag path.
- `.\vivy-code.exe --help` — passed, exit 0.
- Real PTY: launched `vivy-code.exe`, observed the fullscreen `Vivy™ VIVY CODE` surface, opened the live Commands palette with Ctrl+P, and exited with Ctrl+C — exit 0.

## Real-path note

This is a terminal-only surface; browser port `3015` does not exercise VIVY CODE. The release binary was exercised in a real PTY in addition to typed transport and built-in/packed-face tests.
