# Acceptance view (2026-08-30, live context compaction)

After `just dev` (or `just run` + `ui/pnpm dev`), open `http://127.0.0.1:3015`:

1. **The chat-box context ring is real**: the ring to the left of the input shows
   "Context usage: X% — used / model window tokens". Hovering reveals byte details
   (feed bytes / byte limit) and the "compression threshold reached / compressed"
   markers. The numbers come from the server's `session/context` (feed assembly
   semantics + provider model window), not a client-side hard-coded 256KB estimate.
2. **Automatic compaction on overage**: after the session context exceeds "max tokens ×
   threshold%" (or the byte limit), send a message and compaction runs automatically:
   old tool-call/result placeholders are deterministically cleared first (retaining the
   most recent N turns), then the same model generates a summary to replace the history
   if it is still over the limit. The `context.compacted` event enters the Journal
   (`run/log` exposes mode/before_tokens/after_tokens), the ring drops back, and
   "compressed" appears.
3. **Settings take effect for real**: in Settings → General → the "Context compaction"
   card, change enabled / max tokens / threshold / recent-message retention and click
   "Save configuration"; the values are written to `settings.yaml`. Subsequent runs
   compact at the new threshold without a restart (the engine rebuilds immediately when
   idle and defers while a run is active until the next round).
4. **Compact now works for real**: click "Compact now"; the session soon shows
   "Compaction complete: before → after tokens". After refreshing, usage is lower and
   marked "Session compaction summary applied". Later conversations expose "summary +
   recent-message tail" to the model (the Journal/message list still retains the full
   appended history).
5. **It is no longer a demo**: the demo "Context compaction" card in
   `DivaSettingsPreview` is replaced by the real card (only explanatory text remains
   from the old placeholder); the "Run compaction preview / Restore preview defaults"
   buttons disappear.

## Acceptance tests (automation anchors)

- `go test ./internal/runtime/ -run 'TestEngineSummarizationCompaction|TestEngineReductionRunsBeforeSummarization|TestServiceContextStatusAndCompactSession|TestScheduleEngineReload'` — engine compaction pipeline, session-level persistent compaction, and hot reload all pass.
- `go test ./internal/rpc/ -run TestContextCompactionRPC` — `settings/update` persists compaction, `settings/get` echoes it, and `session/context` plus `context/compact` pass.
