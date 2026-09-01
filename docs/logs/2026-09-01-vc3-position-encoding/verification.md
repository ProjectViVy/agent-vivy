# Verification — VC-3 slice 6 (positionEncoding negotiation)

Date: 2026-09-01. Worktree `agent-vivy-vc0`, branch `feat/vc1a-bash-tool`.

## Commands and results

| Command | Result |
| --- | --- |
| `cd plugins/lsp && gofmt -l .` | clean |
| `cd plugins/lsp && go vet ./...` | clean |
| `cd plugins/lsp && go test -race -count=5 ./...` | ok (1.34s) — negotiation parse table, wire-shape assertion, end-to-end handshake (utf-16 default + utf-8 declared), `TestApplyEditsEncodings` (utf-16/utf-8/utf-32 on an emoji line + wrong-encoding corruption guard), existing e2e suite |
| `./vivy-sdk.exe verify plugins/lsp` | ok |
| `./vivy-sdk.exe pack --with lsp` | generation `gen_a4da2b582b6d8df2` (sha256 a52ed0f2…); manifest names lsp with all five tools |
| Root `just ci` | not run this slice — kernel untouched (plugin-module-only change; the lsp module is not scanned by `just ci`, the five-step gate above applies) |

## Flake found and fixed in-slice

`TestDiagnosticsToolEndToEnd` failed intermittently (~50% under `-count`
loops) with "wait_ms elapsed without a diagnostics publish". Root cause:
the wait's base generation was snapshotted after the sync request, racing
an already-arrived publish. Fixed by snapshotting before the request
(`diagGeneration`); `go test -race -count=5` is green and the suite no
longer stalls 3s on the affected path.
