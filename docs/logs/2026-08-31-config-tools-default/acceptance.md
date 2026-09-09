# Acceptance — 2026-08-31 config tools default surface retained

How to verify:

1. The local config.yaml `tools:` section contains only `approval` (no `enabled`).
2. `just run` starts normally, and the log contains no
   `tools.enabled must list at least one tool`.
3. In Settings → Tools, the card displays the literal text "Configuration defaults: 26 tools (no override written)" and most tool toggles are active.
4. A user who explicitly wants to disable all tools must write an override in the Settings page (pure chat mode), rather than an empty table in config.yaml—an empty table is still rejected by startup validation.
