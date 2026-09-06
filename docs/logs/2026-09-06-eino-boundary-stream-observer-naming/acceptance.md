# Acceptance

1. A live model stream still persists reasoning and text deltas before Eino
   materializes the AgentEvent. Approval/question resume still uses the same
   observer. Completing a turn still emits usage from the later Eino event
   without duplicating the already-persisted text.
2. `EinoCommandBackend` / `EinoHTTPBackend` / `EinoMCPBackend` /
   `EinoSequentialThinkingBackend` / `EinoWebFetchBackend` /
   `EinoDownloadBackend` no longer exist as identifiers. Call sites construct
   `NewCommandBackend`, `NewHTTPBackend`, `NewMCPBackend`,
   `NewSequentialThinkingBackend`, `NewWebFetchBackend`, and
   `NewDownloadBackend`.
3. `EinoFilesystemBackend`, `EinoSkillBackend`, `EinoCheckpointAdapter`, and
   `EinoTodoBackend` still exist: they implement Eino interfaces.
4. The audit doc §5.5 says KEEP with the callback gaps; §5.6 lists the
   renames. `EINO-BOUNDARY-AUDIT` remains OPEN for MCP transport,
   tool_search, Sequential Thinking replacement, and plantask.
