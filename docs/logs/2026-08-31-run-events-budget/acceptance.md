# Acceptance — 2026-08-31 run-event budget exemption

How to verify the fix:

1. Restart the development instance (`just run`, loading the new binary and the config's default 26-tool surface after removing the stale `tools.enabled`).
2. In Settings → Tools, the card should show most tools as active, with the top description set to the literal "Configuration defaults: 26 tools (no override written)".
3. Create a new session and send the message "What tools do you have now? Can you see what is in the workspace?".
   - Before the fix: after a few seconds it reported the literal "This conversation did not complete…reached a safety budget".
   - After the fix: the model replies completely; it can be seen genuinely calling `list_dir` (in agent mode, the workspace file list appears in the tool card/reply).
4. Long replies no longer stop midway: ask the model to write a long piece (for example, "Describe yourself in detail"), and streaming output should finish completely without triggering the safety budget.
5. `data/logs/vivy.log.*` should no longer gain new
   `run budget circuit breaker opened ... kind=events` entries.

Note: if the model falls into a meaningless loop, it will still stop at the
model_calls(32)/tool_calls(64) budget ceiling—this is designed runaway
protection, not part of this defect.
