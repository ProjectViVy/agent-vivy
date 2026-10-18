# Acceptance — provider stream tests

1. `go test ./internal/provider/ -run 'TestStream|TestDeepSeekStream'` passes.
2. Product behavior is unchanged: complete replies, tool calls and provider
   errors behave as before; a cut-off stream is still shown as a short reply
   (known bug, tracked on the board).
3. `docs/DEFER.MD` no longer lists `DEEPSEEK-REASONING-CONTENT`;
   `docs/TODO.md` lists `STREAM-TRUNCATION-SILENT-SUCCESS` as a reported bug.
