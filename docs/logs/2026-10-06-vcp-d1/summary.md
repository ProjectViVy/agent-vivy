# VCP D1 — Compaction instructions, per-model overrides, file manifest

Story: `docs/superpowers/plans/vivy-code-parity/D1-compaction-instructions.md`

## What landed

- **`CompactOptions{Instructions}`** (`internal/runtime/compaction_service.go`): `CompactSession(ctx, id, opts)` threads a caller-supplied focus into `generateSessionSummary`, which appends `Focus: <instructions>` to the summarizer's system prompt verbatim (empty = previous behavior).
- **Per-model policy overrides**: `runtime.CompactionOverride{MaxTokens, TriggerPercent, KeepRecent}` (zero = inherit); `CompactionPolicy.PerModel` carries the unresolved map and `For(model)` folds the matching entry into the flat fields and drops the map — the stored policy is always a model-bound snapshot. `compaction.per_model.<model>` exists in both `config.CompactionConfig` (operator default) and `settings.CompactionSettings` (overlay); `compactionPolicyFor` merges field-wise (overlay non-zero fields win per key) and resolves against the active route's model ID at startup (`modelID`) and on settings-save reload (`GetModelInfo().ID`). `mergedCompactionConfig` merges the overlay map field-wise for the startup path.
- **File tracking manifest**: `fileManifest` walks the folded head and collects `path` args (glob falls back to `pattern`) into `Files read:` (read_file/search_files/grep/glob/list_dir) and `Files modified:` (write_file/patch/multiedit) lines appended to the durable summary — deduped, bounded at 50 each. `payloadContextCompacted` gains `files_read`/`files_modified` counts.
- **RPC**: `context/compact` accepts `instructions` (4096-byte bound → `InvalidParams`); busy/not-needed semantics unchanged.
- **TUI**: `/compact [instructions]` — optional joined args (whitespace-only rejected) forwarded verbatim as `instructions`.
- **GUI**: `compactSession(sessionId, instructions?)` through api/store/Face contract; `CompactionSettingsCard` gains an optional focus input feeding the Compact-now path; en/zh strings.

## Boundaries held

No overflow recovery (D2). Summarizer prompt stays as configured (Chinese); instructions pass verbatim — the redaction pass applies to the stored summary, not the prompt. The chat-level compact control remains D3 (not absorbed; the settings card is not the chat surface).

## Acceptance check

`/compact focus on the auth refactor` → `Focus: focus on the auth refactor` on the summarizer prompt and `Files read:`/`Files modified:` in the stored summary; `per_model` override shifts `TriggerTokens` for the matching model only; GUI compact path accepts instructions.
