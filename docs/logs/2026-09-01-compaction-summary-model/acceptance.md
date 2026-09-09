# Acceptance

Manual check (no code reading required):

1. Add `summary_model: <cheaper-model-id>` under `runtime.compaction` in
   `config.yaml` (on the same provider), then restart vivy.
2. Let a session exceed the compaction threshold (a long conversation or large
   tool result): summary generation in the logs/events uses the cheaper model;
   context compacts normally and the response remains intact.
3. Change `summary_model` to a nonexistent ID: compaction still completes—the
   failed summary call automatically falls back to the main model once (the only
   visible difference is a failover event/log), and the run does not fail.
4. Remove `summary_model`: behavior is byte-for-byte identical to before the
   upgrade (main-model summaries, no failover).
5. The Settings → General → Context compaction switch/threshold behavior is
   unchanged (this field is not part of the settings overlay).
