# D1 — Compaction instructions + per-model overrides + file tracking

**Goal:** `CompactSession` accepts instructions; per-model policy overrides; summary carries a `Files:` manifest (read/modified); wired to RPC/TUI/GUI.
**Epic:** D. **Requirements:** RQ-CMP.
**Spec:** [VCP-D1](../../specs/2026-10-06-vivy-code-parity-design.md) §5.5. **Baseline:** `f34f3ce`.

## Scope

**Files:** `internal/runtime/compaction_service.go` (signature + summarizer prompt), `internal/runtime/compaction_policy.go` (`PerModel` map + resolution), `internal/app/compaction.go` (config merge), `internal/rpc/control.go` (`context/compact` param), `sdk/tui/command` (`/compact` takes trailing args), `ui/src` (CompactionSettingsCard or chat control — see D3 split), `schemas/events` if event payload changes.

## Tasks

- [ ] `CompactSession(ctx, id, CompactOptions{Instructions string})`; instructions append to the summarizer prompt under a `Focus:` section; empty = current behavior.
- [ ] `CompactionPolicy.PerModel map[string]CompactionOverride`; resolved against the run's model at `TriggerTokens`/`EffectiveMaxTokens`; config yaml fields `trigger_percent`, `keep_recent`, `max_tokens`.
- [ ] File tracking: fold pass collects read_file/search/glob paths and write_file/patch/multiedit paths into `Files read:` / `Files modified:` manifest lines appended to the summary (bounded, e.g. 50 paths each); extend `context.compacted` event payload with `files_read`, `files_modified` counts.
- [ ] RPC: `context/compact {session_id, instructions?}` — validate length bound.
- [ ] TUI: `/compact` Spec gains arg passthrough (register arg-taking commands; `/compact foo bar` → instructions="foo bar").
- [ ] GUI: extend `compactSession(sessionId, instructions?)` in api/store; CompactionSettingsCard adds an optional instructions field (chat-level button belongs to D3 — if trivial, do it here and mark D3 absorbed).
- [ ] Tests: instructions reach the summarizer prompt; per-model override beats global at threshold math; file manifest appears in folded summary; 409 busy behavior unchanged; `/compact` with args hits RPC.
- [ ] `go test ./internal/runtime ./internal/rpc ./sdk/tui/... -run 'Compact'`; `just ci`.
- [ ] Commit `feat(compaction): instructions arg, per-model overrides, file manifest`.

## Boundary

No overflow recovery (D2). Summarizer prompt stays English; instructions pass verbatim (redaction pass applies to the summary, not the prompt).

## Acceptance

`/compact focus on auth refactor` produces a summary weighted to that instruction; GUI compact path accepts instructions; per-model override demonstrably shifts the trigger point.
