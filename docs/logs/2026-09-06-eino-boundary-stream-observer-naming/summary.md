# EINO-BOUNDARY-AUDIT §5.5 / §5.6

## What changed

- Kept `observingChatModel` as the producer-path ChatModel stream tee. Eino
  v0.9.13 `callbacks.OnEndWithStreamOutput` is a sibling `Copy` after
  `Stream()` returns and cannot place the Begin marker, apply `Pipe(8)`
  backpressure, fail-close the graph stream from persist errors, or run the
  tool-settled barrier.
- Recorded that exception on `Stream` and `withLiveModelStreamObserver`.
- Added unit tests for Begin-before-return, live tee before downstream Recv,
  chunk-error fail-closed, missing-observer identity, inner Stream error, and
  EOF close.
- Renamed six backends that implement only Vivy `tools.*Operations` and have
  no Eino import: `CommandBackend`, `HTTPBackend`, `MCPBackend`,
  `SequentialThinkingBackend`, `WebFetchBackend`, `DownloadBackend`.
- Left `EinoFilesystemBackend`, `EinoSkillBackend`, `EinoCheckpointAdapter`,
  and `EinoTodoBackend` named as Eino interface adapters.

## Scope

Worktree `../agent-vivy-eino-boundary`, branch `feat/eino-boundary-5-5-5-6`.
Kernel/docs only. No Studio, UI, or tenant Journal.

## Explicitly not done

- MCP transport replacement with `eino-ext/components/tool/mcp` (§5.1)
- `tool_search` replacement with `middlewares/dynamictool/toolsearch` (§5.2)
- Sequential Thinking replacement with EinoExt (§5.3) — rename only
- plantask middleware wiring or dead-compat deletion (§5.4)
- Rewriting historical `docs/logs/` that mention the old type names
