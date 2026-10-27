# Verification

Date: 2026-10-24
Host: macOS 27.0.0 arm64 (Apple Silicon)
Checkout: root working tree, branch `stable`

## Toolchain

The host Go is 1.27.1, which fails to build the pinned grpc v1.64.1
(`undefined: http2.TrailerPrefix`). Every command below ran with
`GOTOOLCHAIN=go1.26.4 GOPROXY=https://goproxy.cn,direct`, matching `go.mod`.
Tests ran with `-p 1` to stay inside the 16 GB host budget.

## Commands and results

```text
gofmt -w internal/runtime/message_projector.go internal/runtime/service.go \
  internal/runtime/message_projector_incremental_test.go
GOTOOLCHAIN=go1.26.4 go build ./internal/runtime/...
  pass
GOTOOLCHAIN=go1.26.4 go vet ./internal/runtime/
  pass
GOTOOLCHAIN=go1.26.4 go test -p 1 -run 'TestMessageProjector|TestProjector' ./internal/runtime/
  ok  agent-vivy/internal/runtime  1.432s
GOTOOLCHAIN=go1.26.4 go test -p 1 ./internal/runtime/
  ok  agent-vivy/internal/runtime  66.320s
GOTOOLCHAIN=go1.26.4 go test -p 1 ./internal/storage/... ./internal/rpc/...
  ok  agent-vivy/internal/storage             0.366s
  ok  agent-vivy/internal/storage/migrations  3.211s
  ok  agent-vivy/internal/storage/postgres    0.354s
  ok  agent-vivy/internal/storage/sqlite      9.427s
  ok  agent-vivy/internal/rpc                 19.501s
GOTOOLCHAIN=go1.26.4 go test -p 1 ./internal/app/
  ok  agent-vivy/internal/app                 19.444s
```

## New regression tests

`internal/runtime/message_projector_incremental_test.go`:

- `TestProjectorLiveCursorSkipsEarlierJournalEvents` — the second projection
  resumes after the first exchange's last seq, skips `ListMessages`, produces
  the same four rows, and a no-op fold neither duplicates rows nor rewinds.
- `TestProjectorLiveCursorCarriesPendingDeltas` — a half-streamed answer is
  carried in the cursor across calls and flushed exactly once on completion.
- `TestProjectorLiveCursorDoesNotAdvanceOnConflict` — a pre-existing
  conflicting row fails both attempts and leaves the cursor at the last good
  seq.
- `TestProjectorLiveCursorSuppressesNoContextRuns` — no-context shell runs project
  no rows, never list messages, and a repeat trigger does not replay them.
- `TestProjectorReconcileSeedsLiveCursorAndDropsTerminal` — read repair seeds
  a live run's cursor (next reconcile resumes from the head) and evicts it
  once the run row is terminal.

Existing projector tests (including one-shot child suppression and legacy v1
tolerance) still pass.

## Not run

- `just wiring-ci` / `just kernel-ci`: require an independent Linux-filesystem
  checkout; not available on this macOS host.
- Browser smoke of the split pair (`just run` + `pnpm dev`, port 3015):
  requires a real provider key and is left to the owner. The change is in the
  persistence/projection path, not UI rendering.
- Windows/native full CI.
