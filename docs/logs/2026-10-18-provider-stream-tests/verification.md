# Verification — provider stream tests

Linux sandbox, Go 1.26.4, running as root.

| Command | Result |
|---|---|
| gofmt over tracked + untracked Go files | clean |
| `go vet ./...` | clean |
| `go test ./internal/provider/ -count=1` | ok |
| `go test ./sdk/codeclient/ -count=1` | ok (it failed while the dropped guard was in place) |
| `go test -p 1 -timeout 35m -skip '^TestMemoryLoop' ./...` | 83 packages ok; only `TestDeliverableReadSnapshotWriteFailure` fails (needs an unwritable directory, which root defeats; fails identically on untouched `main`) |
| `go test -race` on the Anthropic stream cases | race inside `claude@v0.1.25 claude.go:446` (`EINOEXT-CLAUDE-STREAM-RACE`, upstream); OpenAI cases race-clean |

The cut-off cases were first written to expect an error and failed on the
unguarded code (partial text, nil error), which confirmed the bug through the
real provider path. With the guard dropped, they now pin that behavior.

Not run: memory-loop/DIVA/ui-ci again (no code outside `internal/provider`
tests changed since the 2026-10-18 Eino upgrade run that passed them).
