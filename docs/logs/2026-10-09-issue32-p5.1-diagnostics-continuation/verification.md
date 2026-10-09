# P5.1 verification

## Baseline red

Before implementation, the focused regressions reported:

- Appending `two` after reading `one` returned `Gap=true` and replayed both rows.
- Same-inode overwrite/regrow preserving size and mtime returned `Gap=false`.
- The oversized-line page produced the legacy dotted cursor instead of v2.
- An incomplete JSON line was emitted as a malformed row.
- A large serialized record measured 163,905 bytes.
- A legacy cursor resumed at its old offset rather than resetting with a gap.

## Green

Commands run with Go 1.26.4 (`GOTOOLCHAIN=local`):

```text
go test ./internal/logging -count=1                  PASS
go test -race ./internal/logging -count=1            PASS
go test ./internal/rpc -run '^TestDiagnostics' -count=1 PASS
```

The RPC test uses the actual control adapter: append GUI row, read logs, append
another row, then continue with the prior cursor. It returns only the appended
row and reports no gap.

Budget evidence:

- Counted reader test reaches exactly 4,096,000 physical bytes when charging
  64-byte input and output anchors plus the scan.
- Oversized-line service read returns cursor offset 4,095,936; the 64-byte
  outgoing anchor brings the first request to exactly 4,096,000 bytes.
- The cursor persists `discard_line=true`; the next request skips the line
  tail and reads `tail` once.
- Serialized-record regression marshals the oversized Unicode/control-field
  fixture to no more than 8,192 bytes and confirms valid UTF-8.

Windows compile check:

```text
GOOS=windows GOARCH=amd64 go test -c -o /workspace/work/issue32/p5/logging-windows.test.exe ./internal/logging PASS
```

This confirms compilation only. Native Windows file identity and runtime
execution remain pending. The repository's `just` executable is unavailable,
so `just ci` has not run.
