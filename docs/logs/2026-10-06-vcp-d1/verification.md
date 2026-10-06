# VCP D1 — verification

## Commands run

```text
go build ./...                                            # clean
go test ./internal/runtime -run 'TestCompactionPolicyForModelOverride|TestFileManifest|TestCompactSession'   # green
go test ./internal/runtime ./internal/rpc ./internal/app ./sdk/tui/...   # all green (runtime 47.7s)
cd ui && pnpm exec tsc --noEmit                           # clean
cd ui && pnpm exec vitest run CompactionSettingsCard store.test ui-sdk-face-compat   # 67 tests green
go test ./sdk/internal/conformance/                       # green after digest re-pin
```

## Coverage added

- `internal/runtime/compaction_test.go`
  - `TestCompactionPolicyForModelOverride` — `For` folds matching override, drops the map, leaves other models on globals; trigger math reflects the override.
  - `TestFileManifestSplitsReadAndModified` — read/modified split, `pattern` fallback, dedup, bad JSON skipped, non-file tools ignored.
  - `TestCompactSessionInstructionsAndManifest` — instructions reach the summarizer prompt as `Focus:`; summary carries `Files read:`/`Files modified:`; event payload carries `files_read`/`files_modified` counts.
- `internal/app/compaction_test.go` (new)
  - `TestCompactionPolicyForPerModelResolution` — config+overlay field-wise merge, `m-x` resolves, `m-z` falls back, `mergedCompactionConfig` merges per key.
  - `TestCompactionPolicyForOverrideBeatsCatalogWindow` — per-model `max_tokens` wins over the catalog window at `TriggerTokens`.
- `internal/rpc/control_test.go` (extended `TestContextCompactionRPC`) — 4097-byte instructions → `InvalidParams`; valid instructions flow through to a normal skipped result.
- `sdk/tui/live/controller_test.go::TestLiveCompactForwardsInstructions` — `/compact focus on auth` sends `{session_id, instructions: "focus on auth"}`.
- `sdk/tui/command/command_test.go` — `/compact focus on auth` valid; whitespace-only arg rejected.

## Acceptance checklist (plan)

- [x] `CompactSession` accepts instructions; appended as `Focus:`; empty = unchanged.
- [x] `PerModel` map resolved at `TriggerTokens`/`EffectiveMaxTokens` path via `For(model)`; yaml fields `trigger_percent`, `keep_recent`, `max_tokens`.
- [x] `Files read:` / `Files modified:` manifest lines (bounded 50 each); `context.compacted` payload gains counts.
- [x] `context/compact {session_id, instructions?}` with length bound.
- [x] `/compact` arg passthrough in TUI.
- [x] GUI `compactSession(sessionId, instructions?)` + settings-card field.
- [x] 409 busy behavior unchanged (covered by existing tests).
