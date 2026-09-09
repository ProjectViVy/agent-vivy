# Verification — UI-CHAT-ACT design slice

Date: 2026-09-02.

```text
(Terrain inventory completed by a read-only exploration agent, 36 tool calls; all key file:line references landed in the design document.)
Confirmed sources of truth:
  internal/storage/contracts.go:57-64   Journal Append/Replay + ErrRunClosed
  internal/storage/sqlite/sessions.go:107-114  DeleteSession — the only chunked deletion
  internal/runtime/service.go:1151-1179 runMessages reads the messages table (model context)
  internal/runtime/compaction_service.go:168-178 SessionCompaction marker-row precedent
  internal/rpc/control.go:475-658      dispatch switch + adjacent method surface
  internal/runtime/service.go:629-631  child run restart does not re-execute (basis for not reusing fork)

just ci   (docs-only slice)
  → CI-EXIT:0
```
